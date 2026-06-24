package modules

import (
	"fmt"
	"strings"

	"github.com/telegram-ai-assistant/root/pkg/config"
)

// ValidateLLMConfig ensures chat LLM is configured (required). Other models are optional per env.
func ValidateLLMConfig() error {
	model := config.LLMModelRequired()
	if model == "" {
		return fmt.Errorf("LLM_MODEL is required (OpenRouter chat model slug, e.g. qwen/qwen-2.5-72b-instruct)")
	}
	if config.IsOpenRouter() && strings.TrimSpace(config.OpenRouterAPIKey()) == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is required when using OpenRouter API base")
	}
	return nil
}
