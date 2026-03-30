package modules

import (
	"context"
	"fmt"
	"strings"
)

// composeReminderLLM — промпт C для сервиса scheduler (POST /scheduler/compose).
func (s *Server) composeReminderLLM(ctx context.Context, requestID, angelName, contextText string) (string, error) {
	if strings.TrimSpace(s.PromptC) == "" {
		return "", fmt.Errorf("prompt C empty")
	}
	body := contextText
	if len(body) > 24000 {
		body = body[:24000]
	}
	user := "Имя ангела: " + angelName + "\n\nКонтекст:\n" + body
	out, err := s.callLLMWithBudget(ctx, requestID, s.PromptC, user, 256)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(StripThink(out)), nil
}
