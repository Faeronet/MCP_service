package modules

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/telegram-ai-assistant/root/pkg/queue"
	"github.com/telegram-ai-assistant/root/pkg/storage"
)

// HandlerDeps are dependencies for the admin API handler.
type HandlerDeps struct {
	Pool          *pgxpool.Pool
	MinIO         *storage.Client
	Queue         *queue.Client
	JWTSecret     []byte
	JWTExpiration time.Duration
	AdminUser     string
	AdminPass     string
	MCPWriteURL   string
	MCPProxyURL   string
	LokiURL       string
}

// Handler implements admin API handlers.
type Handler struct {
	HandlerDeps
}

// NewHandler returns a new Handler.
func NewHandler(d HandlerDeps) *Handler {
	return &Handler{HandlerDeps: d}
}
