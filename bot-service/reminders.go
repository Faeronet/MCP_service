package main

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var mskLoc *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		mskLoc = time.FixedZone("MSK", 3*3600)
	} else {
		mskLoc = loc
	}
}

var reReminderLine = regexp.MustCompile(`(?i)^\[reminder\]\s*([01]?\d|2[0-3])[:.]([0-5]\d)\s*$`)

var reminderDebugChatWhitelist = map[int64]struct{}{
	715780638: {},
	0:         {},
}

func formatDDMMMSK(t time.Time) string {
	x := t.In(mskLoc)
	return fmt.Sprintf("%02d.%02d", x.Day(), int(x.Month()))
}

func dateInMSK(t time.Time) time.Time {
	x := t.In(mskLoc)
	return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, mskLoc)
}

func (b *Bot) effectiveNow(ctx context.Context) time.Time {
	if b.debugMode != 1 {
		return time.Now().In(mskLoc)
	}
	var sim sql.NullTime
	err := b.pool.QueryRow(ctx, `SELECT simulated_at FROM chat.reminder_debug_clock WHERE id = 0`).Scan(&sim)
	if err == nil && sim.Valid {
		return sim.Time.In(mskLoc)
	}
	return time.Now().In(mskLoc)
}

func (b *Bot) realNowMSK() time.Time {
	return time.Now().In(mskLoc)
}

func (b *Bot) remindersGloballyDisabled(ctx context.Context) bool {
	var d bool
	err := b.pool.QueryRow(ctx, `SELECT COALESCE(disabled, false) FROM chat.reminder_global_config WHERE id = 0`).Scan(&d)
	return err == nil && d
}

// parseReminderLine: "[reminder] HH.MM" → час, минута, ok
func parseReminderLine(s string) (hh, mm int, ok bool) {
	s = strings.TrimSpace(stripThink(s))
	m := reReminderLine.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	hh, _ = strconv.Atoi(m[1])
	mm, _ = strconv.Atoi(m[2])
	return hh, mm, true
}

func (b *Bot) upsertReminderSubscriber(ctx context.Context, telegramID, chatID int64, hh, mm int) error {
	_, err := b.pool.Exec(ctx, `
		INSERT INTO chat.reminder_subscribers (telegram_id, chat_id, reminder_hh, reminder_mm, enabled, updated_at)
		VALUES ($1, $2, $3, $4, true, NOW())
		ON CONFLICT (telegram_id, chat_id) DO UPDATE SET
			reminder_hh = EXCLUDED.reminder_hh,
			reminder_mm = EXCLUDED.reminder_mm,
			enabled = true,
			updated_at = NOW()
	`, telegramID, chatID, hh, mm)
	return err
}

func (b *Bot) angelForDeliveryDate(ctx context.Context, deliveryDate time.Time) (chunkID, name string, ok bool) {
	ddmm := formatDDMMMSK(deliveryDate)
	err := b.pool.QueryRow(ctx, `
		SELECT a.chunk_id, a.name
		FROM core.angel_physical_date_entries e
		JOIN core.angel_physical_dates a ON a.chunk_id = e.chunk_id
		WHERE e.ddmm = $1
		ORDER BY a.name
		LIMIT 1
	`, ddmm).Scan(&chunkID, &name)
	return chunkID, name, err == nil
}

func (b *Bot) fullContextByChunkID(ctx context.Context, chunkID string) (string, error) {
	var txt string
	err := b.pool.QueryRow(ctx, `SELECT context FROM core.document_context WHERE chunk_id = $1`, chunkID).Scan(&txt)
	return txt, err
}

func (b *Bot) composeReminderMessage(ctx context.Context, requestID, angelName, contextText string) (string, error) {
	if strings.TrimSpace(b.promptC) == "" {
		return "", fmt.Errorf("prompt C empty")
	}
	body := contextText
	if len(body) > 24000 {
		body = body[:24000]
	}
	user := "Имя ангела: " + angelName + "\n\nКонтекст:\n" + body
	out, err := b.callLLMMax(ctx, requestID, b.promptC, user, 256)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stripThink(out)), nil
}

