package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ExtractSearchQuery calls LLM with prompt A; returns search query for Qdrant.
// К системному промпту добавляется полный список имён ангелов из БД (без количества).
func (s *Server) ExtractSearchQuery(ctx context.Context, requestID, userQuestion string) (string, error) {
	systemContent := s.PromptA
	if names, err := s.GetAngelNamesList(ctx); err == nil && len(names) > 0 {
		systemContent = s.PromptA + "\n\nСписок известных имён ангелов-хранителей:\n" + strings.Join(names, "\n")
	}
	reply, err := s.CallLLM(ctx, requestID, systemContent, userQuestion)
	if err != nil {
		return "", err
	}
	reply = StripThink(reply)
	return reply, nil
}

// CallLLM calls vLLM chat completion; systemContent = system prompt, userQuery = user message.
// Обрезает systemContent по длине, чтобы input_tokens + max_tokens не превышали context_length (vLLM 400).
func (s *Server) CallLLM(ctx context.Context, requestID, systemContent, userQuery string) (string, error) {
	maxInputTokens := s.LlmContextLength - s.LlmMaxTokens
	if maxInputTokens <= 0 {
		maxInputTokens = 38912
	}
	// Приблизительно ~4 символа на токен (кириллица/латиница)
	maxInputChars := maxInputTokens * 4
	margin := 256
	systemMax := maxInputChars - len(userQuery) - margin
	if systemMax < 500 {
		systemMax = 500
	}
	if len(systemContent) > systemMax {
		systemContent = systemContent[:systemMax] + "\n\n[... контекст обрезан из-за лимита модели ...]"
	}
	messages := []map[string]interface{}{
		{"role": "system", "content": systemContent},
		{"role": "user", "content": userQuery},
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"model": s.LlmModel, "messages": messages, "max_tokens": s.LlmMaxTokens,
		"chat_template_kwargs": map[string]interface{}{"enable_thinking": false},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.VllmBase+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	if s.LlmAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.LlmAPIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vllm %d: %s", resp.StatusCode, string(bb))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("no choices")
	}
	return out.Choices[0].Message.Content, nil
}
