package modules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type schedulerComposeReq struct {
	AngelName   string `json:"angel_name"`
	ContextText string `json:"context_text"`
	RequestID   string `json:"request_id"`
}

type schedulerDeliverReq struct {
	TelegramID  int64  `json:"telegram_id"`
	ChatID      int64  `json:"chat_id"`
	Text        string `json:"text"`
	AngelChunkID string `json:"angel_chunk_id,omitempty"`
	AngelName   string `json:"angel_name,omitempty"`
	RequestID   string `json:"request_id"`
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

	body, _ := json.Marshal(map[string]interface{}{
		"chat_id": req.ChatID,
		"text":    text,
	})
	url := "https://api.telegram.org/bot" + s.TelegramBotToken + "/sendMessage"
	httpReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		http.Error(w, `{"error":"telegram send failed"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	bb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf(`{"error":"telegram %d: %s"}`, resp.StatusCode, strings.TrimSpace(string(bb))), http.StatusBadGateway)
		return
	}
	var out struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	_ = json.Unmarshal(bb, &out)
	if out.Result.MessageID != 0 {
		// Best-effort: сохраняем напоминание в память чата, чтобы reply-вопрос шёл по его контексту.
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

