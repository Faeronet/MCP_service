package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
	"github.com/telegram-ai-assistant/root/pkg/queue"
	"github.com/telegram-ai-assistant/root/pkg/storage"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var log = logging.New("admin-backend")

func main() {
	ctx := context.Background()
	pg := config.LoadPostgres()
	pool, err := pgxpool.New(ctx, pg.DSN)
	if err != nil {
		log.Error(ctx, "postgres connect", logging.KV{"error", err})
		os.Exit(1)
	}
	defer pool.Close()

	minioCfg := config.LoadMinIO()
	minioClient, err := storage.New(ctx, storage.Config{
		Endpoint:  minioCfg.Endpoint,
		AccessKey: minioCfg.AccessKey,
		SecretKey: minioCfg.SecretKey,
		Bucket:    minioCfg.Bucket,
		UseSSL:    minioCfg.UseSSL,
	})
	if err != nil {
		log.Error(ctx, "minio connect", logging.KV{"error", err})
		os.Exit(1)
	}

	// RabbitMQ может стартовать позже — повторяем попытки
	var rmq *queue.Client
	rmqURL := config.LoadRabbitMQ().URL
	for i := 0; i < 30; i++ {
		rmq, err = queue.New(ctx, rmqURL)
		if err == nil {
			break
		}
		log.Warn(ctx, "rabbitmq connect retry", logging.KV{"error", err}, logging.KV{"attempt", i + 1})
		time.Sleep(2 * time.Second)
	}
	if rmq == nil {
		log.Error(ctx, "rabbitmq connect failed after retries", logging.KV{"error", err})
		os.Exit(1)
	}
	defer rmq.Close()

	jwtSecret := config.LoadString("JWT_SECRET", "change-me")
	adminUser := strings.TrimSpace(config.LoadString("ADMIN_USER", "admin"))
	adminPass := strings.TrimSpace(config.LoadString("ADMIN_PASSWORD", "admin"))
	if adminUser == "" {
		adminUser = "admin"
	}
	if adminPass == "" {
		adminPass = "admin"
	}
	mcpWriteURL := config.LoadString("MCP_WRITE_URL", "http://mcp-write:8001")
	lokiURL := config.LoadString("LOKI_URL", "http://loki:3100")

	handler := NewHandler(HandlerDeps{
		Pool:       pool,
		MinIO:      minioClient,
		Queue:      rmq,
		JWTSecret:  []byte(jwtSecret),
		AdminUser:  adminUser,
		AdminPass:  adminPass,
		MCPWriteURL: mcpWriteURL,
		LokiURL:    lokiURL,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/api/login", handler.Login)
	mux.HandleFunc("/api/login/", handler.Login)
	mux.Handle("/api/upload", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.Upload)))
	mux.Handle("/api/docs", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.ListDocs)))
	mux.Handle("/api/jobs", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.ListJobs)))
	mux.Handle("/api/jobs/", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.JobStatus)))
	mux.Handle("/api/logs/search", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.LogsSearch)))
	mux.Handle("/api/logs/raw", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.LogsRaw)))
	// Передаём в Grafana путь с префиксом /api/grafana — при serve_from_sub_path=true она так и ожидает
	mux.Handle("/api/grafana/", grafanaAuthMiddleware(handler.JWTSecret, handler.GrafanaProxy()))

	// Логируем необработанные запросы (404)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "" {
			log.Warn(r.Context(), "not found", logging.KV{"method", r.Method}, logging.KV{"path", r.URL.Path})
		}
		http.NotFound(w, r)
	})

	srv := &http.Server{Addr: ":8080", Handler: corsMiddleware(requestIDMiddleware(mux)), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(ctx, "http serve", logging.KV{"error", err})
		}
	}()
	log.Info(ctx, "admin-backend listening on :8080")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		ctx := logging.WithRequestID(r.Context(), reqID)
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authMiddleware(secret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
			return
		}
		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		}
		_, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) { return secret, nil })
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// grafanaAuthMiddleware accepts JWT from Authorization header or from ?token= (for iframe).
// Статика /api/grafana/public/ пускается без JWT (страница Grafana уже открыта по токену).
func grafanaAuthMiddleware(secret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/grafana/public/") || strings.HasPrefix(r.URL.Path, "/api/grafana/img/") {
			next.ServeHTTP(w, r)
			return
		}
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			tokenStr = r.URL.Query().Get("token")
		}
		if tokenStr == "" {
			http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
			return
		}
		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		}
		_, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) { return secret, nil })
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func fileHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
