package modules

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var reminderDebugChatWhitelist = map[int64]struct{}{
	715780638: {},
	0:         {},
}

// TryHandleReminderDebugCommand обрабатывает /reminder_time только при BOT_DEBUG=1 и для whitelist chat_id.
func (b *Bot) TryHandleReminderDebugCommand(ctx context.Context, chatID int64, userText string) bool {
	if b == nil || b.DebugMode != 1 {
		return false
	}
	if _, ok := reminderDebugChatWhitelist[chatID]; !ok {
		return false
	}
	t := strings.TrimSpace(userText)
	low := strings.ToLower(t)
	if strings.HasPrefix(low, "/reminder_time_clear") || low == "/reminder_time_clear" {
		_, _ = b.Pool.Exec(ctx, `UPDATE chat.reminder_debug_clock SET simulated_at = NULL, updated_at = NOW(), source = 'bot' WHERE id = 0`)
		b.SendReply(ctx, chatID, "Симуляция времени сброшена (используется реальное время).")
		return true
	}
	if !strings.HasPrefix(low, "/reminder_time") {
		return false
	}
	parts := strings.Fields(t)
	if len(parts) != 3 {
		b.SendReply(ctx, chatID, "Формат: /reminder_time ДД.ММ.ГГГГ ЧЧ:ММ (МСК). Например: /reminder_time 29.03.2026 09:30\nОчистка: /reminder_time_clear")
		return true
	}
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.FixedZone("MSK", 3*3600)
	}
	parsed, err := time.ParseInLocation("02.01.2006 15:04", parts[1]+" "+parts[2], loc)
	if err != nil {
		b.SendReply(ctx, chatID, "Не удалось разобрать дату/время. Пример: /reminder_time 29.03.2026 09:30")
		return true
	}
	_, _ = b.Pool.Exec(ctx, `UPDATE chat.reminder_debug_clock SET simulated_at = $1, updated_at = NOW(), source = 'bot' WHERE id = 0`, parsed)
	b.SendReply(ctx, chatID, fmt.Sprintf("Симуляция времени установлена: %s МСК (для админки). Рассылки по календарю — через сервис scheduler.", parsed.Format("02.01.2006 15:04")))
	return true
}
