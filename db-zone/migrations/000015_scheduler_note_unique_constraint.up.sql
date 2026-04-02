-- Ensure ON CONFLICT (telegram_id, note_item_id) is valid for upsert.
DELETE FROM chat.scheduler_notifications a
USING chat.scheduler_notifications b
WHERE a.ctid < b.ctid
  AND a.telegram_id = b.telegram_id
  AND coalesce(a.note_item_id, '') <> ''
  AND a.note_item_id = b.note_item_id;

DROP INDEX IF EXISTS chat.idx_scheduler_notifications_telegram_note;

ALTER TABLE chat.scheduler_notifications
    ADD CONSTRAINT uq_scheduler_notifications_telegram_note_item
    UNIQUE (telegram_id, note_item_id);
