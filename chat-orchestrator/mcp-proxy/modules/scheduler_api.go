package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode"
)

type schedulerComposeReq struct {
	AngelName   string `json:"angel_name"`
	ContextText string `json:"context_text"`
	RequestID   string `json:"request_id"`
}

type schedulerDeliverReq struct {
	TelegramID   int64  `json:"telegram_id"`
	ChatID       int64  `json:"chat_id"`
	Text         string `json:"text"`
	AngelChunkID string `json:"angel_chunk_id,omitempty"`
	AngelName    string `json:"angel_name,omitempty"`
	RequestID    string `json:"request_id"`
	// SkipChatMemory: при true не пишем второй assistant в chat.messages (ответ уже сохранён в HandleChat).
	SkipChatMemory bool `json:"skip_chat_memory,omitempty"`
}

func (s *Server) schedulerInternalAuthorized(r *http.Request) bool {
	want := strings.TrimSpace(os.Getenv("SCHEDULER_INTERNAL_SECRET"))
	if want == "" {
		return true
	}
	return strings.TrimSpace(r.Header.Get("X-Scheduler-Secret")) == want
}

// HandleSchedulerCompose POST /scheduler/compose
func (s *Server) HandleSchedulerCompose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.schedulerInternalAuthorized(r) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req schedulerComposeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.AngelName)
	ctxText := strings.TrimSpace(req.ContextText)
	if name == "" || ctxText == "" {
		http.Error(w, `{"error":"angel_name and context_text required"}`, http.StatusBadRequest)
		return
	}
	uid := strings.TrimSpace(req.RequestID)
	if uid == "" {
		uid = "scheduler-compose"
	}
	out, err := s.composeReminderLLM(r.Context(), uid, name, ctxText)
	if err != nil || strings.TrimSpace(out) == "" {
		fallback := "Напоминание: сегодня день ангела " + name + "."
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"reminder_text": fallback, "fallback": true})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"reminder_text": strings.TrimSpace(out), "fallback": false})
}

// splitPhotoCaptionAndRemainder — подпись к фото (лимит Telegram 1024 символа), остаток для sendMessage; разрыв по пробелу, если есть.
func splitPhotoCaptionAndRemainder(full string, maxCaptionRunes int) (caption string, remainder string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	rs := []rune(full)
	if len(rs) <= maxCaptionRunes {
		return full, ""
	}
	half := maxCaptionRunes / 2
	splitAt := maxCaptionRunes
	for i := maxCaptionRunes - 1; i >= half; i-- {
		if unicode.IsSpace(rs[i]) {
			splitAt = i + 1
			break
		}
	}
	if splitAt == maxCaptionRunes {
		for i := half - 1; i >= 0; i-- {
			if unicode.IsSpace(rs[i]) {
				splitAt = i + 1
				break
			}
		}
	}
	caption = strings.TrimSpace(string(rs[:splitAt]))
	remainder = strings.TrimSpace(string(rs[splitAt:]))
	return caption, remainder
}

// HandleSchedulerDeliver POST /scheduler/deliver
func (s *Server) HandleSchedulerDeliver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.schedulerInternalAuthorized(r) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req schedulerDeliverReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(req.Text)
	if req.ChatID == 0 || text == "" {
		http.Error(w, `{"error":"chat_id and text required"}`, http.StatusBadRequest)
		return
	}
	if s.TelegramBotToken == "" {
		http.Error(w, `{"error":"TELEGRAM_BOT_TOKEN not configured in mcp-proxy"}`, http.StatusServiceUnavailable)
		return
	}

	imgPath := s.ResolveAngelImagePathForScheduler(r.Context(), req.AngelChunkID, req.AngelName)
	var msgID int
	if imgPath != "" {
		capText, rest := splitPhotoCaptionAndRemainder(text, TelegramPhotoCaptionMax)
		var photoErr error
		msgID, photoErr = s.TelegramSendPhoto(r.Context(), req.ChatID, imgPath, capText)
		if photoErr != nil || msgID == 0 {
			msgID = 0
		} else if strings.TrimSpace(rest) != "" {
			if _, err := s.sendTelegramTextChunks(r.Context(), req.ChatID, rest); err != nil {
				enc, _ := json.Marshal(map[string]string{"error": err.Error()})
				http.Error(w, string(enc), http.StatusBadGateway)
				return
			}
		}
	}
	if msgID == 0 {
		var err error
		msgID, err = s.sendTelegramTextChunks(r.Context(), req.ChatID, text)
		if err != nil {
			enc, _ := json.Marshal(map[string]string{"error": err.Error()})
			http.Error(w, string(enc), http.StatusBadGateway)
			return
		}
	}
	out := struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}{}
	out.Result.MessageID = msgID
	if out.Result.MessageID != 0 && !req.SkipChatMemory {
		// Напоминания из notification: пишем в chat.messages. После /chat ответ уже в БД + answer_context — дублировать нельзя.
		_ = s.SaveSchedulerReminderMemory(
			r.Context(),
			req.TelegramID,
			req.ChatID,
			out.Result.MessageID,
			text,
			req.AngelChunkID,
		)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message_id": out.Result.MessageID})
}

