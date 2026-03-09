package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const proxyRequestTimeout = 120 * time.Second

// ChatResponse from mcp-proxy POST /chat.
type ChatResponse struct {
	ReplyText    string `json:"reply_text"`
	DebugMessage string `json:"debug_message,omitempty"`
	MessageID    string `json:"message_id,omitempty"`
}

// CallChat sends message to mcp-proxy and returns reply_text, debug_message, message_id.
func (b *Bot) CallChat(ctx context.Context, sessionID uuid.UUID, chatID, userID int64, username, messageText string, replyToTelegramMessageID int, requestID string) (replyText, debugMessage, messageID string, err error) {
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
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: proxyRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	bb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("proxy %d: %s", resp.StatusCode, string(bb))
	}
	var out ChatResponse
	if err := json.Unmarshal(bb, &out); err != nil {
		return "", "", "", err
	}
	return out.ReplyText, out.DebugMessage, out.MessageID, nil
}
