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
	// OpenAI-compatible /v1 (vLLM в Docker или хост): LLM_BINDING_HOST либо VLLM_OPENAI_BASE.
	llmBase := strings.TrimSpace(config.LoadString("LLM_BINDING_HOST", ""))
	if llmBase == "" {
		llmBase = strings.TrimSpace(config.LoadString("VLLM_OPENAI_BASE", ""))
	}
	if llmBase == "" {
		llmBase = "http://vllm:8000/v1"
	}
	llmBase = strings.TrimSuffix(llmBase, "/")
	llmModel := config.LoadString("LLM_MODEL", "Qwen/Qwen3-0.6B")
	llmAPIKey := config.LoadString("LLM_BINDING_API_KEY", "")
	llmMaxTokens := config.LoadInt("LLM_MAX_TOKENS", 1024)
	if llmMaxTokens < 256 {
		llmMaxTokens = 4096
	}
	if llmMaxTokens > 32768 {
		llmMaxTokens = 32768
	}
	llmContextLength := config.LoadInt("LLM_CONTEXT_LENGTH", 8192)
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