// ensureJobPreparedForDeliveryDate готовит строку в reminder_jobs для указанного календарного дня МСК (дата доставки).
func (b *Bot) ensureJobPreparedForDeliveryDate(ctx context.Context, deliveryDate time.Time) {
	d0 := dateInMSK(deliveryDate)
	var exists bool
	_ = b.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM chat.reminder_jobs WHERE delivery_date_msk = $1::date)`, d0).Scan(&exists)
	if exists {
		return
	}

	prev := d0.AddDate(0, 0, -1)
	chPrev, _, okPrev := b.angelForDeliveryDate(ctx, prev)
	chDay, nameDay, okDay := b.angelForDeliveryDate(ctx, d0)
	if !okDay {
		_, _ = b.pool.Exec(ctx, `
			INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
			VALUES ($1::date, NULL, NULL, true, NOW(), false)
			ON CONFLICT (delivery_date_msk) DO NOTHING
		`, d0)
		return
	}

	if okPrev && chPrev == chDay {
		_, _ = b.pool.Exec(ctx, `
			INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
			VALUES ($1::date, $2, NULL, true, NOW(), false)
			ON CONFLICT (delivery_date_msk) DO NOTHING
		`, d0, chDay)
		return
	}

	ctxText, err := b.fullContextByChunkID(ctx, chDay)
	if err != nil || strings.TrimSpace(ctxText) == "" {
		_, _ = b.pool.Exec(ctx, `
			INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
			VALUES ($1::date, $2, NULL, true, NOW(), false)
			ON CONFLICT (delivery_date_msk) DO NOTHING
		`, d0, chDay)
		return
	}
	text, err := b.composeReminderMessage(ctx, uuid.New().String(), nameDay, ctxText)
	if err != nil || text == "" {
		log.Warn(ctx, "composeReminder failed", logging.KV{"error", err}, logging.KV{"chunk_id", chDay})
		_, _ = b.pool.Exec(ctx, `
			INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
			VALUES ($1::date, $2, NULL, true, NOW(), false)
			ON CONFLICT (delivery_date_msk) DO NOTHING
		`, d0, chDay)
		return
	}
	_, _ = b.pool.Exec(ctx, `
		INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
		VALUES ($1::date, $2, $3, false, NOW(), false)
		ON CONFLICT (delivery_date_msk) DO NOTHING
	`, d0, chDay, text)
}

func (b *Bot) runReminderTick(ctx context.Context) {
	if b.remindersGloballyDisabled(ctx) || b.bot == nil {
		return
	}
	eff := b.effectiveNow(ctx)
	today := dateInMSK(eff)
	tomorrow := today.AddDate(0, 0, 1)
	b.ensureJobPreparedForDeliveryDate(ctx, tomorrow)
	b.ensureJobPreparedForDeliveryDate(ctx, today)

	var skipped bool
	var msgText sql.NullString
	err := b.pool.QueryRow(ctx, `
		SELECT skipped_duplicate, message_text
		FROM chat.reminder_jobs WHERE delivery_date_msk = $1::date
	`, today).Scan(&skipped, &msgText)
	if err != nil || skipped || !msgText.Valid || strings.TrimSpace(msgText.String) == "" {
		return
	}
	text := strings.TrimSpace(msgText.String)
	debug := b.debugMode == 1

	rows, err := b.pool.Query(ctx, `
		SELECT telegram_id, chat_id, reminder_hh, reminder_mm FROM chat.reminder_subscribers WHERE enabled
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tg, chat int64
		var hh, mm int
		if rows.Scan(&tg, &chat, &hh, &mm) != nil {
			continue
		}
		if eff.Hour() != hh || eff.Minute() != mm {
			continue
		}
		var already bool
		_ = b.pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM chat.reminder_sent
				WHERE telegram_id = $1 AND chat_id = $2 AND delivery_date_msk = $3::date AND debug_mode = $4
			)
		`, tg, chat, today, debug).Scan(&already)
		if already {
			continue
		}
		b.sendReply(ctx, chat, text)
		_, _ = b.pool.Exec(ctx, `
			INSERT INTO chat.reminder_sent (telegram_id, chat_id, delivery_date_msk, debug_mode)
			VALUES ($1, $2, $3::date, $4)
			ON CONFLICT (telegram_id, chat_id, delivery_date_msk, debug_mode) DO NOTHING
		`, tg, chat, today, debug)
	}
}

func (b *Bot) sendTodayAngelReminderToChat(ctx context.Context, requestID string, chatID int64) {
	if b.remindersGloballyDisabled(ctx) || b.bot == nil {
		return
	}
	eff := b.effectiveNow(ctx)
	today := dateInMSK(eff)
	ch, name, ok := b.angelForDeliveryDate(ctx, today)
	if !ok {
		b.sendReply(ctx, chatID, "Напоминания включены. На сегодня в календаре нет ангела с физической датой.")
		return
	}
	ctxText, err := b.fullContextByChunkID(ctx, ch)
	if err != nil || strings.TrimSpace(ctxText) == "" {
		b.sendReply(ctx, chatID, "Напоминания включены. Полный контекст ангела пока недоступен.")
		return
	}
	text, err := b.composeReminderMessage(ctx, requestID, name, ctxText)
	if err != nil || text == "" {
		b.sendReply(ctx, chatID, "Напоминания включены. Не удалось сформулировать текст про сегодняшнего ангела.")
		return
	}
	b.sendReply(ctx, chatID, "Напоминание на сегодня:\n\n"+text)
}

func (b *Bot) reminderSchedulerLoop(ctx context.Context) {
	interval := time.Minute
	if b.debugMode == 1 {
		interval = 20 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rctx := context.Background()
			b.runReminderTick(rctx)
		}
	}
}

func (b *Bot) reconcileRemindersOnStartup(ctx context.Context) {
	if b.debugMode != 1 {
		now := b.realNowMSK()
		today := dateInMSK(now)
		b.ensureJobPreparedForDeliveryDate(ctx, today)
		b.ensureJobPreparedForDeliveryDate(ctx, today.AddDate(0, 0, 1))

		var skipped bool
		var msgText sql.NullString
		_ = b.pool.QueryRow(ctx, `
			SELECT skipped_duplicate, message_text FROM chat.reminder_jobs WHERE delivery_date_msk = $1::date
		`, today).Scan(&skipped, &msgText)
		if b.remindersGloballyDisabled(ctx) || skipped || !msgText.Valid {
			return
		}
		text := strings.TrimSpace(msgText.String)
		if text == "" {
			return
		}
		// Догон: если время напоминания уже прошло сегодня — отправить тем, кому ещё не слали (только prod).
		rows, err := b.pool.Query(ctx, `
			SELECT telegram_id, chat_id, reminder_hh, reminder_mm FROM chat.reminder_subscribers WHERE enabled
		`)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var tg, chat int64
			var rh, rm int
			if rows.Scan(&tg, &chat, &rh, &rm) != nil {
				continue
			}
			slot := time.Date(today.Year(), today.Month(), today.Day(), rh, rm, 0, 0, mskLoc)
			if now.Before(slot) {
				continue
			}
			var already bool
			_ = b.pool.QueryRow(ctx, `
				SELECT EXISTS(SELECT 1 FROM chat.reminder_sent
				WHERE telegram_id = $1 AND chat_id = $2 AND delivery_date_msk = $3::date AND debug_mode = false)
			`, tg, chat, today).Scan(&already)
			if already || b.bot == nil {
				continue
			}
			b.sendReply(ctx, chat, text)
			_, _ = b.pool.Exec(ctx, `
				INSERT INTO chat.reminder_sent (telegram_id, chat_id, delivery_date_msk, debug_mode)
				VALUES ($1, $2, $3::date, false)
				ON CONFLICT (telegram_id, chat_id, delivery_date_msk, debug_mode) DO NOTHING
			`, tg, chat, today)
		}
	}
}

func (b *Bot) tryHandleReminderDebugCommand(ctx context.Context, chatID int64, userText string) bool {
	if b.debugMode != 1 {
		return false
	}
	if _, ok := reminderDebugChatWhitelist[chatID]; !ok {
		return false
	}
	t := strings.TrimSpace(userText)
	low := strings.ToLower(t)
	if strings.HasPrefix(low, "/reminder_time_clear") || low == "/reminder_time_clear" {
		_, _ = b.pool.Exec(ctx, `UPDATE chat.reminder_debug_clock SET simulated_at = NULL, updated_at = NOW(), source = 'bot' WHERE id = 0`)
		b.sendReply(ctx, chatID, "Симуляция времени сброшена (используется реальное время).")
		return true
	}
	if !strings.HasPrefix(low, "/reminder_time") {
		return false
	}
	parts := strings.Fields(t)
	if len(parts) != 3 {
		b.sendReply(ctx, chatID, "Формат: /reminder_time ДД.ММ.ГГГГ ЧЧ:ММ (МСК). Например: /reminder_time 29.03.2026 09:30\nОчистка: /reminder_time_clear")
		return true
	}
	parsed, err := time.ParseInLocation("02.01.2006 15:04", parts[1]+" "+parts[2], mskLoc)
	if err != nil {
		b.sendReply(ctx, chatID, "Не удалось разобрать дату/время. Пример: /reminder_time 29.03.2026 09:30")
		return true
	}
	_, _ = b.pool.Exec(ctx, `UPDATE chat.reminder_debug_clock SET simulated_at = $1, updated_at = NOW(), source = 'bot' WHERE id = 0`, parsed)
	b.sendReply(ctx, chatID, fmt.Sprintf("Симуляция времени установлена: %s МСК. Запускаю проверку напоминаний.", parsed.Format("02.01.2006 15:04")))
	go func() {
		b.runReminderTick(context.Background())
	}()
	return true
}
