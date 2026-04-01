-- Scheduled notifications from angels-web via scheduler service (independent of legacy reminder_subscribers tick).
CREATE TABLE IF NOT EXISTS chat.scheduler_notifications (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id      BIGINT NOT NULL,
    chat_id          BIGINT NOT NULL,
    angel_chunk_id   TEXT NOT NULL,
    angel_name       TEXT NOT NULL,
    message_text     TEXT NOT NULL,
    send_at          TIMESTAMPTZ NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at          TIMESTAMPTZ,
    last_error       TEXT,
    CONSTRAINT chk_scheduler_notifications_status CHECK (status IN ('pending', 'sending', 'sent', 'failed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scheduler_notifications_dedup
    ON chat.scheduler_notifications (telegram_id, angel_chunk_id, send_at);

CREATE INDEX IF NOT EXISTS idx_scheduler_notifications_pending_send
    ON chat.scheduler_notifications (send_at)
    WHERE status = 'pending';
