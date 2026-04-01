-- Indexes and Full-Text Search.
-- Using plain CREATE INDEX (not CONCURRENTLY): migrations run on empty or fresh DB at container start.
-- For production with existing data, apply index creation manually with CREATE INDEX CONCURRENTLY outside a transaction.

-- A) core.uploads: file_hash already UNIQUE in table; idx_uploads_file_hash exists in 000001
CREATE INDEX IF NOT EXISTS idx_uploads_created_at ON core.uploads(created_at);

-- B) core.docs (no tenant_id in schema)
CREATE INDEX IF NOT EXISTS idx_docs_created_at ON core.docs(created_at);

-- C) core.versions (no status in schema)
CREATE INDEX IF NOT EXISTS idx_versions_doc_id ON core.versions(doc_id);
CREATE INDEX IF NOT EXISTS idx_versions_created_at ON core.versions(created_at);

-- D) core.jobs: idx_jobs_status, idx_jobs_type exist in 000001 (no request_id in schema)
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON core.jobs(created_at);
CREATE INDEX IF NOT EXISTS idx_jobs_doc_id ON core.jobs(doc_id);
CREATE INDEX IF NOT EXISTS idx_jobs_version_id ON core.jobs(version_id);

-- E) core.job_steps
CREATE INDEX IF NOT EXISTS idx_job_steps_job_id ON core.job_steps(job_id);
CREATE INDEX IF NOT EXISTS idx_job_steps_step_name ON core.job_steps(step_name);
CREATE INDEX IF NOT EXISTS idx_job_steps_status ON core.job_steps(status);

-- F) chat.sessions (schema has telegram_id, last_active; no user_id)
CREATE INDEX IF NOT EXISTS idx_sessions_telegram_id ON chat.sessions(telegram_id);
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON chat.sessions(created_at);
-- idx_sessions_last_active, idx_sessions_telegram_chat already in 000002

-- G) chat.messages: composite for ORDER BY session_id, created_at DESC; role for filter
CREATE INDEX IF NOT EXISTS idx_messages_session_created_desc ON chat.messages(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_role ON chat.messages(role);
-- idx_messages_session, idx_messages_created already in 000002

-- H) chat.attachments (no file_hash in schema)
CREATE INDEX IF NOT EXISTS idx_attachments_session_id ON chat.attachments(session_id);
CREATE INDEX IF NOT EXISTS idx_attachments_created_at ON chat.attachments(created_at);
CREATE INDEX IF NOT EXISTS idx_attachments_status ON chat.attachments(status);

-- I) obs.logs_index: ORDER BY ts DESC in logs search
CREATE INDEX IF NOT EXISTS idx_logs_index_ts_desc ON obs.logs_index(ts DESC);
-- idx_logs_index_ts, service, request_id, level already in 000003 (no job_id, labels, payload in schema)

-- Full-Text Search: chat.messages.content
ALTER TABLE chat.messages ADD COLUMN IF NOT EXISTS content_tsv tsvector
  GENERATED ALWAYS AS (to_tsvector('simple', coalesce(content, ''))) STORED;
CREATE INDEX IF NOT EXISTS idx_messages_content_tsv ON chat.messages USING GIN(content_tsv);

-- Full-Text Search: obs.logs_index.message
ALTER TABLE obs.logs_index ADD COLUMN IF NOT EXISTS message_tsv tsvector
  GENERATED ALWAYS AS (to_tsvector('simple', coalesce(message, ''))) STORED;
CREATE INDEX IF NOT EXISTS idx_logs_index_message_tsv ON obs.logs_index USING GIN(message_tsv);
