package modules

import (
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/telegram-ai-assistant/root/pkg/ratelimit"
	"github.com/telegram-ai-assistant/root/pkg/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot holds Telegram bot API, DB, MinIO, proxy URL and state for the Telegram-only service.
type Bot struct {
	Bot              *tgbotapi.BotAPI
	Pool             *pgxpool.Pool
	Minio            *storage.Client
	ProxyURL         string
	ExtractToolURL   string
	Debounce         time.Duration
	ChatMu           map[int64]chan struct{}
	ChatMuGuard      *sync.Mutex
	PerChatLimiter   *ratelimit.PerKey
}
