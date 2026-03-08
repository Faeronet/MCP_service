package modules

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

const maxMessagesBeforeTrim = 30
const keepMessagesAfterTrim = 20

var logMemory = logging.New("mcp-proxy.memory")

// AppendMessageWithReply saves a message; for role=user, replyToTelegramID is the TG message we're replying to (0 = none).
func (s *Server) AppendMessageWithReply(ctx context.Context, sessionID uuid.UUID, role, content string, replyToTelegramID int) (uuid.UUID, error) {
	var id uuid.UUID
	if replyToTelegramID != 0 && role == "user" {
		err := s.Pool.QueryRow(ctx, `INSERT INTO chat.messages (session_id, role, content, reply_to_telegram_message_id) VALUES ($1, $2, $3, $4) RETURNING id`,
			sessionID, role, content, replyToTelegramID).Scan(&id)
		return id, err
	}
	err := s.Pool.QueryRow(ctx, `INSERT INTO chat.messages (session_id, role, content) VALUES ($1, $2, $3) RETURNING id`, sessionID, role, content).Scan(&id)
	return id, err
}

// UpdateMessageTelegramID sets telegram_message_id for a stored message.
func (s *Server) UpdateMessageTelegramID(ctx context.Context, messageID uuid.UUID, telegramMessageID int) error {
	_, err := s.Pool.Exec(ctx, `UPDATE chat.messages SET telegram_message_id = $1 WHERE id = $2`, telegramMessageID, messageID)
	return err
}

// SaveAnswerContext stores context_text and context_ref for an assistant message.
func (s *Server) SaveAnswerContext(ctx context.Context, sessionID, messageID uuid.UUID, contextText, contextRef string) error {
	if contextRef != "" {
		contextText = ""
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO chat.answer_context (session_id, message_id, context_text, context_ref) VALUES ($1, $2, $3, $4)`, sessionID, messageID, contextText, toNullString(contextRef))
	return err
}

// GetContextByTelegramMessageID returns context_text (or full context by ref) for a bot message.
func (s *Server) GetContextByTelegramMessageID(ctx context.Context, sessionID uuid.UUID, telegramMessageID int) (string, bool) {
	var contextText string
	var contextRef *string
	err := s.Pool.QueryRow(ctx, `
		SELECT ac.context_text, ac.context_ref FROM chat.answer_context ac
		JOIN chat.messages m ON m.id = ac.message_id
		WHERE m.session_id = $1 AND m.telegram_message_id = $2
		LIMIT 1
	`, sessionID, telegramMessageID).Scan(&contextText, &contextRef)
	if err != nil {
		return "", false
	}
	if contextRef != nil && *contextRef != "" {
		if full, ok := s.GetFullContextByRef(ctx, *contextRef); ok && full != "" {
			return full, true
		}
	}
	return contextText, true
}

// GetReplyToContext returns (userQuestion, botAnswer, contextText) for the message we're replying to.
func (s *Server) GetReplyToContext(ctx context.Context, sessionID uuid.UUID, replyToTelegramMessageID int) (userQuestion, botAnswer, contextText string, ok bool) {
	var botMsgID uuid.UUID
	var botContent string
	err := s.Pool.QueryRow(ctx, `
		SELECT m.id, m.content FROM chat.messages m
		WHERE m.session_id = $1 AND m.telegram_message_id = $2 AND m.role = 'assistant'
		LIMIT 1
	`, sessionID, replyToTelegramMessageID).Scan(&botMsgID, &botContent)
	if err != nil {
		return "", "", "", false
	}
	contextText, _ = s.GetContextByTelegramMessageID(ctx, sessionID, replyToTelegramMessageID)
	_ = s.Pool.QueryRow(ctx, `
		SELECT content FROM chat.messages
		WHERE session_id = $1 AND created_at < (SELECT created_at FROM chat.messages WHERE id = $2)
		ORDER BY created_at DESC LIMIT 1
	`, sessionID, botMsgID).Scan(&userQuestion)
	return userQuestion, botContent, contextText, true
}

// GetAttachmentsText returns concatenated extracted_text from session attachments.
func (s *Server) GetAttachmentsText(ctx context.Context, sessionID uuid.UUID) string {
	rows, err := s.Pool.Query(ctx,
		`SELECT extracted_text FROM chat.attachments WHERE session_id = $1 AND status = 'done' AND extracted_text IS NOT NULL AND extracted_text != '' ORDER BY created_at DESC LIMIT 10`,
		sessionID)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil && t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

// TrimSessionMessagesIfNeeded deletes oldest messages beyond keepMessagesAfterTrim.
func (s *Server) TrimSessionMessagesIfNeeded(ctx context.Context, sessionID uuid.UUID) {
	var count int64
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM chat.messages WHERE session_id = $1`, sessionID).Scan(&count); err != nil || count < maxMessagesBeforeTrim {
		return
	}
	res, err := s.Pool.Exec(ctx, `
		DELETE FROM chat.messages WHERE session_id = $1 AND id NOT IN (
			SELECT id FROM chat.messages WHERE session_id = $1 ORDER BY created_at DESC LIMIT $2
		)
	`, sessionID, keepMessagesAfterTrim)
	if err != nil {
		logMemory.Warn(ctx, "trim session messages", logging.KV{"error", err}, logging.KV{"session_id", sessionID})
		return
	}
	if res.RowsAffected() > 0 {
		logMemory.Info(ctx, "trimmed session messages", logging.KV{"session_id", sessionID}, logging.KV{"deleted", res.RowsAffected()})
	}
}
