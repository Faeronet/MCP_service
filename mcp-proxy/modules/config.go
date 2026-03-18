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
	VllmBase       string
	LlmModel         string
	LlmAPIKey        string
	LlmMaxTokens     int
	LlmContextLength int   // лимит контекста модели (например 40960); вход обрезается до context_length - max_tokens
	LlmLimiter       *ratelimit.InFlight
	PerChatLimiter *ratelimit.PerKey
	PromptA   string
	PromptB   string
	DebugMode int
}

func NewServer(pool *pgxpool.Pool, promptA, promptB string) *Server {
	mcpReadURL := config.LoadString("MCP_READ_URL", "http://mcp-read:8082")
	// Prefer code-based LLM service (llm-code) over vLLM.
	llmBase := config.LoadString("LLM_BINDING_HOST", "")
	if llmBase == "" {
		llmBase = "http://llm-code:8005/v1"
	}
	// Hard guard: requirement is to avoid vLLM in chat flow.
	if strings.Contains(strings.ToLower(llmBase), "vllm") {
		llmBase = "http://llm-code:8005/v1"
	}
	llmBase = strings.TrimSuffix(llmBase, "/")
	llmModel := config.LoadString("LLM_MODEL", "Qwen/Qwen3-0.6B")
	llmAPIKey := config.LoadString("LLM_BINDING_API_KEY", "")
	llmMaxTokens := config.LoadInt("LLM_MAX_TOKENS", 2048)
	if llmMaxTokens < 256 {
		llmMaxTokens = 4096
	}
	if llmMaxTokens > 32768 {
		llmMaxTokens = 32768
	}
	llmContextLength := config.LoadInt("LLM_CONTEXT_LENGTH", 40960)
	if llmContextLength < 2048 {
		llmContextLength = 40960
	}
	maxInflightLLM := config.LoadInt("MAX_INFLIGHT_LLM", 32)
	llmLimiter := ratelimit.NewInFlight(maxInflightLLM)
	perChatLimiter := ratelimit.NewPerKey(5, 1*time.Minute)
	debugMode := config.LoadInt("BOT_DEBUG", 0)
	return &Server{
		Pool:             pool,
		McpReadURL:       mcpReadURL,
		VllmBase:         llmBase, // Backward-compatible field name.
		LlmModel:         llmModel,
		LlmAPIKey:        llmAPIKey,
		LlmMaxTokens:     llmMaxTokens,
		LlmContextLength: llmContextLength,
		LlmLimiter:       llmLimiter,
		PerChatLimiter:   perChatLimiter,
		PromptA:   promptA,
		PromptB:   promptB,
		DebugMode: debugMode,
	}
}
