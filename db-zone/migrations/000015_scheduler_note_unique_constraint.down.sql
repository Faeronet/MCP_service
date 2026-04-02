ALTER TABLE chat.scheduler_notifications
    DROP CONSTRAINT IF EXISTS uq_scheduler_notifications_telegram_note_item;

CREATE UNIQUE INDEX IF NOT EXISTS idx_scheduler_notifications_telegram_note
    ON chat.scheduler_notifications (telegram_id, note_item_id)
    WHERE note_item_id IS NOT NULL AND note_item_id <> '';
