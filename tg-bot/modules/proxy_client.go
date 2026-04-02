package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const proxyRequestTimeout = 120 * time.Second

// ChatResponse from mcp-proxy POST /chat.
type ChatResponse struct {
	ReplyText           string `json:"reply_text"`
	DebugMessage        string `json:"debug_message,omitempty"`
	MessageID           string `json:"message_id,omitempty"`
	ReminderExtraText   string `json:"reminder_extra_text,omitempty"`
	AngelChunkID        string `json:"angel_chunk_id,omitempty"`
}

// CallChat sends message to mcp-proxy and returns reply_text, debug_message, message_id, reminder_extra_text, angel_chunk_id.
func (b *Bot) CallChat(ctx context.Context, sessionID uuid.UUID, chatID, userID int64, username, messageText string, replyToTelegramMessageID int, requestID string) (replyText, debugMessage, messageID, reminderExtra, angelChunkID string, err error) {
	if requestID == "" {
		requestID = uuid.New().String()
	}
	body := map[string]interface{}{
		"session_id":                  sessionID.String(),
		"chat_id":                     chatID,
		"user_id":                     userID,
		"username":                    username,
		"message_text":                messageText,
		"reply_to_telegram_message_id": replyToTelegramMessageID,
		"request_id":                  requestID,
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.ProxyURL+"/chat", bytes.NewReader(payload))
	if err != nil {
		return "", "", "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: proxyRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", "", "", err
	}
	defer resp.Body.Close()
	bb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", "", fmt.Errorf("proxy %d: %s", resp.StatusCode, string(bb))
	}
	var out ChatResponse
	if err := json.Unmarshal(bb, &out); err != nil {
		return "", "", "", "", "", err
	}
	return out.ReplyText, out.DebugMessage, out.MessageID, out.ReminderExtraText, out.AngelChunkID, nil
}

// ProxySchedulerDeliver sends photo+caption or text via mcp-proxy (тот же путь, что и у scheduler).
func (b *Bot) ProxySchedulerDeliver(ctx context.Context, chatID, telegramUserID int64, text, angelChunkID string) (telegramMessageID int, err error) {
	body := map[string]interface{}{
		"chat_id":           chatID,
		"telegram_id":       telegramUserID,
		"text":              text,
		"angel_chunk_id":    angelChunkID,
		"angel_name":        "",
		"request_id":        "tg-chat",
		"skip_chat_memory":  true, // ответ уже записан в HandleChat; иначе дубль в Chat Log
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.ProxyURL+"/scheduler/deliver", bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(b.SchedulerSecret) != "" {
		req.Header.Set("X-Scheduler-Secret", b.SchedulerSecret)
	}
	client := &http.Client{Timeout: proxyRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	bb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("deliver %d: %s", resp.StatusCode, string(bb))
	}
	var out struct {
		OK         bool `json:"ok"`
		MessageID  int  `json:"message_id"`
	}
	if err := json.Unmarshal(bb, &out); err != nil {
		return 0, err
	}
	if !out.OK || out.MessageID == 0 {
		return 0, fmt.Errorf("deliver: no message_id in response")
	}
	return out.MessageID, nil
}
