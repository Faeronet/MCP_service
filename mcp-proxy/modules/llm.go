package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ExtractSearchQuery calls LLM with prompt A; returns search query for Qdrant.
func (s *Server) ExtractSearchQuery(ctx context.Context, requestID, userQuestion string) (string, error) {
	reply, err := s.CallLLM(ctx, requestID, s.PromptA, userQuestion)
	if err != nil {
		return "", err
	}
	reply = StripThink(reply)
	return reply, nil
}

// CallLLM calls vLLM chat completion; systemContent = system prompt, userQuery = user message.
func (s *Server) CallLLM(ctx context.Context, requestID, systemContent, userQuery string) (string, error) {
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
