-- Физические даты ангелов (ключ «Физическое» при инжесте), dd.mm
CREATE TABLE IF NOT EXISTS core.angel_physical_dates (
    chunk_id   TEXT PRIMARY KEY,
    doc_id     TEXT NOT NULL,
    name       TEXT NOT NULL,
    dates_ddmm TEXT[] NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_angel_physical_doc_id ON core.angel_physical_dates(doc_id);

CREATE TABLE IF NOT EXISTS core.angel_physical_date_entries (
    ddmm     TEXT NOT NULL,
    chunk_id TEXT NOT NULL REFERENCES core.angel_physical_dates(chunk_id) ON DELETE CASCADE,
    PRIMARY KEY (ddmm, chunk_id)
);
CREATE INDEX IF NOT EXISTS idx_angel_physical_entries_ddmm ON core.angel_physical_date_entries(ddmm);

CREATE TABLE IF NOT EXISTS chat.reminder_subscribers (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id  BIGINT NOT NULL,
    chat_id      BIGINT NOT NULL,
    reminder_hh  SMALLINT NOT NULL CHECK (reminder_hh >= 0 AND reminder_hh <= 23),
    reminder_mm  SMALLINT NOT NULL CHECK (reminder_mm >= 0 AND reminder_mm <= 59),
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (telegram_id, chat_id)
);
CREATE INDEX IF NOT EXISTS idx_reminder_subscribers_enabled ON chat.reminder_subscribers(enabled) WHERE enabled;

CREATE TABLE IF NOT EXISTS chat.reminder_jobs (
    delivery_date_msk DATE PRIMARY KEY,
    angel_chunk_id    TEXT,
    message_text      TEXT,
    skipped_duplicate BOOLEAN NOT NULL DEFAULT FALSE,
    prepared_at       TIMESTAMPTZ,
    prod_complete     BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS chat.reminder_sent (
    telegram_id       BIGINT NOT NULL,
    chat_id           BIGINT NOT NULL,
    delivery_date_msk DATE NOT NULL,
    sent_at           TIMESTAMPTZ DEFAULT NOW(),
    debug_mode        BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (telegram_id, chat_id, delivery_date_msk, debug_mode)
);

CREATE TABLE IF NOT EXISTS chat.reminder_debug_clock (
    id              SMALLINT PRIMARY KEY DEFAULT 0,
    simulated_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    source          TEXT
);

INSERT INTO chat.reminder_debug_clock (id, simulated_at, source)
VALUES (0, NULL, 'init')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS chat.reminder_debug_clock_chat (
    chat_id      BIGINT PRIMARY KEY,
    simulated_at TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat.reminder_global_config (
    id         SMALLINT PRIMARY KEY DEFAULT 0,
    disabled   BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO chat.reminder_global_config (id, disabled)
VALUES (0, FALSE)
ON CONFLICT (id) DO NOTHING;
