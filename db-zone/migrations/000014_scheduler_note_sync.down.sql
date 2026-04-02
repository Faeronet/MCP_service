DROP INDEX IF EXISTS chat.idx_scheduler_notifications_note_item;
DROP INDEX IF EXISTS chat.idx_scheduler_notifications_telegram_note;

ALTER TABLE chat.scheduler_notifications
    DROP COLUMN IF EXISTS notify_daily,
    DROP COLUMN IF EXISTS note_item_id,
    DROP COLUMN IF EXISTS telegram_username;
