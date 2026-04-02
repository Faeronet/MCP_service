package modules

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var workerLog = logging.New("scheduler-worker")

// RunDispatcher periodically plans pending notifications and sets per-item timers.
// It selects notifications where send_at <= now() + 10 minutes and schedules a timer for each item.
// To avoid missing items created shortly after the previous scan, we plan at least as often
// as SCHEDULER_DISPATCH_INTERVAL.
func RunDispatcher(ctx context.Context, pool *pgxpool.Pool, cfg Config, bot *BotClient) {
	planEvery := cfg.PollInterval
	if cfg.DispatchInterval > 0 && cfg.DispatchInterval < planEvery {
		planEvery = cfg.DispatchInterval
	}
	poll := time.NewTicker(planEvery)
	defer poll.Stop()
	scheduled := map[string]struct{}{}
	var mu sync.Mutex

	plan := func() {
		// Одноразовые уведомления: через час после отправки удаляем из БД,
		// чтобы они исчезали и из админ-списков.
		_, _ = pool.Exec(ctx, `
			DELETE FROM chat.scheduler_notifications
			WHERE status = 'sent'
			  AND sent_at IS NOT NULL
			  AND sent_at <= (now() - $1::interval)
		`, "1 hour")

		rows, err := pool.Query(ctx, `
			SELECT id, telegram_id, chat_id, message_text, send_at
			FROM chat.scheduler_notifications
			WHERE status = 'pending'
			  AND send_at <= (now() + interval '10 minutes')
			ORDER BY send_at ASC
			LIMIT 2048
		`)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var tg, chat int64
			var text string
			var sendAt time.Time
			if rows.Scan(&id, &tg, &chat, &text, &sendAt) != nil {
				continue
			}
			mu.Lock()
			if _, ok := scheduled[id]; ok {
				mu.Unlock()
				continue
			}
			scheduled[id] = struct{}{}
			mu.Unlock()

			delay := time.Until(sendAt)
			if delay < 0 {
				delay = 0
			}
			go func(id string, telegramID, chatID int64, text string, delay time.Duration) {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					mu.Lock()
					delete(scheduled, id)
					mu.Unlock()
					return
				case <-timer.C:
					dispatchOne(ctx, pool, bot, id, telegramID, chatID, text)
					mu.Lock()
					delete(scheduled, id)
					mu.Unlock()
				}
			}(id, tg, chat, text, delay)
		}
	}

	plan()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			plan()
		}
	}
}

func dispatchOne(ctx context.Context, pool *pgxpool.Pool, bot *BotClient, id string, telegramID, chatID int64, text string) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM chat.scheduler_notifications
		WHERE id = $1::uuid
		FOR UPDATE SKIP LOCKED
	`, id).Scan(&currentStatus)
	if err != nil || currentStatus != "pending" {
		return
	}
	_, _ = tx.Exec(ctx, `UPDATE chat.scheduler_notifications SET status = 'sending' WHERE id = $1::uuid`, id)
	if tx.Commit(ctx) != nil {
		return
	}

	err = bot.Deliver(context.Background(), telegramID, chatID, text)
	if err != nil {
		// err может не сериализоваться корректно в structured logging, поэтому логируем строку.
		workerLog.Warn(ctx, "deliver failed", logging.KV{"id", id}, logging.KV{"error", err.Error()})
		_, _ = pool.Exec(ctx, `
			UPDATE chat.scheduler_notifications
			SET status = 'failed', last_error = $2
			WHERE id = $1::uuid
		`, id, err.Error())
		return
	}
	var daily bool
	_ = pool.QueryRow(ctx, `SELECT coalesce(notify_daily, false) FROM chat.scheduler_notifications WHERE id = $1::uuid`, id).Scan(&daily)
	if daily {
		_, _ = pool.Exec(ctx, `
			UPDATE chat.scheduler_notifications
			SET status = 'pending',
			    send_at = (send_at + interval '1 day'),
			    sent_at = now(),
			    last_error = NULL
			WHERE id = $1::uuid
		`, id)
		return
	}
	_, _ = pool.Exec(ctx, `
		UPDATE chat.scheduler_notifications
		SET status = 'sent', sent_at = now(), last_error = NULL
		WHERE id = $1::uuid
	`, id)
}
