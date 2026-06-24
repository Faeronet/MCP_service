package config

import (
	"net/http"
	"strings"
)

const defaultOpenRouterBase = "https://openrouter.ai/api/v1"

// OpenRouterBase returns OpenAI-compatible API base (OpenRouter or legacy override).
func OpenRouterBase() string {
	if v := strings.TrimSpace(LoadString("OPENROUTER_API_BASE", "")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	if v := strings.TrimSpace(LoadString("OPENROUTER_BASE_URL", "")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	if v := strings.TrimSpace(LoadString("LLM_BINDING_HOST", "")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	if v := strings.TrimSpace(LoadString("VLLM_OPENAI_BASE", "")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	return defaultOpenRouterBase
}

// OpenRouterAPIKey — единый ключ для всех моделей OpenRouter (fallback на legacy per-service keys).
func OpenRouterAPIKey() string {
	if v := strings.TrimSpace(LoadString("OPENROUTER_API_KEY", "")); v != "" {
		return v
	}
	return strings.TrimSpace(LoadString("LLM_BINDING_API_KEY", ""))
}

// IsOpenRouter returns true when API base points to OpenRouter.
func IsOpenRouter() bool {
	base := strings.ToLower(OpenRouterBase())
	return strings.Contains(base, "openrouter.ai")
}

// OptionalModel returns trimmed model name or empty (service disabled).
func OptionalModel(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(LoadString(k, "")); v != "" {
			return v
		}
	}
	return ""
}

// LLMModelRequired returns configured chat model or empty.
func LLMModelRequired() string {
	return OptionalModel("LLM_MODEL", "OPENROUTER_LLM_MODEL")
}

// SetOpenRouterHeaders adds optional OpenRouter attribution headers.
func SetOpenRouterHeaders(req *http.Request) {
	if !IsOpenRouter() {
		return
	}
	if ref := strings.TrimSpace(LoadString("OPENROUTER_HTTP_REFERER", "")); ref != "" {
		req.Header.Set("HTTP-Referer", ref)
	}
	if title := strings.TrimSpace(LoadString("OPENROUTER_APP_TITLE", "MCP Telegram Assistant")); title != "" {
		req.Header.Set("X-Title", title)
	}
}