// Лимит Telegram Bot API на одно текстовое сообщение (символы Unicode).
const maxTelegramMessageRunes = 4096

func splitTelegramMessageChunks(text string, maxRunes int) []string {
	if maxRunes < 256 {
		maxRunes = maxTelegramMessageRunes
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	rs := []rune(text)
	if len(rs) <= maxRunes {
		return []string{text}
	}

	var chunks []string
	var b strings.Builder
	b.Grow(min(maxRunes*5, 65536))
	n := 0

	flush := func() {
		s := strings.TrimRight(b.String(), " \t")
		if strings.TrimSpace(s) != "" {
			chunks = append(chunks, s)
		}
		b.Reset()
		n = 0
	}

	appendRunes := func(r []rune) {
		b.WriteString(string(r))
		n += len(r)
	}

	emitOversizedWord := func(word []rune) {
		for len(word) > 0 {
			if n > 0 {
				flush()
			}
			take := maxRunes
			if take > len(word) {
				take = len(word)
			}
			chunks = append(chunks, string(word[:take]))
			word = word[take:]
		}
	}

	i := 0
	for i < len(rs) {
		wsStart := i
		for i < len(rs) && unicode.IsSpace(rs[i]) {
			i++
		}
		ws := rs[wsStart:i]

		wordStart := i
		for i < len(rs) && !unicode.IsSpace(rs[i]) {
			i++
		}
		word := rs[wordStart:i]

		for len(ws) > 0 {
			if n >= maxRunes {
				flush()
			}
			avail := maxRunes - n
			if avail == 0 {
				continue
			}
			take := len(ws)
			if take > avail {
				take = avail
			}
			appendRunes(ws[:take])
			ws = ws[take:]
		}

		if len(word) == 0 {
			continue
		}
		wlen := len(word)
		if wlen > maxRunes {
			emitOversizedWord(word)
			continue
		}
		if n+wlen <= maxRunes {
			appendRunes(word)
			continue
		}
		flush()
		appendRunes(word)
	}

	if n > 0 {
		s := strings.TrimRight(b.String(), " \t")
		if strings.TrimSpace(s) != "" {
			chunks = append(chunks, s)
		}
	}
	return chunks
}

// sendTelegramTextChunks шлёт текст одним или несколькими sendMessage (до 4096 символов, разбиение по словам).
func (s *Server) sendTelegramTextChunks(ctx context.Context, chatID int64, text string) (firstMsgID int, err error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, nil
	}
	chunks := splitTelegramMessageChunks(text, maxTelegramMessageRunes)
	if len(chunks) == 0 {
		chunks = []string{text}
	}
	url := "https://api.telegram.org/bot" + s.TelegramBotToken + "/sendMessage"
	for i, chunk := range chunks {
		body, _ := json.Marshal(map[string]interface{}{
			"chat_id": chatID,
			"text":    chunk,
		})
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		resp, e := http.DefaultClient.Do(httpReq)
		if e != nil {
			return firstMsgID, fmt.Errorf("telegram send failed: %w", e)
		}
		bb, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return firstMsgID, fmt.Errorf("telegram read failed: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return firstMsgID, fmt.Errorf("telegram %d: %s", resp.StatusCode, strings.TrimSpace(string(bb)))
		}
		var telegramOut struct {
			OK     bool `json:"ok"`
			Result struct {
				MessageID int `json:"message_id"`
			} `json:"result"`
		}
		_ = json.Unmarshal(bb, &telegramOut)
		if i == 0 {
			firstMsgID = telegramOut.Result.MessageID
		}
	}
	return firstMsgID, nil
}
