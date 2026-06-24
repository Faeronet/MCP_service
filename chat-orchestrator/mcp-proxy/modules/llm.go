package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/telegram-ai-assistant/root/pkg/config"
)

// LLMChatMessage — одно сообщение в истории диалога для multi-turn chat.
type LLMChatMessage struct {
	Role    string
	Content string
}

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

// llmCharBudget: maxOutCap 0 = LlmMaxTokens; иначе верхняя граница max_tokens (для короткого ответа промпта A).
// Оценка символов → токенов занижена (кириллица, Qwen): иначе vLLM 400 при max-model-len 2048–8192.
func (s *Server) llmCharBudget(userQueryLen int, maxOutCap int) (maxOut int, systemMaxChars int) {
	maxOut = s.LlmMaxTokens
	if maxOutCap > 0 && maxOutCap < maxOut {
		maxOut = maxOutCap
	}
	if maxOut > s.LlmContextLength-256 {
		maxOut = s.LlmContextLength - 256
		if maxOut < 128 {
			maxOut = 128
		}
	}
	// Запас под служебные токены чата/шаблона и расхождение с токенайзером vLLM.
	const slackTokens = 128
	maxInputTokens := s.LlmContextLength - maxOut - slackTokens
	if maxInputTokens < 256 {
		maxInputTokens = 256
	}
	// Пользовательское сообщение: консервативно ~1 токен на 2 байта UTF-8 (RU часто хуже).
	userTokEst := userQueryLen / 2
	if userTokEst < 8 {
		userTokEst = 8
	}
	systemTokBudget := maxInputTokens - userTokEst
	if systemTokBudget < 96 {
		systemTokBudget = 96
	}
	// Верхняя граница символов system: не более ~1.5 символа на токен для длинных русских промптов.
	const approxCharsPerToken = 1.5
	systemMaxChars = int(float64(systemTokBudget) * approxCharsPerToken)
	if systemMaxChars < 400 {
		systemMaxChars = 400
	}
	return maxOut, systemMaxChars
}

// ComposeAnswerSystem склеивает промпт B и извлечённый CONTEXT так, чтобы данные из Qdrant/Postgres не отрезались:
// answer.txt очень длинный; CallLLM обрезает system по префиксу до лимита — если B шёл первым, CONTEXT оказывался за пределом.
// Здесь под CONTEXT резервируется место, длинный answer.txt укорачивается с конца (оставляем начало с правилами).
func (s *Server) ComposeAnswerSystem(promptB, replyPrefix, contextText string, userQueryLen int) string {
	_, maxTotal := s.llmCharBudget(userQueryLen, 0)
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
	_, systemMax := s.llmCharBudget(len(userQuestion), s.LlmExtractMaxTokens)
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

	reply, err := s.callLLMWithBudget(ctx, requestID, systemContent, userQuestion, nil, s.LlmExtractMaxTokens)
	if err != nil {
		return "", err
	}
	reply = StripThink(reply)
	return reply, nil
}

// CallLLM calls OpenAI-compatible /chat/completions with optional session history.
func (s *Server) CallLLM(ctx context.Context, requestID, systemContent, userQuery string, history []LLMChatMessage) (string, error) {
	return s.callLLMWithBudget(ctx, requestID, systemContent, userQuery, history, 0)
}

// CallLLMWithBudget — финальный ответ (max_tokens = LlmMaxTokens).
func (s *Server) CallLLMWithBudget(ctx context.Context, requestID, systemContent, userQuery string) (string, error) {
	return s.callLLMWithBudget(ctx, requestID, systemContent, userQuery, nil, 0)
}

func (s *Server) callLLMWithBudget(ctx context.Context, requestID, systemContent, userQuery string, history []LLMChatMessage, maxOutCap int) (string, error) {
	maxOut, systemMax := s.llmCharBudget(len(userQuery), maxOutCap)
	if len(systemContent) > systemMax {
		suffix := "\n\n[... контекст обрезан из-за лимита модели ...]"
		n := systemMax - len(suffix)
		if n < 256 {
			n = 256
		}
		systemContent = truncateUTF8(systemContent, n) + suffix
	}
	history = trimHistoryForBudget(history, systemContent, userQuery, s.LlmContextLength, maxOut)
	messages := make([]map[string]interface{}, 0, 2+len(history))
	messages = append(messages, map[string]interface{}{"role": "system", "content": systemContent})
	for _, m := range history {
		role := strings.TrimSpace(m.Role)
		content := strings.TrimSpace(m.Content)
		if role == "" || content == "" {
			continue
		}
		if role != "user" && role != "assistant" {
			continue
		}
		messages = append(messages, map[string]interface{}{"role": role, "content": content})
	}
	messages = append(messages, map[string]interface{}{"role": "user", "content": userQuery})

	payloadMap := map[string]interface{}{
		"model":      s.LlmModel,
		"messages":   messages,
		"max_tokens": maxOut,
	}
	if !strings.Contains(strings.ToLower(s.LlmModel), "qwen") {
		// chat_template_kwargs — только для локального vLLM + Qwen
	} else if !config.IsOpenRouter() {
		payloadMap["chat_template_kwargs"] = map[string]interface{}{"enable_thinking": false}
	}
	payload, _ := json.Marshal(payloadMap)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.VllmBase+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	if s.LlmAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.LlmAPIKey)
	}
	setOpenRouterHeaders(req)
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

// trimHistoryForBudget removes oldest turns so system + history + user fit context window.
func trimHistoryForBudget(history []LLMChatMessage, systemContent, userQuery string, contextLen, maxOut int) []LLMChatMessage {
	if len(history) == 0 {
		return history
	}
	const approxCharsPerToken = 2
	maxInputChars := (contextLen - maxOut - 128) * approxCharsPerToken
	if maxInputChars < 512 {
		maxInputChars = 512
	}
	used := len(systemContent) + len(userQuery)
	out := make([]LLMChatMessage, 0, len(history))
	for i := len(history) - 1; i >= 0; i-- {
		m := history[i]
		cost := len(m.Content) + 16
		if used+cost > maxInputChars {
			break
		}
		out = append(out, m)
		used += cost
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
