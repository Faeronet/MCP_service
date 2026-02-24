-- Drop indexes and FTS columns added in 000004

DROP INDEX IF EXISTS chat.idx_messages_content_tsv;
ALTER TABLE chat.messages DROP COLUMN IF EXISTS content_tsv;

DROP INDEX IF EXISTS obs.idx_logs_index_message_tsv;
ALTER TABLE obs.logs_index DROP COLUMN IF EXISTS message_tsv;

DROP INDEX IF EXISTS obs.idx_logs_index_ts_desc;
DROP INDEX IF EXISTS chat.idx_attachments_status;
DROP INDEX IF EXISTS chat.idx_attachments_created_at;
DROP INDEX IF EXISTS chat.idx_attachments_session_id;
DROP INDEX IF EXISTS chat.idx_messages_role;
DROP INDEX IF EXISTS chat.idx_messages_session_created_desc;
DROP INDEX IF EXISTS chat.idx_sessions_created_at;
DROP INDEX IF EXISTS chat.idx_sessions_telegram_id;
DROP INDEX IF EXISTS core.idx_job_steps_status;
DROP INDEX IF EXISTS core.idx_job_steps_step_name;
DROP INDEX IF EXISTS core.idx_job_steps_job_id;
DROP INDEX IF EXISTS core.idx_jobs_version_id;
DROP INDEX IF EXISTS core.idx_jobs_doc_id;
DROP INDEX IF EXISTS core.idx_jobs_created_at;
DROP INDEX IF EXISTS core.idx_versions_created_at;
DROP INDEX IF EXISTS core.idx_versions_doc_id;
DROP INDEX IF EXISTS core.idx_docs_created_at;
DROP INDEX IF EXISTS core.idx_uploads_created_at;
