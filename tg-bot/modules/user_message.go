package modules

import (
	"strings"
)

const userFriendlyErrorRU = "Упс, с ботом что-то пошло не так. Попробуйте ещё раз."

func userFacingProxyError(debugMode int, err error) string {
	if debugMode == 1 && err != nil {
		return "Ошибка прокси: " + err.Error()
	}
	return userFriendlyErrorRU
}

func sanitizeProxyReplyText(debugMode int, reply string) string {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		if debugMode == 1 {
			return "Пустой ответ от mcp-proxy."
		}
		return userFriendlyErrorRU
	}
	if debugMode == 1 {
		return reply
	}
	if isTechnicalProxyReply(reply) {
		return userFriendlyErrorRU
	}
	return reply
}

func isTechnicalProxyReply(s string) bool {
	lower := strings.ToLower(s)
	markers := []string{
		"ответ модели пустой",
		"модель недоступна",
		"ошибка llm:",
		"запустите mcp-proxy",
		"openrouter_api_key",
		"host.docker.internal",
		"context deadline exceeded",
		"client.timeout",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
