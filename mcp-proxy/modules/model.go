package modules

import "github.com/google/uuid"

// ChatRequest is the body of POST /chat from the Telegram bot.
type ChatRequest struct {
	SessionID                 uuid.UUID `json:"session_id"`
	ChatID                    int64     `json:"chat_id"`
	UserID                    int64     `json:"user_id"`
	Username                  string    `json:"username"`
	MessageText               string    `json:"message_text"`
	ReplyToTelegramMessageID  int       `json:"reply_to_telegram_message_id"`
	RequestID                 string    `json:"request_id"`
}

// ChatResponse is returned by POST /chat.
type ChatResponse struct {
	ReplyText    string `json:"reply_text"`
	DebugMessage string `json:"debug_message,omitempty"`
	MessageID    string `json:"message_id,omitempty"` // UUID of assistant message for bot to set telegram_message_id
}
