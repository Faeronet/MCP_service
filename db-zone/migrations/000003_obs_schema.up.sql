-- Observability schema: logs_index for Admin UI search
CREATE SCHEMA IF NOT EXISTS obs;

CREATE TABLE IF NOT EXISTS obs.logs_index (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ts          TIMESTAMPTZ NOT NULL,
    level       TEXT,
    service     TEXT,
    request_id  TEXT,
    message     TEXT,
    log_id      TEXT,
    raw_ref     TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_logs_index_ts ON obs.logs_index(ts);
CREATE INDEX IF NOT EXISTS idx_logs_index_service ON obs.logs_index(service);
CREATE INDEX IF NOT EXISTS idx_logs_index_request_id ON obs.logs_index(request_id);
CREATE INDEX IF NOT EXISTS idx_logs_index_level ON obs.logs_index(level);
