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

func parseDDMM(s string) (day, month int, err error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("ddmm")
	}
	day, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	month, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if day < 1 || day > 31 || month < 1 || month > 12 {
		return 0, 0, fmt.Errorf("bad date")
	}
	return day, month, nil
}

func nextOccurrenceMSK(day, month, hh, mm int, from time.Time) (time.Time, error) {
	from = from.In(mskLoc)
	t0 := time.Date(from.Year(), time.Month(month), day, hh, mm, 0, 0, mskLoc)
	if t0.Month() != time.Month(month) || t0.Day() != day {
		return time.Time{}, fmt.Errorf("invalid calendar day %d.%02d", day, month)
	}
	if !t0.After(from) {
		t0 = time.Date(from.Year()+1, time.Month(month), day, hh, mm, 0, 0, mskLoc)
		if t0.Month() != time.Month(month) || t0.Day() != day {
			return time.Time{}, fmt.Errorf("invalid calendar day (year+1) %d.%02d", day, month)
		}
	}
	return t0, nil
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
			return 0, fmt.Errorf("user not found: open %s, press Start, then retry", telegramBotURL)
		}
		return 0, err
	}
	return tg, nil
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

func loadPhysicalDDMMs(ctx context.Context, pool *pgxpool.Pool, chunkID string) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT ddmm FROM core.angel_physical_date_entries WHERE chunk_id = $1 ORDER BY ddmm
	`, chunkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if rows.Scan(&d) == nil && d != "" {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}

func groupItems(items []FromNoteItem) []groupedAngel {
	seen := make(map[string]groupedAngel)
	order := []string{}
	for _, it := range items {
		key := strings.TrimSpace(it.Name)
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
				Key:     key,
				NameRU:  strings.TrimSpace(it.Name),
				Valid:   strings.TrimSpace(it.Validation),
				TimeHH:  hh,
				TimeMM:  mm,
				Message: strings.TrimSpace(it.Message),
			}
		} else {
			g := seen[key]
			if g.Message == "" && strings.TrimSpace(it.Message) != "" {
				g.Message = strings.TrimSpace(it.Message)
				seen[key] = g
			}
		}
	}
	out := make([]groupedAngel, 0, len(order))
	for _, k := range order {
		out = append(out, seen[k])
	}
	return out
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
	if req.TelegramUsername == "" {
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
	tgID, err := lookupUserTelegramID(ctx, pool, norm)
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
		ctxText, errC := loadDocumentContext(ctx, pool, ch)
		if errC != nil || strings.TrimSpace(ctxText) == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("no document context for %s", displayName))
			continue
		}
		reqID := uuid.New().String()
		text, errL := bot.Compose(ctx, displayName, ctxText, reqID)
		if errL != nil || strings.TrimSpace(text) == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("compose failed for %s: %v", displayName, errL))
			continue
		}
		_, _ = pool.Exec(ctx, `
			INSERT INTO chat.scheduler_compose_results
			  (telegram_id, angel_chunk_id, angel_name, reminder_text, request_id)
			VALUES ($1,$2,$3,$4,$5)
		`, tgID, ch, displayName, text, reqID)
		if g.Message != "" {
			text = strings.TrimSpace(text) + "\n\n" + g.Message
		}
		ddmms, errD := loadPhysicalDDMMs(ctx, pool, ch)
		if errD != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("physical dates: %s: %v", displayName, errD))
			continue
		}
		if len(ddmms) == 0 {
			// No angel_physical_date_entries: schedule once tomorrow at the user's local (MSK) clock time.
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, mskLoc)
			nextDay := today.AddDate(0, 0, 1)
			tSend := time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), g.TimeHH, g.TimeMM, 0, 0, mskLoc)
			_, errI := pool.Exec(ctx, `
				INSERT INTO chat.scheduler_notifications
				  (telegram_id, chat_id, angel_chunk_id, angel_name, message_text, send_at, status)
				VALUES ($1,$2,$3,$4,$5,$6,'pending')
				ON CONFLICT DO NOTHING
			`, tgID, chatID, ch, displayName, text, tSend)
			if errI != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("insert: %v", errI))
				continue
			}
			total++
			continue
		}
		// Одно уведомление на ангела: ближайшая по времени физическая дата, а не по строке на каждую DD.MM в таблице.
		var (
			bestSend time.Time
			hasBest  bool
		)
		for _, raw := range ddmms {
			day, month, errP := parseDDMM(raw)
			if errP != nil {
				continue
			}
			tSend, errN := nextOccurrenceMSK(day, month, g.TimeHH, g.TimeMM, now)
			if errN != nil {
				continue
			}
			if !hasBest || tSend.Before(bestSend) {
				bestSend = tSend
				hasBest = true
			}
		}
		if !hasBest {
			res.Errors = append(res.Errors, fmt.Sprintf("no valid physical dates for %s", displayName))
			continue
		}
		_, errI := pool.Exec(ctx, `
			INSERT INTO chat.scheduler_notifications
			  (telegram_id, chat_id, angel_chunk_id, angel_name, message_text, send_at, status)
			VALUES ($1,$2,$3,$4,$5,$6,'pending')
			ON CONFLICT DO NOTHING
		`, tgID, chatID, ch, displayName, text, bestSend)
		if errI != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("insert: %v", errI))
			continue
		}
		total++
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
