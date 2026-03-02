-- telegram_message_id: ID сообщения в Telegram (для определения ответа пользователя на наше сообщение)
ALTER TABLE chat.messages ADD COLUMN IF NOT EXISTS telegram_message_id BIGINT;

-- Контекст (чанки), на основе которого был сформирован ответ; при ответе пользователя на это сообщение используем его без запроса в Qdrant
CREATE TABLE IF NOT EXISTS chat.answer_context (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id   UUID NOT NULL REFERENCES chat.sessions(id) ON DELETE CASCADE,
    message_id   UUID NOT NULL REFERENCES chat.messages(id) ON DELETE CASCADE,
    context_text TEXT NOT NULL,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_answer_context_message ON chat.answer_context(message_id);
CREATE INDEX IF NOT EXISTS idx_messages_telegram_id ON chat.messages(session_id, telegram_message_id);
