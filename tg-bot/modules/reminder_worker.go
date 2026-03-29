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

const reminderTickInterval = 45 * time.Second
const reminderTickHTTPTimeout = 60 * time.Second

type reminderTickResponse struct {
	Notifications []struct {
		TelegramID int64  `json:"telegram_id"`
		ChatID     int64  `json:"chat_id"`
		Text       string `json:"text"`
	} `json:"notifications"`
}

// TickRemindersDispatch вызывает mcp-proxy POST /reminders/tick и рассылает уведомления в Telegram.
func (b *Bot) TickRemindersDispatch(ctx context.Context) {
	if b == nil || strings.TrimSpace(b.ProxyURL) == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.ProxyURL+"/reminders/tick", bytes.NewReader([]byte("{}")))
	if err != nil {
		logReminders.Warn(ctx, "reminders tick request", logging.KV{"error", err})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: reminderTickHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		logReminders.Warn(ctx, "reminders tick http", logging.KV{"error", err})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		logReminders.Warn(ctx, "reminders tick status", logging.KV{"code", resp.StatusCode}, logging.KV{"body", truncateForLog(string(body))})
		return
	}
	var out reminderTickResponse
	if err := json.Unmarshal(body, &out); err != nil {
		logReminders.Warn(ctx, "reminders tick json", logging.KV{"error", err})
		return
	}
	for _, n := range out.Notifications {
		t := strings.TrimSpace(n.Text)
		if t == "" || b.Bot == nil {
			continue
		}
		b.SendLongReply(ctx, n.ChatID, 0, t)
	}
}

func truncateForLog(s string) string {
	if len(s) <= 200 {
		return s
	}
	return s[:200] + "..."
}

// StartReminderWorker периодически опрашивает mcp-proxy на предмет напоминаний (пока жив контекст ctx).
func (b *Bot) StartReminderWorker(ctx context.Context) {
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
