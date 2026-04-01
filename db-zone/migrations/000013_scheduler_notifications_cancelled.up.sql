-- Разрешаем статус cancelled (отмена из админки до отправки).
ALTER TABLE chat.scheduler_notifications DROP CONSTRAINT IF EXISTS chk_scheduler_notifications_status;
ALTER TABLE chat.scheduler_notifications ADD CONSTRAINT chk_scheduler_notifications_status
    CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'cancelled'));
