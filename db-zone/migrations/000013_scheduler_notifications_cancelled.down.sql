UPDATE chat.scheduler_notifications SET status = 'failed' WHERE status = 'cancelled';
ALTER TABLE chat.scheduler_notifications DROP CONSTRAINT IF EXISTS chk_scheduler_notifications_status;
ALTER TABLE chat.scheduler_notifications ADD CONSTRAINT chk_scheduler_notifications_status
    CHECK (status IN ('pending', 'sending', 'sent', 'failed'));
