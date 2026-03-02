DROP TABLE IF EXISTS chat.answer_context;
DROP INDEX IF EXISTS idx_messages_telegram_id;
ALTER TABLE chat.messages DROP COLUMN IF EXISTS telegram_message_id;
