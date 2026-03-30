package modules

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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

// Триггер напоминания и время ЧЧ:ММ.
// Принимаем и новый "[напоминание]", и старый "[reminder]" (обратная совместимость).
// Ищем как подстроку, чтобы переживать лишние пробелы/служебный хвост модели.
var reReminderLine = regexp.MustCompile(`(?i)\[(?:напоминание|reminder)\]\s*([01]?\d|2[0-3])\s*:\s*([0-5]\d)(?:\s*:\s*[0-5]\d)?`)

func formatDDMMMSK(t time.Time) string {
	x := t.In(mskLoc)
	return fmt.Sprintf("%02d.%02d", x.Day(), int(x.Month()))
}

func dateInMSK(t time.Time) time.Time {
	x := t.In(mskLoc)
	return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, mskLoc)
}

func (s *Server) effectiveReminderNow(ctx context.Context) time.Time {
	if s.DebugMode != 1 {
		return time.Now().In(mskLoc)
	}
	var sim sql.NullTime
	err := s.Pool.QueryRow(ctx, `SELECT simulated_at FROM chat.reminder_debug_clock WHERE id = 0`).Scan(&sim)
	if err == nil && sim.Valid {
		return sim.Time.In(mskLoc)
	}
	return time.Now().In(mskLoc)
}

func (s *Server) realNowMSK() time.Time {
	return time.Now().In(mskLoc)
}

func (s *Server) remindersGloballyDisabled(ctx context.Context) bool {
	var d bool
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(disabled, false) FROM chat.reminder_global_config WHERE id = 0`).Scan(&d)
	return err == nil && d
}

// ParseReminderLine распознаёт ответ промпта A: "[напоминание] HH:MM" (МСК), например [напоминание] 12:00
func ParseReminderLine(line string) (hh, mm int, ok bool) {
	line = strings.TrimSpace(StripThink(line))
	// Нормализация частых артефактов LLM/копипаста.
	line = strings.NewReplacer(
		"\u00a0", " ", // NBSP
		"：", ":",      // полноширинное двоеточие
		"`", " ",
	).Replace(line)
	m := reReminderLine.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, false
	}
	hh, _ = strconv.Atoi(m[1])
	mm, _ = strconv.Atoi(m[2])
	return hh, mm, true
}

func (s *Server) UpsertReminderSubscriber(ctx context.Context, telegramID, chatID int64, hh, mm int) error {
	_, err := s.Pool.Exec(ctx, `
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

func (s *Server) angelForDeliveryDate(ctx context.Context, deliveryDate time.Time) (chunkID, name string, ok bool) {
	ddmm := formatDDMMMSK(deliveryDate)
	err := s.Pool.QueryRow(ctx, `
		SELECT a.chunk_id, a.name
		FROM core.angel_physical_date_entries e
		JOIN core.angel_physical_dates a ON a.chunk_id = e.chunk_id
		WHERE e.ddmm = $1
		ORDER BY a.name
		LIMIT 1
	`, ddmm).Scan(&chunkID, &name)
	return chunkID, name, err == nil
}

func (s *Server) fullContextByChunkID(ctx context.Context, chunkID string) (string, error) {
	var txt string
	err := s.Pool.QueryRow(ctx, `SELECT context FROM core.document_context WHERE chunk_id = $1`, chunkID).Scan(&txt)
	return txt, err
}

func (s *Server) composeReminderLLM(ctx context.Context, requestID, angelName, contextText string) (string, error) {
	if strings.TrimSpace(s.PromptC) == "" {
		return "", fmt.Errorf("prompt C empty")
	}
	body := contextText
	if len(body) > 24000 {
		body = body[:24000]
	}
	user := "Имя ангела: " + angelName + "\n\nКонтекст:\n" + body
	out, err := s.callLLMWithBudget(ctx, requestID, s.PromptC, user, 256)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(StripThink(out)), nil
}

// FallbackSearchQueryAfterReminder — поисковая строка для RAG, если промпт A вернул подписку на напоминание.
func (s *Server) FallbackSearchQueryAfterReminder(userMsg string) string {
	translated := TranslateMonthToRussian(userMsg)
	userDateStr := ExtractDateFromQuestion(translated)
	var searchQuery string
	if userDateStr != "" {
		searchQuery = userDateStr
	} else {
		searchQuery = translated
	}
	lowerMsg := strings.ToLower(strings.TrimSpace(userMsg))
	if HasAngelWord(userMsg) && strings.Contains(lowerMsg, "дат") {
		searchQuery = "[date] list"
	}
	return searchQuery
}

func (s *Server) ensureJobPreparedForDeliveryDate(ctx context.Context, deliveryDate time.Time) {
	d0 := dateInMSK(deliveryDate)
	var exists bool
	_ = s.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM chat.reminder_jobs WHERE delivery_date_msk = $1::date)`, d0).Scan(&exists)
	if exists {
		return
	}
	prev := d0.AddDate(0, 0, -1)
	chPrev, _, okPrev := s.angelForDeliveryDate(ctx, prev)
	chDay, nameDay, okDay := s.angelForDeliveryDate(ctx, d0)
	if !okDay {
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
			VALUES ($1::date, NULL, NULL, true, NOW(), false)
			ON CONFLICT (delivery_date_msk) DO NOTHING
		`, d0)
		return
	}
	if okPrev && chPrev == chDay {
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
			VALUES ($1::date, $2, NULL, true, NOW(), false)
			ON CONFLICT (delivery_date_msk) DO NOTHING
		`, d0, chDay)
		return
	}
	ctxText, err := s.fullContextByChunkID(ctx, chDay)
	if err != nil || strings.TrimSpace(ctxText) == "" {
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
			VALUES ($1::date, $2, NULL, true, NOW(), false)
			ON CONFLICT (delivery_date_msk) DO NOTHING
		`, d0, chDay)
		return
	}
	text, err := s.composeReminderLLM(ctx, uuid.New().String(), nameDay, ctxText)
	if err != nil || text == "" {
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
			VALUES ($1::date, $2, NULL, true, NOW(), false)
			ON CONFLICT (delivery_date_msk) DO NOTHING
		`, d0, chDay)
		return
	}
	_, _ = s.Pool.Exec(ctx, `
		INSERT INTO chat.reminder_jobs (delivery_date_msk, angel_chunk_id, message_text, skipped_duplicate, prepared_at, prod_complete)
		VALUES ($1::date, $2, $3, false, NOW(), false)
		ON CONFLICT (delivery_date_msk) DO NOTHING
	`, d0, chDay, text)
}

