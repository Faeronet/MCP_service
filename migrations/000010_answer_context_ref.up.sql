-- Ссылка на полный контекст (chunk_id или collection:chunk_id) вместо дублирования текста в answer_context.
ALTER TABLE chat.answer_context ADD COLUMN IF NOT EXISTS context_ref TEXT;
CREATE INDEX IF NOT EXISTS idx_answer_context_context_ref ON chat.answer_context(context_ref) WHERE context_ref IS NOT NULL;
