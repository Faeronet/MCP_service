-- Bind scheduler rows to note entries and user identity for sync/delete.
ALTER TABLE chat.scheduler_notifications
    ADD COLUMN IF NOT EXISTS telegram_username TEXT,
    ADD COLUMN IF NOT EXISTS note_item_id TEXT,
    ADD COLUMN IF NOT EXISTS notify_daily BOOLEAN NOT NULL DEFAULT FALSE;

-- One scheduler row per note item per telegram user.
CREATE UNIQUE INDEX IF NOT EXISTS idx_scheduler_notifications_telegram_note
    ON chat.scheduler_notifications (telegram_id, note_item_id)
    WHERE note_item_id IS NOT NULL AND note_item_id <> '';

CREATE INDEX IF NOT EXISTS idx_scheduler_notifications_note_item
    ON chat.scheduler_notifications (note_item_id)
    WHERE note_item_id IS NOT NULL AND note_item_id <> '';
