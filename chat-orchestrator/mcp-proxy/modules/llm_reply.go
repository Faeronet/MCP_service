package modules

import (
	"context"
	"errors"
	"strings"

	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var errEmptyLLMReply = errors.New("empty llm reply after strip")

func (s *Server) cleanLLMReply(reply string) string {
	if s.DebugMode == 1 {
		return strings.TrimSpace(reply)
	}
	return StripThink(reply)
}

func (s *Server) finalizeLLMReply(ctx context.Context, requestID, systemContent, userQuery string, history []LLMChatMessage) (string, error) {
	reply, err := s.CallLLM(ctx, requestID, systemContent, userQuery, history)
	reply = s.cleanLLMReply(reply)
	if err == nil && strings.TrimSpace(reply) != "" {
		return reply, nil
	}
	if err != nil {
		logHandler.Warn(ctx, "llm call failed, no retry", logging.KV{"request_id", requestID}, logging.KV{"error", err})
		return "", err
	}

	logHandler.Warn(ctx, "llm reply empty after strip, retrying once", logging.KV{"request_id", requestID})
	retrySystem := systemContent + "\n\nВажно: ответь пользователю сразу, без внутренних рассуждений и без блоков think/reasoning."
	reply, err = s.CallLLM(ctx, requestID+"-retry", retrySystem, userQuery, history)
	reply = s.cleanLLMReply(reply)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(reply) == "" {
		return "", errEmptyLLMReply
	}
	return reply, nil
}
