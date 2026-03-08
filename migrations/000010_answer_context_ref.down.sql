DROP INDEX IF EXISTS chat.idx_answer_context_context_ref;
ALTER TABLE chat.answer_context DROP COLUMN IF EXISTS context_ref;
