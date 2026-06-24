package modules

import (
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/ratelimit"
)

// Server holds config and clients for mcp-proxy (chat logic, no Telegram).
type Server struct {
	Pool           *pgxpool.Pool
	McpReadURL     string
	SchedulerURL   string
	VllmBase       string
	LlmModel         string
	LlmAPIKey        string
	LlmMaxTokens     int
	LlmContextLength int   // лимит контекста модели (например 40960); вход обрезается до context_length - max_tokens
	LlmLimiter       *ratelimit.InFlight
	PerChatLimiter *ratelimit.PerKey
	PromptA   string
	PromptB   string
	PromptC   string
	DebugMode int
	// QueryExtractMode: always | never | no_date (по умолчанию no_date — без LLM-этапа A, если в тексте есть дата).
	QueryExtractMode string
	// LlmExtractMaxTokens — max_tokens только для промпта A (короткая строка); меньше = быстрее декодирование.
	LlmExtractMaxTokens int
	ChatHistoryMaxMessages int
	TelegramBotToken string
}

func NewServer(pool *pgxpool.Pool, promptA, promptB, promptC string) *Server {
	mcpReadURL := config.LoadString("MCP_READ_URL", "http://mcp-read:8082")
	llmBase := config.OpenRouterBase()
	llmModel := config.LLMModelRequired()
	llmAPIKey := config.OpenRouterAPIKey()
	llmMaxTokens := config.LoadInt("LLM_MAX_TOKENS", 1024)
	if llmMaxTokens < 256 {
		llmMaxTokens = 4096
	}
	if llmMaxTokens > 32768 {
		llmMaxTokens = 32768
	}
	// Совпадайте с --max-model-len у vLLM (или задайте VLLM_MAX_MODEL_LEN в .env и передайте в mcp-proxy).
	llmContextLength := config.LoadInt("LLM_CONTEXT_LENGTH", 0)
	if llmContextLength < 512 {
		llmContextLength = config.LoadInt("VLLM_MAX_MODEL_LEN", 8192)
	}
	if llmContextLength < 512 {
		llmContextLength = 8192
	}
	// max_tokens не может занимать весь контекст — иначе vLLM: «maximum input length of 0 tokens».
	if llmMaxTokens >= llmContextLength {
		llmMaxTokens = llmContextLength / 2
		if llmMaxTokens < 256 {
			llmMaxTokens = 256
		}
	}
	if llmMaxTokens > llmContextLength-256 {
		llmMaxTokens = llmContextLength - 256
		if llmMaxTokens < 128 {
			llmMaxTokens = 128
		}
	}
	maxInflightLLM := config.LoadInt("MAX_INFLIGHT_LLM", 32)
	llmLimiter := ratelimit.NewInFlight(maxInflightLLM)
	perChatLimiter := ratelimit.NewPerKey(5, 1*time.Minute)
	debugMode := config.LoadInt("BOT_DEBUG", 0)
	queryExtractMode := strings.ToLower(strings.TrimSpace(config.LoadString("LLM_QUERY_EXTRACT", "no_date")))
	if queryExtractMode == "" {
		queryExtractMode = "no_date"
	}
	extractMax := config.LoadInt("LLM_EXTRACT_MAX_TOKENS", 256)
	if extractMax < 32 {
		extractMax = 32
	}
	if extractMax > 2048 {
		extractMax = 2048
	}
	historyMax := config.LoadInt("CHAT_HISTORY_MAX_MESSAGES", 40)
	if historyMax < 0 {
		historyMax = 0
	}
	if historyMax > 200 {
		historyMax = 200
	}
	return &Server{
		Pool:             pool,
		McpReadURL:       mcpReadURL,
		SchedulerURL:     strings.TrimSuffix(config.LoadString("SCHEDULER_INTERNAL_URL", "http://scheduler:8090"), "/"),
		VllmBase:         llmBase, // Backward-compatible field name.
		LlmModel:         llmModel,
		LlmAPIKey:        llmAPIKey,
		LlmMaxTokens:     llmMaxTokens,
		LlmContextLength: llmContextLength,
		LlmLimiter:       llmLimiter,
		PerChatLimiter:   perChatLimiter,
		PromptA:   promptA,
		PromptB:   promptB,
		PromptC:   promptC,
		DebugMode: debugMode,
		QueryExtractMode:      queryExtractMode,
		LlmExtractMaxTokens:    extractMax,
		ChatHistoryMaxMessages: historyMax,
		TelegramBotToken:       strings.TrimSpace(config.LoadString("TELEGRAM_BOT_TOKEN", "")),
	}
}
