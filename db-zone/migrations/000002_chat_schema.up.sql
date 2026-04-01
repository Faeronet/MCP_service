-- Chat schema: sessions, messages, attachments, TTL cleanup
CREATE SCHEMA IF NOT EXISTS chat;

CREATE TABLE IF NOT EXISTS chat.sessions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id  BIGINT NOT NULL,
    chat_id      BIGINT NOT NULL,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    last_active  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(telegram_id, chat_id)
);

CREATE TABLE IF NOT EXISTS chat.messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat.sessions(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat.attachments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id   UUID NOT NULL REFERENCES chat.sessions(id) ON DELETE CASCADE,
    message_id   UUID REFERENCES chat.messages(id),
    job_id       UUID,
    object_key   TEXT NOT NULL,
    mime_type    TEXT,
    extracted_text TEXT,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_telegram_chat ON chat.sessions(telegram_id, chat_id);
CREATE INDEX IF NOT EXISTS idx_messages_session ON chat.messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_created ON chat.messages(created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_last_active ON chat.sessions(last_active);
