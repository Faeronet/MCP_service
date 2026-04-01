-- Request journal and compose cache for scheduler ingest.
CREATE TABLE IF NOT EXISTS chat.scheduler_note_requests (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_username TEXT NOT NULL,
    telegram_id       BIGINT,
    payload_json      JSONB NOT NULL,
    accepted          BOOLEAN NOT NULL DEFAULT FALSE,
    errors_json       JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scheduler_note_requests_created_at
    ON chat.scheduler_note_requests (created_at DESC);

CREATE TABLE IF NOT EXISTS chat.scheduler_compose_results (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id    BIGINT NOT NULL,
    angel_chunk_id TEXT NOT NULL,
    angel_name     TEXT NOT NULL,
    reminder_text  TEXT NOT NULL,
    request_id     TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scheduler_compose_results_lookup
    ON chat.scheduler_compose_results (telegram_id, angel_chunk_id, created_at DESC);
