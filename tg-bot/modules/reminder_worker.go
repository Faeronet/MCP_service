package modules

import (
	"context"
	"time"
)

const reminderTickInterval = 20 * time.Second
const reminderTickHTTPTimeout = 60 * time.Second

// TickRemindersDispatch — заглушка: напоминания по календарю перенесены в сервис scheduler.
func (b *Bot) TickRemindersDispatch(ctx context.Context) {
	_ = ctx
	_ = b
}

// StartReminderWorker периодически дергает TickRemindersDispatch (заглушка, пока жив контекст ctx).
func (b *Bot) StartReminderWorker(ctx context.Context) {
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
