package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const telegramBotURL = "https://t.me/tet_mcp_bot"

var mskLoc *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		mskLoc = time.FixedZone("MSK", 3*3600)
	} else {
		mskLoc = loc
	}
}

func normalizeTelegramUsername(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	return strings.ToLower(s)
}

func parseClock(s string) (hh, mm int, err error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("time must be HH:MM")
	}
	hh, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	mm, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, fmt.Errorf("time out of range")
	}
	return hh, mm, nil
}

func nextTodayOrTomorrowMSK(hh, mm int, from time.Time) time.Time {
	from = from.In(mskLoc)
	today := time.Date(from.Year(), from.Month(), from.Day(), hh, mm, 0, 0, mskLoc)
	if !today.After(from) {
		return today.AddDate(0, 0, 1)
	}
	return today
}

func lookupUserTelegramID(ctx context.Context, pool *pgxpool.Pool, usernameNorm string) (int64, error) {
	var tg int64
	err := pool.QueryRow(ctx, `
		SELECT telegram_id FROM core.users
		WHERE telegram_id IS NOT NULL
		  AND lower(regexp_replace(trim(coalesce(username, '')), '^@', '', 'g')) = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, usernameNorm).Scan(&tg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			similar, _ := findClosestUsername(ctx, pool, usernameNorm)
			if similar != "" {
				return 0, fmt.Errorf("такого пользователя нет, но возможно вы имели в виду @%s; откройте %s, нажмите Start и повторите", similar, telegramBotURL)
			}
			return 0, fmt.Errorf("user not found: open %s, press Start, then retry", telegramBotURL)
		}
		return 0, err
	}
	return tg, nil
}

func findClosestUsername(ctx context.Context, pool *pgxpool.Pool, usernameNorm string) (string, error) {
	usernameNorm = normalizeTelegramUsername(usernameNorm)
	if usernameNorm == "" {
		return "", nil
	}
	var out string
	err := pool.QueryRow(ctx, `
		WITH u AS (
			SELECT lower(regexp_replace(trim(coalesce(username, '')), '^@', '', 'g')) AS uname
			FROM core.users
			WHERE telegram_id IS NOT NULL
			  AND trim(coalesce(username, '')) <> ''
		)
		SELECT uname
		FROM u
		ORDER BY
		  CASE
			WHEN uname = $1 THEN 0
			WHEN uname LIKE $1 || '%' THEN 1
			WHEN uname LIKE '%' || $1 || '%' THEN 2
			WHEN $1 LIKE uname || '%' THEN 3
			ELSE 4
		  END,
		  abs(length(uname) - length($1)),
		  uname
		LIMIT 1
	`, usernameNorm).Scan(&out)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

func lookupAngel(ctx context.Context, pool *pgxpool.Pool, nameRU, validation string) (chunkID, canonName string, err error) {
	err = pool.QueryRow(ctx, `
		SELECT chunk_id, name FROM core.angel_names
		WHERE lower(trim(name)) = lower(trim($1))
		   OR ($2 <> '' AND lower(trim(name)) = lower(trim($2)))
		LIMIT 1
	`, nameRU, validation).Scan(&chunkID, &canonName)
	return chunkID, canonName, err
}

func loadDocumentContext(ctx context.Context, pool *pgxpool.Pool, chunkID string) (string, error) {
	var txt string
	err := pool.QueryRow(ctx, `SELECT context FROM core.document_context WHERE chunk_id = $1`, chunkID).Scan(&txt)
	return txt, err
}

func groupItems(items []FromNoteItem) []groupedAngel {
	seen := make(map[string]groupedAngel)
	order := []string{}
	for _, it := range items {
		noteID := strings.TrimSpace(it.NoteItemID)
		key := noteID
		if key == "" {
			key = strings.TrimSpace(it.Name)
		}
		if key == "" {
			key = strings.TrimSpace(it.Validation)
		}
		if key == "" {
			continue
		}
		hh, mm, err := parseClock(it.Time)
		if err != nil {
			continue
		}
		if _, ok := seen[key]; !ok {
			order = append(order, key)
			seen[key] = groupedAngel{
				Key:         key,
				NoteItemID:  noteID,
				NameRU:      strings.TrimSpace(it.Name),
				Valid:       strings.TrimSpace(it.Validation),
				KeyName:     strings.TrimSpace(it.KeyName),
				TimeRaw:     strings.TrimSpace(it.Time),
				TimeHH:      hh,
				TimeMM:      mm,
				Part:        strings.TrimSpace(it.Part),
				Message:     strings.TrimSpace(it.Message),
				NotifyDaily: it.NotifyDaily,
			}
		} else {
			g := seen[key]
			if g.KeyName == "" && strings.TrimSpace(it.KeyName) != "" {
				g.KeyName = strings.TrimSpace(it.KeyName)
			}
			if g.TimeRaw == "" && strings.TrimSpace(it.Time) != "" {
				g.TimeRaw = strings.TrimSpace(it.Time)
			}
			if g.Part == "" && strings.TrimSpace(it.Part) != "" {
				g.Part = strings.TrimSpace(it.Part)
			}
			if g.Message == "" && strings.TrimSpace(it.Message) != "" {
				g.Message = strings.TrimSpace(it.Message)
			}
			if it.NotifyDaily {
				g.NotifyDaily = true
			}
			seen[key] = g
		}
	}
	out := make([]groupedAngel, 0, len(order))
	for _, k := range order {
		out = append(out, seen[k])
	}
	return out
}

func applyGoalPartToReminderText(text, goal, part string) string {
	out := strings.TrimSpace(text)
	goal = strings.TrimSpace(goal)
	part = strings.TrimSpace(part)

	// Подстановка маркеров из промпта C.
	out = strings.ReplaceAll(out, "[цель]", goal)
	out = strings.ReplaceAll(out, "[часть]", part)
	out = strings.ReplaceAll(out, "[Цель]", goal)
	out = strings.ReplaceAll(out, "[Часть]", part)

	// Иногда LLM дописывает "часть" последней строкой — срезаем этот хвост.
	if part != "" {
		t := strings.TrimSpace(out)
		if strings.HasSuffix(t, part) {
			t = strings.TrimSpace(strings.TrimSuffix(t, part))
		}
		out = t
	}
	return strings.TrimSpace(out)
}

func applyTimeToReminderText(text, keyName, timeRaw string, hh, mm int) string {
	out := strings.TrimSpace(text)
	marker := strings.TrimSpace(keyName)
	if marker == "" {
		marker = strings.TrimSpace(timeRaw)
	}
	if marker == "" {
		marker = fmt.Sprintf("%02d:%02d", hh, mm)
	}
	out = strings.ReplaceAll(out, "[время]", marker)
	out = strings.ReplaceAll(out, "[Время]", marker)
	return strings.TrimSpace(out)
}

// ProcessFromNote validates user, composes text via bot-service, inserts scheduler_notifications.
func ProcessFromNote(ctx context.Context, pool *pgxpool.Pool, bot *BotClient, req FromNoteRequest) FromNoteResponse {
	res := FromNoteResponse{Accepted: true}
	payloadJSON := "{}"
	if b, err := json.Marshal(req); err == nil {
		payloadJSON = string(b)
	}
	var requestID string
	_ = pool.QueryRow(ctx, `
		INSERT INTO chat.scheduler_note_requests (telegram_username, payload_json)
		VALUES ($1, $2::jsonb)
		RETURNING id::text
	`, req.TelegramUsername, payloadJSON).Scan(&requestID)
	if req.TelegramUsername == "" && req.TelegramID == 0 {
		res.Accepted = false
		res.Errors = append(res.Errors, "telegram_username or telegram_id required")
		if requestID != "" {
			_, _ = pool.Exec(ctx, `
			UPDATE chat.scheduler_note_requests
			SET accepted = false, errors_json = $2::jsonb
			WHERE id::text = $1
		`, requestID, `["telegram_username or telegram_id required"]`)
		}
		return res
	}
	var tgID int64
	if req.TelegramID != 0 {
		tgID = req.TelegramID
	} else {
		norm := normalizeTelegramUsername(req.TelegramUsername)
		if norm == "" {
			res.Accepted = false
			res.Errors = append(res.Errors, "telegram_username required")
			if requestID != "" {
				_, _ = pool.Exec(ctx, `
				UPDATE chat.scheduler_note_requests
				SET accepted = false, errors_json = $2::jsonb
				WHERE id::text = $1
			`, requestID, `["telegram_username required"]`)
			}
			return res
		}
		tgResolved, err := lookupUserTelegramID(ctx, pool, norm)
		if err != nil {
			res.Accepted = false
			res.Errors = append(res.Errors, err.Error())
			if requestID != "" {
				var errList []string
				errList = append(errList, err.Error())
				b, _ := json.Marshal(errList)
				_, _ = pool.Exec(ctx, `
				UPDATE chat.scheduler_note_requests
				SET accepted = false, errors_json = $2::jsonb
				WHERE id::text = $1
			`, requestID, string(b))
			}
			return res
		}
		tgID = tgResolved
	}
	chatID := tgID

	groups := groupItems(req.Items)
	if len(groups) == 0 {
		res.Accepted = false
		res.Errors = append(res.Errors, "no valid items (need name, time HH:MM)")
		_, _ = pool.Exec(ctx, `
			UPDATE chat.scheduler_note_requests
			SET accepted = false, telegram_id = $2, errors_json = $3::jsonb
			WHERE id::text = $1
		`, requestID, tgID, `["no valid items (need name, time HH:MM)"]`)
		return res
	}
	res.GroupedAngelsCount = len(groups)

	now := time.Now().In(mskLoc)
	total := 0
	composeCache := make(map[string]string)
	noteIDs := make([]string, 0, len(groups))

	for _, g := range groups {
		nameRU := g.NameRU
		if nameRU == "" {
			nameRU = g.Key
		}
		ch, displayName, errA := lookupAngel(ctx, pool, nameRU, g.Valid)
		if errA != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("angel not in DB: %s", nameRU))
			continue
		}
		text, ok := composeCache[ch]
		if !ok {
			ctxText, errC := loadDocumentContext(ctx, pool, ch)
			if errC != nil || strings.TrimSpace(ctxText) == "" {
				res.Errors = append(res.Errors, fmt.Sprintf("no document context for %s", displayName))
				continue
			}
			reqID := uuid.New().String()
			compiled, errL := bot.Compose(ctx, displayName, ctxText, reqID)
			if errL != nil || strings.TrimSpace(compiled) == "" {
				res.Errors = append(res.Errors, fmt.Sprintf("compose failed for %s: %v", displayName, errL))
				continue
			}
			_, _ = pool.Exec(ctx, `
				INSERT INTO chat.scheduler_compose_results
				  (telegram_id, angel_chunk_id, angel_name, reminder_text, request_id)
				VALUES ($1,$2,$3,$4,$5)
			`, tgID, ch, displayName, compiled, reqID)
			text = compiled
			composeCache[ch] = compiled
		}
		text = applyGoalPartToReminderText(text, g.Message, g.Part)
		text = applyTimeToReminderText(text, g.KeyName, g.TimeRaw, g.TimeHH, g.TimeMM)
		tSend := nextTodayOrTomorrowMSK(g.TimeHH, g.TimeMM, now)
		noteID := g.NoteItemID
		if noteID == "" {
			noteID = g.Key
		}
		if noteID != "" {
			noteIDs = append(noteIDs, noteID)
		}
		ct, errI := pool.Exec(ctx, `
			INSERT INTO chat.scheduler_notifications
			  (telegram_id, telegram_username, note_item_id, notify_daily, chat_id, angel_chunk_id, angel_name, message_text, send_at, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending')
			ON CONFLICT (telegram_id, note_item_id)
			DO UPDATE SET
			  telegram_username = EXCLUDED.telegram_username,
			  notify_daily      = EXCLUDED.notify_daily,
			  chat_id           = EXCLUDED.chat_id,
			  angel_chunk_id    = EXCLUDED.angel_chunk_id,
			  angel_name        = EXCLUDED.angel_name,
			  message_text      = EXCLUDED.message_text,
			  send_at           = EXCLUDED.send_at,
			  status            = 'pending',
			  sent_at           = NULL,
			  last_error        = NULL
		`, tgID, normalizeTelegramUsername(req.TelegramUsername), noteID, g.NotifyDaily, chatID, ch, displayName, text, tSend)
		if errI != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("insert: %v", errI))
			continue
		}
		if ct.RowsAffected() > 0 {
			total++
		}
	}
	if req.Sync {
		if len(noteIDs) == 0 {
			_, _ = pool.Exec(ctx, `
				DELETE FROM chat.scheduler_notifications
				WHERE telegram_id = $1
				  AND note_item_id IS NOT NULL
				  AND note_item_id <> ''
			`, tgID)
		} else {
			_, _ = pool.Exec(ctx, `
				DELETE FROM chat.scheduler_notifications
				WHERE telegram_id = $1
				  AND note_item_id IS NOT NULL
				  AND note_item_id <> ''
				  AND NOT (note_item_id = ANY($2))
			`, tgID, noteIDs)
		}
	}

	res.ScheduledCount = total
	if total == 0 && len(res.Errors) > 0 {
		res.Accepted = false
	}
	errJSON := "[]"
	if len(res.Errors) > 0 {
		if b, err := json.Marshal(res.Errors); err == nil {
			errJSON = string(b)
		}
	}
	if requestID != "" {
		_, _ = pool.Exec(ctx, `
		UPDATE chat.scheduler_note_requests
		SET telegram_id = $2, accepted = $3, errors_json = $4::jsonb
		WHERE id::text = $1
	`, requestID, tgID, res.Accepted, errJSON)
	}
	return res
}
