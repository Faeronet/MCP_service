package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/telegram-ai-assistant/root/pkg/logging"
	"github.com/telegram-ai-assistant/root/notification/scheduler/modules"
)

var log = logging.New("scheduler")

func main() {
	ctx := context.Background()
	cfg := modules.LoadConfig()
	if cfg.InternalSecret == "" {
		log.Warn(ctx, "SCHEDULER_INTERNAL_SECRET empty; set it in production to protect bot internal API")
	}

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Error(ctx, "postgres", logging.KV{"error", err})
		os.Exit(1)
	}
	defer pool.Close()

	bot := modules.NewBotClient(cfg.McpProxyURL, cfg.InternalSecret)
	srv := modules.NewServer(pool, cfg, bot)

	ctxStop, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	go modules.RunDispatcher(ctxStop, pool, cfg, bot)

	log.Info(ctx, "scheduler listen", logging.KV{"addr", cfg.ListenAddr})
	if err := srv.Listen(cfg.ListenAddr); err != nil {
		log.Error(ctx, "http", logging.KV{"error", err})
		os.Exit(1)
	}
}
