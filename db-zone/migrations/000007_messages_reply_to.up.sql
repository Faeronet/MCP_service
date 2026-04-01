-- Связь «ответ по контексту»: сообщение пользователя может быть ответом на сообщение бота (reply in Telegram)
ALTER TABLE chat.messages ADD COLUMN IF NOT EXISTS reply_to_telegram_message_id BIGINT;
