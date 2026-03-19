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

// truncateUTF8 cuts s to at most maxBytes without breaking a UTF-8 code point.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for len(s) > 0 && (s[len(s)-1]&0xC0) == 0x80 {
		s = s[:len(s)-1]
	}
	return s
}

// llmCharBudget mirrors CallLLM limits: max output tokens and max system prompt size in runes/bytes (same as before: byte length).
func (s *Server) llmCharBudget(userQueryLen int) (maxOut int, systemMaxChars int) {
	maxOut = s.LlmMaxTokens
	if maxOut > s.LlmContextLength-256 {
		maxOut = s.LlmContextLength - 256
		if maxOut < 128 {
			maxOut = 128
		}
	}
	maxInputTokens := s.LlmContextLength - maxOut
	if maxInputTokens <= 0 {
		maxInputTokens = 512
	}
	// Консервативнее 4 символа/токен — кириллица и спецсимволы; иначе vLLM отклоняет запрос, этап A «молча» падает в fallback.
	maxInputChars := maxInputTokens * 3
	margin := 512
	systemMaxChars = maxInputChars - userQueryLen - margin
	if systemMaxChars < 500 {
		systemMaxChars = 500
	}
	return maxOut, systemMaxChars
}

// ComposeAnswerSystem склеивает промпт B и извлечённый CONTEXT так, чтобы данные из Qdrant/Postgres не отрезались:
// answer.txt очень длинный; CallLLM обрезает system по префиксу до лимита — если B шёл первым, CONTEXT оказывался за пределом.
// Здесь под CONTEXT резервируется место, длинный answer.txt укорачивается с конца (оставляем начало с правилами).
func (s *Server) ComposeAnswerSystem(promptB, replyPrefix, contextText string, userQueryLen int) string {
	_, maxTotal := s.llmCharBudget(userQueryLen)
	const sep = "\n\n=== CONTEXT (данные для ответа; единственный источник фактов) ===\n"
	const headSuffix = "\n\n[хвост файла инструкций снят — опирайся на CONTEXT ниже]"
	overhead := len(sep)

	var tail strings.Builder
	if strings.TrimSpace(replyPrefix) != "" {
		tail.WriteString(replyPrefix)
		if strings.TrimSpace(contextText) != "" {
			tail.WriteString("\n")
		}
	}
	tail.WriteString(contextText)
	tailStr := tail.String()

	const wantHeadMin = 2048
	maxTail := maxTotal - overhead - wantHeadMin
	if maxTail < 384 {
		maxTail = maxTotal - overhead - 600
	}
	if maxTail < 128 {
		maxTail = 128
	}
	if len(tailStr) > maxTail {
		tailStr = truncateUTF8(tailStr, maxTail) + "\n[... CONTEXT обрезан по лимиту модели ...]"
	}

	headBudget := maxTotal - overhead - len(tailStr)
	head := promptB
	if len(head) > headBudget {
		trim := headBudget - len(headSuffix)
		if trim < 256 {
			trim = 256
		}
		if trim+len(headSuffix) > headBudget {
			trim = headBudget - len(headSuffix)
			if trim < 100 {
				trim = 100
			}
		}
		head = truncateUTF8(promptB, trim) + headSuffix
	}
	out := head + sep + tailStr
	if len(out) > maxTotal {
		out = truncateUTF8(out, maxTotal)
	}
	return out
}

// ExtractSearchQuery calls LLM with prompt A; returns search query for Qdrant.
// К системному промпту добавляется список имён; бюджет подгоняется под контекст, иначе prompt A (~32KB+) обрезается в CallLLM
// с потерей хвоста и без имён — модель перестаёт выдавать ключ поиска, цепочка выглядит как «сразу только промпт B».
func (s *Server) ExtractSearchQuery(ctx context.Context, requestID, userQuestion string) (string, error) {
	_, systemMax := s.llmCharBudget(len(userQuestion))
	const namesHeader = "\n\nСписок известных имён ангелов-хранителей:\n"
	truncSuffix := "\n\n[промпт извлечения обрезан по лимиту контекста модели]\n"

	namesPart := ""
	if names, err := s.GetAngelNamesList(ctx); err == nil && len(names) > 0 {
		joined := strings.Join(names, "\n")
		maxNames := systemMax / 3
		if maxNames < 1500 {
			maxNames = 1500
		}
		if maxNames > 14000 {
			maxNames = 14000
		}
		if len(joined) > maxNames {
			joined = truncateUTF8(joined, maxNames) + "\n..."
		}
		namesPart = namesHeader + joined
	}

	room := systemMax - len(namesPart)
	if room < 2048 {
		namesPart = ""
		room = systemMax
	}
	pa := s.PromptA
	if len(pa) > room {
		if room <= len(truncSuffix)+64 {
			pa = truncateUTF8(pa, room)
		} else {
			trim := room - len(truncSuffix)
			if trim < 128 {
				trim = 128
			}
			if len(pa) > trim {
				pa = truncateUTF8(pa, trim) + truncSuffix
			}
		}
	}
	systemContent := pa + namesPart
	if len(systemContent) > systemMax {
		systemContent = truncateUTF8(systemContent, systemMax)
	}

	reply, err := s.CallLLMWithBudget(ctx, requestID, systemContent, userQuestion)
	if err != nil {
		return "", err
	}
	reply = StripThink(reply)
	return reply, nil
}

// CallLLM calls OpenAI-compatible /chat/completions (vLLM или другой OpenAI-совместимый сервер).
// Обрезает systemContent по длине, чтобы input_tokens + max_tokens не превышали context_length.
func (s *Server) CallLLM(ctx context.Context, requestID, systemContent, userQuery string) (string, error) {
	return s.CallLLMWithBudget(ctx, requestID, systemContent, userQuery)
}

// CallLLMWithBudget uses the same maxOut/system cap as llmCharBudget (final ответ с длинным промптом B + контекст).
func (s *Server) CallLLMWithBudget(ctx context.Context, requestID, systemContent, userQuery string) (string, error) {
	maxOut, systemMax := s.llmCharBudget(len(userQuery))
	if len(systemContent) > systemMax {
		suffix := "\n\n[... контекст обрезан из-за лимита модели ...]"
		n := systemMax - len(suffix)
		if n < 256 {
			n = 256
		}
		systemContent = truncateUTF8(systemContent, n) + suffix
	}
	messages := []map[string]interface{}{
		{"role": "system", "content": systemContent},
		{"role": "user", "content": userQuery},
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"model": s.LlmModel, "messages": messages, "max_tokens": maxOut,
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
		return "", fmt.Errorf("llm %d: %s", resp.StatusCode, string(bb))
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
