package main

import (
	"context"
	"embed"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/telegram-ai-assistant/root/chat-orchestrator/mcp-proxy/modules"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

//go:embed prompts/query_extract.txt prompts/answer.txt prompts/reminder_compose.txt
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

	var promptA, promptB, promptC string
	tryDir := strings.TrimSpace(os.Getenv("MCP_PROMPTS_DIR"))
	readFromDir := func(name string) (string, bool) {
		if tryDir == "" {
			return "", false
		}
		b, err := os.ReadFile(filepath.Join(tryDir, name))
		if err != nil {
			return "", false
		}
		return strings.TrimSpace(string(b)), true
	}
	var fromDiskA, fromDiskB, fromDiskC bool
	if s, ok := readFromDir("query_extract.txt"); ok {
		promptA, fromDiskA = s, true
	}
	if s, ok := readFromDir("answer.txt"); ok {
		promptB, fromDiskB = s, true
	}
	if s, ok := readFromDir("reminder_compose.txt"); ok {
		promptC, fromDiskC = s, true
	}
	if !fromDiskA && promptA == "" {
		if raw, err := promptFS.ReadFile("prompts/query_extract.txt"); err == nil {
			promptA = strings.TrimSpace(string(raw))
		}
	}
	if !fromDiskB && promptB == "" {
		if raw, err := promptFS.ReadFile("prompts/answer.txt"); err == nil {
			promptB = strings.TrimSpace(string(raw))
		}
	}
	if !fromDiskC && promptC == "" {
		if raw, err := promptFS.ReadFile("prompts/reminder_compose.txt"); err == nil {
			promptC = strings.TrimSpace(string(raw))
		}
	}
	if promptA == "" {
		promptA = "Сформулируй короткий поисковый запрос по сообщению пользователя для поиска в базе. Только запрос, без пояснений."
	}
	if promptB == "" {
		promptB = "Ты помощник. Отвечай по контексту ниже. Кратко, на языке вопроса.\n\nКонтекст:"
	}
	if promptC == "" {
		promptC = "Кратко напомни пользователю об ангеле по контексту."
	}
	srv := modules.NewServer(pool, promptA, promptB, promptC)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/chat", srv.HandleChat)
	mux.HandleFunc("/metrics/llm", srv.HandleLLMStats)
	mux.HandleFunc("/scheduler/compose", srv.HandleSchedulerCompose)
	mux.HandleFunc("/scheduler/deliver", srv.HandleSchedulerDeliver)

	go func() {
		log.Info(ctx, "mcp-proxy listening on :8083")
		_ = http.ListenAndServe(":8083", mux)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info(ctx, "shutting down")
}
