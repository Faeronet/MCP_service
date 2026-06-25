package main

import (
	"context"
	"crypto/sha256"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
	"github.com/telegram-ai-assistant/root/pkg/queue"
	"github.com/telegram-ai-assistant/root/pkg/storage"

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

	jwtExpHours := config.LoadInt("JWT_EXPIRATION_HOURS", 168) // 168 = 7 дней
	if jwtExpHours < 1 {
		jwtExpHours = 168
	}

	kcIssuer := normalizeKeycloakIssuer(config.LoadString("KEYCLOAK_ISSUER", ""))
	kcJWKSURL := strings.TrimSpace(config.LoadString("KEYCLOAK_JWKS_URL", ""))
	if kcJWKSURL == "" && kcIssuer != "" {
		kcJWKSURL = kcIssuer + "/protocol/openid-connect/certs"
	}
	var kcKF keyfunc.Keyfunc
	if kcIssuer != "" && kcJWKSURL != "" {
		kf, kerr := newKeycloakKeyfunc(ctx, kcJWKSURL)
		if kerr != nil {
			log.Warn(ctx, "KEYCLOAK_ISSUER set but JWKS init failed (Keycloak auth disabled)", logging.KV{"error", kerr})
		} else {
			kcKF = kf
		}
	}

	handler := NewHandler(HandlerDeps{
		Pool:                  pool,
		MinIO:                 minioClient,
		Queue:                 rmq,
		JWTSecret:             secretBytes,
		JWTExpiration:         time.Duration(jwtExpHours) * time.Hour,
		AdminUser:             config.LoadString("ADMIN_USER", "admin"),
		AdminPass:             config.LoadString("ADMIN_PASSWORD", "admin"),
		MCPWriteURL:           config.LoadString("MCP_WRITE_URL", "http://mcp-write:8001"),
		MCPProxyURL:           strings.TrimSuffix(config.LoadString("MCP_PROXY_URL", "http://mcp-proxy:8083"), "/"),
		LokiURL:               config.LoadString("LOKI_URL", "http://loki:3100"),
		ReminderSuperAdminSub: config.LoadString("REMINDER_SUPERADMIN_SUB", "admin"),
		ZoneAgents:            parseZoneAgentsJSON(config.LoadString("ZONE_AGENTS", "")),
		KeycloakIssuer:        kcIssuer,
		KeycloakClientID:      strings.TrimSpace(config.LoadString("KEYCLOAK_CLIENT_ID", "")),
		KeycloakClientSecret:  strings.TrimSpace(config.LoadString("KEYCLOAK_CLIENT_SECRET", "")),
		KeycloakRequiredRole:       strings.TrimSpace(config.LoadString("KEYCLOAK_REQUIRED_ROLE", "")),
		KeycloakOnly:               strings.EqualFold(strings.TrimSpace(config.LoadString("KEYCLOAK_ONLY", "")), "1"),
		KeycloakKeyfunc:            kcKF,
		KeycloakTokenURL:           strings.TrimSpace(config.LoadString("KEYCLOAK_TOKEN_URL", "")),
		KeycloakAuthorizationURL:   strings.TrimSpace(config.LoadString("KEYCLOAK_AUTHORIZATION_URL", "")),
	})
	handler.StartChatLogStatsSampler()

	ipAllow := loadIPAllowlist()
	if ipAllow.enabled {
		log.Info(ctx, "admin IP allowlist enabled", logging.KV{"rules", config.LoadString("ADMIN_ALLOWED_IPS", "")})
	} else {
		log.Warn(ctx, "ADMIN_ALLOWED_IPS not set — admin API accepts any client IP")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/api/login", handler.Login)
	mux.HandleFunc("/api/login/", handler.Login)
	mux.HandleFunc("/api/auth/keycloak", handler.KeycloakPublicConfig)
	mux.HandleFunc("/api/auth/keycloak/", handler.KeycloakPublicConfig)
	mux.HandleFunc("/api/auth/keycloak/callback", handler.KeycloakCallback)
	mux.HandleFunc("/api/auth/keycloak/callback/", handler.KeycloakCallback)
	mux.Handle("/api/upload", handler.AuthMiddleware(http.HandlerFunc(handler.Upload)))
	mux.Handle("/api/docs", handler.AuthMiddleware(docsRouter(handler)))
	mux.Handle("/api/docs/", handler.AuthMiddleware(docsRouter(handler)))
	mux.Handle("/api/jobs", handler.AuthMiddleware(http.HandlerFunc(handler.ListJobs)))
	mux.Handle("/api/jobs/", handler.AuthMiddleware(http.HandlerFunc(handler.JobStatus)))
	mux.Handle("/api/logs/search", handler.AuthMiddleware(http.HandlerFunc(handler.LogsSearch)))
	mux.Handle("/api/logs/raw", handler.AuthMiddleware(http.HandlerFunc(handler.LogsRaw)))
	mux.Handle("/api/chats", handler.AuthMiddleware(http.HandlerFunc(handler.ListChats)))
	mux.Handle("/api/chats/stats", handler.AuthMiddleware(http.HandlerFunc(handler.ChatLogStats)))
	mux.Handle("/api/chats/", handler.AuthMiddleware(chatsRouter(handler)))
	mux.Handle("/api/monitor/metrics", handler.AuthMiddleware(http.HandlerFunc(handler.MonitorMetrics)))
	mux.Handle("/api/reminders/config", handler.AuthMiddleware(http.HandlerFunc(handler.RemindersConfig)))
	mux.Handle("/api/reminders/toggle", handler.AuthMiddleware(http.HandlerFunc(handler.RemindersToggle)))
	mux.Handle("/api/reminders/debug-clock", handler.AuthMiddleware(http.HandlerFunc(handler.RemindersDebugClock)))
	mux.Handle("/api/reminders/subscribers", handler.AuthMiddleware(http.HandlerFunc(handler.RemindersSubscribers)))
	mux.Handle("/api/reminders/reset-user", handler.AuthMiddleware(http.HandlerFunc(handler.RemindersResetUser)))
	mux.Handle("/api/reminders/scheduler-notifications", handler.AuthMiddleware(schedulerNotificationsRouter(handler)))
	mux.Handle("/api/reminders/scheduler-notifications/", handler.AuthMiddleware(schedulerNotificationsRouter(handler)))
	mux.Handle("/api/chat/llm", handler.AuthMiddleware(http.HandlerFunc(handler.ChatLLM)))
	mux.Handle("/api/grafana/", handler.GrafanaAuthMiddleware(http.StripPrefix("/api/grafana", handler.GrafanaProxy())))
	mux.Handle("/api/zones", handler.AuthMiddleware(http.HandlerFunc(handler.ZonesRoutes)))
	mux.Handle("/api/zones/", handler.AuthMiddleware(http.HandlerFunc(handler.ZonesRoutes)))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "" {
			log.Warn(r.Context(), "not found", logging.KV{"method", r.Method}, logging.KV{"path", r.URL.Path})
		}
		http.NotFound(w, r)
	})

	srv := &http.Server{Addr: ":8080", Handler: ipAllowMiddleware(ipAllow, corsMiddleware(requestIDMiddleware(mux))), ReadHeaderTimeout: 10 * time.Second}
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

// docsRouter обрабатывает /api/docs и /api/docs/<id>: список или операция по id. Нужен отдельный роутер,
// чтобы DELETE /api/docs/<id> гарантированно попадал в обработчик (в т.ч. под Go 1.22 ServeMux).
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

func schedulerNotificationsRouter(h *Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimSuffix(r.URL.Path, "/")
		switch p {
		case "/api/reminders/scheduler-notifications":
			if r.Method == http.MethodGet {
				h.SchedulerNotificationsList(w, r)
				return
			}
		case "/api/reminders/scheduler-notifications/cancel":
			if r.Method == http.MethodPost {
				h.SchedulerNotificationCancel(w, r)
				return
			}
		case "/api/reminders/scheduler-notifications/delete":
			if r.Method == http.MethodPost {
				h.SchedulerNotificationDelete(w, r)
				return
			}
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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

