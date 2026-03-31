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
	if strings.TrimSpace(s.PromptD) == "" {
		return "", fmt.Errorf("prompt D empty")
	}
	body := contextText
	if len(body) > 24000 {
		body = body[:24000]
	}
	user := "Имя ангела: " + angelName + "\n\nКонтекст:\n" + body
	outC, err := s.callLLMWithBudget(ctx, requestID+"-c", s.PromptC, user, 256)
	if err != nil {
		return "", err
	}
	outD, err := s.callLLMWithBudget(ctx, requestID+"-d", s.PromptD, user, 256)
	if err != nil {
		return "", err
	}
	cleanC := strings.TrimSpace(StripThink(outC))
	cleanD := strings.TrimSpace(StripThink(outD))
	if cleanC == "" {
		return "", nil
	}
	tail := extractDistortionTail(cleanD)
	if tail == "" {
		return cleanC, nil
	}
	if strings.Contains(cleanC, "Его искажения:") {
		return cleanC, nil
	}
	return strings.TrimSpace(cleanC + "\n\n" + tail), nil
}

func extractDistortionTail(s string) string {
	const marker = "Его искажения:"
	i := strings.Index(s, marker)
	if i < 0 {
		// fallback на нижний регистр, если модель изменила регистр
		l := strings.ToLower(s)
		j := strings.Index(l, strings.ToLower(marker))
		if j < 0 {
			return ""
		}
		return strings.TrimSpace(s[j:])
	}
	return strings.TrimSpace(s[i:])
}
