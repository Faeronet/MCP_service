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
	LlmModel       string
	LlmAPIKey      string
	LlmMaxTokens   int
	LlmLimiter     *ratelimit.InFlight
	PerChatLimiter *ratelimit.PerKey
	PromptA        string
	PromptB        string
	DebugMode      int
}

func NewServer(pool *pgxpool.Pool, promptA, promptB string) *Server {
	mcpReadURL := config.LoadString("MCP_READ_URL", "http://mcp-read:8082")
	vllmBase := config.LoadString("LLM_BINDING_HOST", "")
	if vllmBase == "" {
		vllmBase = config.LoadString("VLLM_OPENAI_BASE", "http://vllm:8000/v1")
	}
	vllmBase = strings.TrimSuffix(vllmBase, "/")
	llmModel := config.LoadString("LLM_MODEL", "Qwen/Qwen3-0.6B")
	llmAPIKey := config.LoadString("LLM_BINDING_API_KEY", "")
	llmMaxTokens := config.LoadInt("LLM_MAX_TOKENS", 2048)
	if llmMaxTokens < 256 {
		llmMaxTokens = 4096
	}
	if llmMaxTokens > 32768 {
		llmMaxTokens = 32768
	}
	maxInflightLLM := config.LoadInt("MAX_INFLIGHT_LLM", 32)
	llmLimiter := ratelimit.NewInFlight(maxInflightLLM)
	perChatLimiter := ratelimit.NewPerKey(5, 1*time.Minute)
	debugMode := config.LoadInt("BOT_DEBUG", 0)
	return &Server{
		Pool:           pool,
		McpReadURL:     mcpReadURL,
		VllmBase:       vllmBase,
		LlmModel:       llmModel,
		LlmAPIKey:      llmAPIKey,
		LlmMaxTokens:   llmMaxTokens,
		LlmLimiter:     llmLimiter,
		PerChatLimiter: perChatLimiter,
		PromptA:        promptA,
		PromptB:        promptB,
		DebugMode:      debugMode,
	}
}
