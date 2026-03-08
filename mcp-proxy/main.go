package main

import (
	"context"
	"embed"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/telegram-ai-assistant/root/mcp-proxy/modules"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

//go:embed prompts/query_extract.txt prompts/answer.txt
var promptFS embed.FS

var log = logging.New("mcp-proxy")

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, config.LoadPostgres().DSN)
	if err != nil {
		log.Error(ctx, "postgres connect", logging.KV{"error", err})
		os.Exit(1)
	}
	defer pool.Close()

	var promptA, promptB string
	if raw, err := promptFS.ReadFile("prompts/query_extract.txt"); err == nil {
		promptA = strings.TrimSpace(string(raw))
	}
	if raw, err := promptFS.ReadFile("prompts/answer.txt"); err == nil {
		promptB = strings.TrimSpace(string(raw))
	}
	if promptA == "" {
		promptA = "Сформулируй короткий поисковый запрос по сообщению пользователя для поиска в базе. Только запрос, без пояснений."
	}
	if promptB == "" {
		promptB = "Ты помощник. Отвечай по контексту ниже. Кратко, на языке вопроса.\n\nКонтекст:"
	}

	srv := modules.NewServer(pool, promptA, promptB)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/chat", srv.HandleChat)

	go func() {
		log.Info(ctx, "mcp-proxy listening on :8083")
		_ = http.ListenAndServe(":8083", mux)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info(ctx, "shutting down")
}
