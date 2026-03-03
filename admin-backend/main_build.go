// Файл используется только для Docker-сборки: в образ копируется как main.go,
// чтобы избежать дублирования с handler.go при рассинхроне main.go на диске.
package main

import (
	"context"
	"crypto/sha256"
	"errors"
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

	jwtSecret := config.LoadString("JWT_SECRET", "change-me-in-production")
	if jwtSecret == "change-me-in-production" {
		log.Warn(ctx, "JWT_SECRET is default; set in production")
	}
	secretBytes := []byte(jwtSecret)
	if len(secretBytes) < 32 {
		hash := sha256.Sum256([]byte(jwtSecret))
		secretBytes = hash[:]
	}

	jwtExpHours := config.LoadInt("JWT_EXPIRATION_HOURS", 168)
	if jwtExpHours < 1 {
		jwtExpHours = 168
	}
	handler := NewHandler(HandlerDeps{
		Pool:          pool,
		MinIO:         minioClient,
		Queue:         rmq,
		JWTSecret:     secretBytes,
		JWTExpiration: time.Duration(jwtExpHours) * time.Hour,
		AdminUser:     config.LoadString("ADMIN_USER", "admin"),
		AdminPass:     config.LoadString("ADMIN_PASSWORD", "admin"),
		MCPWriteURL:   config.LoadString("MCP_WRITE_URL", "http://mcp-write:8001"),
		LokiURL:       config.LoadString("LOKI_URL", "http://loki:3100"),
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/api/login", handler.Login)
	mux.HandleFunc("/api/login/", handler.Login)
	mux.Handle("/api/upload", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.Upload)))
	mux.Handle("/api/docs", authMiddleware(handler.JWTSecret, docsRouter(handler)))
	mux.Handle("/api/docs/", authMiddleware(handler.JWTSecret, docsRouter(handler)))
	mux.Handle("/api/jobs", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.ListJobs)))
	mux.Handle("/api/jobs/", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.JobStatus)))
	mux.Handle("/api/logs/search", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.LogsSearch)))
	mux.Handle("/api/logs/raw", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.LogsRaw)))
	mux.Handle("/api/chats", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.ListChats)))
	mux.Handle("/api/chats/", authMiddleware(handler.JWTSecret, chatsRouter(handler)))
	mux.Handle("/api/monitor/metrics", authMiddleware(handler.JWTSecret, http.HandlerFunc(handler.MonitorMetrics)))
	mux.Handle("/api/grafana/", grafanaAuthMiddleware(handler.JWTSecret, handler.GrafanaProxy()))

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

func docsRouter(h *Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/api/docs" || path == "/api/docs/" {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			h.ListDocs(w, r)
			return
		}
		if strings.HasPrefix(path, "/api/docs/") {
			h.DocsWithID(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

func chatsRouter(h *Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/messages") && strings.HasPrefix(path, "/api/chats/") {
			h.GetChatMessages(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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
		if tokenStr == "" {
			http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
			return
		}
		_, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) { return secret, nil })
		if err != nil {
			log.Warn(r.Context(), "auth middleware jwt parse failed", logging.KV{"error", err})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			body := `{"error":"invalid token"}`
			if errors.Is(err, jwt.ErrTokenExpired) {
				body = `{"error":"token_expired"}`
			}
			_, _ = w.Write([]byte(body))
			return
		}
		next.ServeHTTP(w, r)
	})
}

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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing authorization","hint":"Open Grafana from the admin panel (log in first, then open Grafana in the menu)"}`))
			return
		}
		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		}
		_, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) { return secret, nil })
		if err != nil {
			log.Warn(r.Context(), "grafana auth jwt parse failed", logging.KV{"error", err})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			body := `{"error":"invalid token"}`
			if errors.Is(err, jwt.ErrTokenExpired) {
				body = `{"error":"token_expired"}`
			}
			_, _ = w.Write([]byte(body))
			return
		}
		next.ServeHTTP(w, r)
	})
}
