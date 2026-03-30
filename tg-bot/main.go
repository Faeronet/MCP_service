package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/telegram-ai-assistant/root/tg-bot/modules"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
	"github.com/telegram-ai-assistant/root/pkg/ratelimit"
	"github.com/telegram-ai-assistant/root/pkg/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var log = logging.New("tg-bot")

func main() {
	ctx := context.Background()
	token := config.LoadString("TELEGRAM_BOT_TOKEN", "")
	if token == "" {
		log.Warn(ctx, "TELEGRAM_BOT_TOKEN not set; bot will not receive updates")
	}

	pool, err := pgxpool.New(ctx, config.LoadPostgres().DSN)
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
		Bucket:    minioCfg.AttachmentsBucket,
		UseSSL:    minioCfg.UseSSL,
	})
	if err != nil {
		log.Error(ctx, "minio connect", logging.KV{"error", err})
		os.Exit(1)
	}

	proxyURL := strings.TrimSuffix(config.LoadString("MCP_PROXY_URL", "http://mcp-proxy:8083"), "/")
	extractToolURL := strings.TrimSuffix(config.LoadString("EXTRACT_TOOL_URL", config.LoadString("OCR_SERVICE_URL", config.LoadString("ASR_SERVICE_URL", "http://extract-tool:8004"))), "/")
	debounceMs := config.LoadInt("PER_CHAT_DEBOUNCE_MS", 500)
	workerConcurrency := config.LoadInt("WORKER_CONCURRENCY", 64)
	perChatLimiter := ratelimit.NewPerKey(5, 1*time.Minute)
	botDebug := config.LoadInt("BOT_DEBUG", 0)

	app := &modules.Bot{
		Bot:            nil,
		Pool:           pool,
		Minio:          minioClient,
		ProxyURL:       proxyURL,
		ExtractToolURL: extractToolURL,
		Debounce:       time.Duration(debounceMs) * time.Millisecond,
		ChatMu:         make(map[int64]chan struct{}),
		ChatMuGuard:    &sync.Mutex{},
		PerChatLimiter: perChatLimiter,
		DebugMode:      botDebug,
	}

	updatesCh := make(chan tgbotapi.Update, 512)
	for i := 0; i < workerConcurrency; i++ {
		go func() {
			for u := range updatesCh {
				app.HandleUpdate(u)
			}
		}()
	}

	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		_ = http.ListenAndServe(":8081", nil)
	}()

	if token == "" {
		log.Info(ctx, "bot idle (no token); health on :8081")
		select {}
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Error(ctx, "telegram bot init", logging.KV{"error", err})
		os.Exit(1)
	}
	bot.Debug = config.LoadBool("TELEGRAM_DEBUG", false)
	app.Bot = bot
	log.Info(ctx, "bot authorized", logging.KV{"username", bot.Self.UserName})

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	go func() {
		for update := range updates {
			select {
			case updatesCh <- update:
			default:
				log.Warn(ctx, "updates queue full, dropping", logging.KV{"chat_id", modules.GetChatID(update)})
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info(ctx, "shutting down")
}
