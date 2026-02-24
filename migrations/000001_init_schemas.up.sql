-- Core schema: users, docs, versions, jobs, uploads
CREATE SCHEMA IF NOT EXISTS core;

CREATE TABLE IF NOT EXISTS core.users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id BIGINT UNIQUE,
    username   TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS core.docs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS core.versions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    doc_id     UUID NOT NULL REFERENCES core.docs(id) ON DELETE CASCADE,
    version    INT NOT NULL DEFAULT 1,
    file_path  TEXT NOT NULL,
    file_hash  TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(doc_id, version)
);

CREATE TABLE IF NOT EXISTS core.uploads (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_hash  TEXT NOT NULL UNIQUE,
    file_path  TEXT NOT NULL,
    size_bytes BIGINT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS core.jobs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type        TEXT NOT NULL,
    payload     JSONB,
    status      TEXT NOT NULL DEFAULT 'pending',
    doc_id      UUID REFERENCES core.docs(id),
    version_id  UUID REFERENCES core.versions(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS core.job_steps (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id    UUID NOT NULL REFERENCES core.jobs(id) ON DELETE CASCADE,
    step_name TEXT NOT NULL,
    status    TEXT NOT NULL DEFAULT 'pending',
    detail    JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON core.jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON core.jobs(type);
CREATE INDEX IF NOT EXISTS idx_uploads_file_hash ON core.uploads(file_hash);
