package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"os/signal"
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

	rmq, err := queue.New(ctx, config.LoadRabbitMQ().URL)
	if err != nil {
		log.Error(ctx, "rabbitmq connect", logging.KV{"error", err})
		os.Exit(1)
	}
	defer rmq.Close()

	jwtSecret := config.LoadString("JWT_SECRET", "change-me")
	adminUser := config.LoadString("ADMIN_USER", "admin")
	adminPass := config.LoadString("ADMIN_PASSWORD", "admin")
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
	mux.Handle("/api/upload", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.Upload)))
	mux.Handle("/api/docs", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.ListDocs)))
	mux.Handle("/api/jobs", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.ListJobs)))
	mux.Handle("/api/jobs/", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.JobStatus)))
	mux.Handle("/api/logs/search", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.LogsSearch)))
	mux.Handle("/api/logs/raw", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.LogsRaw)))
	mux.Handle("/api/grafana/", authMiddleware(handler.JWTSecret, http.StripPrefix("/api/grafana", handler.GrafanaProxy())))

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

func fileHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
