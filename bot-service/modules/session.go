package modules

import (
	"context"

	"github.com/google/uuid"
)

// EnsureSession creates or updates session for (telegram_id, chat_id); saves username to core.users. Returns session_id.
func (b *Bot) EnsureSession(ctx context.Context, chatID, userID int64, username string) (uuid.UUID, error) {
	if username != "" {
		_, _ = b.Pool.Exec(ctx, `
			INSERT INTO core.users (telegram_id, username) VALUES ($1, $2)
			ON CONFLICT (telegram_id) DO UPDATE SET username = EXCLUDED.username
		`, userID, username)
	}
	var id uuid.UUID
	err := b.Pool.QueryRow(ctx, `
		INSERT INTO chat.sessions (telegram_id, chat_id, last_active)
		VALUES ($1, $2, NOW())
		ON CONFLICT (telegram_id, chat_id) DO UPDATE SET last_active = NOW()
		RETURNING id
	`, userID, chatID).Scan(&id)
	return id, err
}

// UpdateMessageTelegramID sets telegram_message_id for a stored message (after we send reply to TG).
func (b *Bot) UpdateMessageTelegramID(ctx context.Context, messageID uuid.UUID, telegramMessageID int) error {
	_, err := b.Pool.Exec(ctx, `UPDATE chat.messages SET telegram_message_id = $1 WHERE id = $2`, telegramMessageID, messageID)
	return err
}

// DeleteSession deletes session and all messages/attachments/answer_context for (userID, chatID).
func (b *Bot) DeleteSession(ctx context.Context, userID, chatID int64) error {
	_, err := b.Pool.Exec(ctx, `
		WITH sid AS (
			SELECT id FROM chat.sessions WHERE telegram_id = $1 AND chat_id = $2
		)
		DELETE FROM chat.answer_context WHERE session_id IN (SELECT id FROM sid);
		DELETE FROM chat.messages WHERE session_id IN (SELECT id FROM sid);
		DELETE FROM chat.attachments WHERE session_id IN (SELECT id FROM sid);
		DELETE FROM chat.sessions WHERE telegram_id = $1 AND chat_id = $2
	`, userID, chatID)
	return err
}
