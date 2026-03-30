package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logReminders = logging.New("tg-bot.reminders")

const reminderTickInterval = 20 * time.Second
const reminderTickHTTPTimeout = 60 * time.Second

type reminderTickResponse struct {
	Notifications []struct {
		TelegramID int64  `json:"telegram_id"`
		ChatID     int64  `json:"chat_id"`
		Text       string `json:"text"`
	} `json:"notifications"`
}

// TickRemindersDispatch — заглушка: напоминания по календарю перенесены в сервис scheduler.
func (b *Bot) TickRemindersDispatch(ctx context.Context) {
	_ = ctx
	_ = b
}

func truncateForLog(s string) string {
	if len(s) <= 200 {
		return s
	}
	return s[:200] + "..."
}

// StartReminderWorker периодически опрашивает mcp-proxy на предмет напоминаний (пока жив контекст ctx).
func (b *Bot) StartReminderWorker(ctx context.Context) {
	// Первый тик сразу после старта, чтобы не ждать интервал.
	b.TickRemindersDispatch(ctx)
	t := time.NewTicker(reminderTickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c, cancel := context.WithTimeout(context.Background(), reminderTickHTTPTimeout+15*time.Second)
			b.TickRemindersDispatch(c)
			cancel()
		}
	}
}
