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
	if contextRef != nil && strings.TrimSpace(*contextRef) != "" {
		ref := strings.TrimSpace(*contextRef)
		if full, ok := s.ResolveFullContextFromRef(ctx, ref); ok && strings.TrimSpace(full) != "" {
			return full, true
		}
	}
	return strings.TrimSpace(contextText), true
}

// GetReplyToContext returns предыдущий вопрос пользователя, текст ответа бота, фактический CONTEXT,
// который использовался для того ответа (полный текст из Postgres по context_ref, если ответ строился на full;
// иначе ровно context_text — фрагмент чанка), и сохранённый context_ref для цепочки follow-up (пусто если был только чанк).
func (s *Server) GetReplyToContext(ctx context.Context, sessionID uuid.UUID, replyToTelegramMessageID int) (userQuestion, botAnswer, contextForLLM, storedContextRef string, ok bool) {
	var botMsgID uuid.UUID
	var botContent string
	err := s.Pool.QueryRow(ctx, `
		SELECT m.id, m.content FROM chat.messages m
		WHERE m.session_id = $1 AND m.telegram_message_id = $2 AND m.role = 'assistant'
		LIMIT 1
	`, sessionID, replyToTelegramMessageID).Scan(&botMsgID, &botContent)
	if err != nil {
		return "", "", "", "", false
	}

	var ctxText string
	var ctxRef *string
	_ = s.Pool.QueryRow(ctx, `
		SELECT ac.context_text, ac.context_ref FROM chat.answer_context ac
		WHERE ac.session_id = $1 AND ac.message_id = $2
	`, sessionID, botMsgID).Scan(&ctxText, &ctxRef)

	contextForLLM = ""
	storedContextRef = ""
	if ctxRef != nil && strings.TrimSpace(*ctxRef) != "" {
		storedContextRef = strings.TrimSpace(*ctxRef)
		// Сначала Postgres (document_context), иначе mcp-read — тот же полный документ, что и при первичном ответе.
		if full, okFull := s.ResolveFullContextFromRef(ctx, storedContextRef); okFull && strings.TrimSpace(full) != "" {
			contextForLLM = strings.TrimSpace(full)
		}
	}
	if contextForLLM == "" && strings.TrimSpace(ctxText) != "" {
		// Сохранён только фрагмент чанка (без ref) — в LLM уходит ровно он.
		contextForLLM = strings.TrimSpace(ctxText)
		storedContextRef = ""
	}
	if contextForLLM == "" {
		storedContextRef = ""
	}

	var prevUserContent string
	_ = s.Pool.QueryRow(ctx, `
		SELECT content FROM chat.messages
		WHERE session_id = $1 AND created_at < (SELECT created_at FROM chat.messages WHERE id = $2)
		ORDER BY created_at DESC LIMIT 1
	`, sessionID, botMsgID).Scan(&prevUserContent)

	return prevUserContent, botContent, contextForLLM, storedContextRef, true
}

// GetLastAssistantMessage returns the content of the most recent assistant message in the session (для fallback при ответе по списку name all).
func (s *Server) GetLastAssistantMessage(ctx context.Context, sessionID uuid.UUID) (content string, ok bool) {
	err := s.Pool.QueryRow(ctx, `
		SELECT content FROM chat.messages
		WHERE session_id = $1 AND role = 'assistant' AND content IS NOT NULL AND content != ''
		ORDER BY created_at DESC LIMIT 1
	`, sessionID).Scan(&content)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(content), true
}

// GetLastAssistantNumberedList returns the content of the most recent assistant message that parses as a numbered list (>= 3 items).
// Используется, когда последнее сообщение ассистента могло быть не списком (например, отладочное или «не найдено»).
func (s *Server) GetLastAssistantNumberedList(ctx context.Context, sessionID uuid.UUID, parseList func(string) []string) (content string, ok bool) {
	rows, err := s.Pool.Query(ctx, `
		SELECT content FROM chat.messages
		WHERE session_id = $1 AND role = 'assistant' AND content IS NOT NULL AND content != ''
		ORDER BY created_at DESC LIMIT 10
	`, sessionID)
	if err != nil {
		return "", false
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if rows.Scan(&c) != nil {
			continue
		}
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if len(parseList(c)) >= 3 {
			return c, true
		}
	}
	return "", false
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