// ReminderNotify одно сообщение для отправки из tg-bot.
type ReminderNotify struct {
	TelegramID int64  `json:"telegram_id"`
	ChatID     int64  `json:"chat_id"`
	Text       string `json:"text"`
}

// TickReminders готовит джобы и возвращает уведомления для текущей минуты (эффективное время МСК).
func (s *Server) TickReminders(ctx context.Context) ([]ReminderNotify, error) {
	if s.remindersGloballyDisabled(ctx) {
		return nil, nil
	}
	eff := s.effectiveReminderNow(ctx)
	today := dateInMSK(eff)
	tomorrow := today.AddDate(0, 0, 1)
	s.ensureJobPreparedForDeliveryDate(ctx, tomorrow)
	s.ensureJobPreparedForDeliveryDate(ctx, today)

	var skipped bool
	var msgText sql.NullString
	err := s.Pool.QueryRow(ctx, `
		SELECT skipped_duplicate, message_text
		FROM chat.reminder_jobs WHERE delivery_date_msk = $1::date
	`, today).Scan(&skipped, &msgText)
	if err != nil || skipped || !msgText.Valid || strings.TrimSpace(msgText.String) == "" {
		return nil, nil
	}
	text := strings.TrimSpace(msgText.String)
	debug := s.DebugMode == 1

	rows, err := s.Pool.Query(ctx, `
		SELECT telegram_id, chat_id, reminder_hh, reminder_mm FROM chat.reminder_subscribers WHERE enabled
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ReminderNotify
	for rows.Next() {
		var tg, chat int64
		var hh, mm int
		if rows.Scan(&tg, &chat, &hh, &mm) != nil {
			continue
		}
		slotPast := (eff.Hour() > hh) || (eff.Hour() == hh && eff.Minute() >= mm)
		atSlot := eff.Hour() == hh && eff.Minute() == mm
		if debug {
			if !atSlot {
				continue
			}
		} else {
			if !atSlot && !slotPast {
				continue
			}
		}
		var already bool
		_ = s.Pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM chat.reminder_sent
				WHERE telegram_id = $1 AND chat_id = $2 AND delivery_date_msk = $3::date AND debug_mode = $4
			)
		`, tg, chat, today, debug).Scan(&already)
		if already {
			continue
		}
		out = append(out, ReminderNotify{TelegramID: tg, ChatID: chat, Text: text})
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO chat.reminder_sent (telegram_id, chat_id, delivery_date_msk, debug_mode)
			VALUES ($1, $2, $3::date, $4)
			ON CONFLICT (telegram_id, chat_id, delivery_date_msk, debug_mode) DO NOTHING
		`, tg, chat, today, debug)
	}
	return out, nil
}

// BuildTodayAngelReminderText — текст сразу после активации напоминания (один пользователь).
func (s *Server) BuildTodayAngelReminderText(ctx context.Context, requestID string) string {
	if s.remindersGloballyDisabled(ctx) {
		return ""
	}
	eff := s.effectiveReminderNow(ctx)
	today := dateInMSK(eff)
	ch, name, ok := s.angelForDeliveryDate(ctx, today)
	if !ok {
		return "Напоминания включены. На сегодня в календаре нет ангела с физической датой."
	}
	ctxText, err := s.fullContextByChunkID(ctx, ch)
	if err != nil || strings.TrimSpace(ctxText) == "" {
		return "Напоминания включены. Полный контекст ангела пока недоступен."
	}
	text, err := s.composeReminderLLM(ctx, requestID, name, ctxText)
	if err != nil || text == "" {
		return "Напоминания включены. Не удалось сформулировать текст про сегодняшнего ангела."
	}
	return "Напоминание на сегодня:\n\n" + text
}


