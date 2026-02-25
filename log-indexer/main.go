package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"

	"github.com/jackc/pgx/v5/pgxpool"
)

var log = logging.New("log-indexer")

func main() {
	ctx := context.Background()
	pg := config.LoadPostgres()
	pool, err := pgxpool.New(ctx, pg.DSN)
	if err != nil {
		log.Error(ctx, "postgres connect", logging.KV{"error", err})
		os.Exit(1)
	}
	defer pool.Close()

	lokiURL := config.LoadString("LOKI_URL", "http://loki:3100")
	interval := config.LoadDuration("INDEX_INTERVAL_SEC", 30*time.Second)
	if interval < time.Second {
		interval = 30 * time.Second
	}
	log.Info(ctx, "log-indexer started", logging.KV{"loki", lokiURL}, logging.KV{"interval_sec", int(interval.Seconds())})

	// Ensure schema and table (на случай если migrate не создал obs)
	_, _ = pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS obs;`)
	_, _ = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS obs.logs_index (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			ts TIMESTAMPTZ NOT NULL,
			level TEXT,
			service TEXT,
			request_id TEXT,
			message TEXT,
			log_id TEXT,
			raw_ref TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_logs_index_ts ON obs.logs_index(ts);
	`)

	// Первый прогон сразу, чтобы логи появились в панели без ожидания interval
	if err := indexLoki(ctx, pool, lokiURL); err != nil {
		log.Warn(ctx, "index run (startup)", logging.KV{"error", err.Error()})
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := indexLoki(ctx, pool, lokiURL); err != nil {
			log.Warn(ctx, "index run", logging.KV{"error", err.Error()})
		}
	}
}

type lokiEntry struct {
	Ts   string            `json:"ts"`
	Line string            `json:"line"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type lokiResponse struct {
	Data struct {
		Result []lokiStream `json:"result"`
	} `json:"data"`
}

func indexLoki(ctx context.Context, pool *pgxpool.Pool, baseURL string) error {
	// Окно 30 минут: Loki может отдавать данные с задержкой (flush), плюс возможен сдвиг времени
	start := time.Now().Add(-30 * time.Minute)
	end := time.Now()
	u, _ := url.Parse(baseURL + "/loki/api/v1/query_range")
	q := u.Query()
	// Loki не принимает пустой {}. Используем container — он всегда есть у Promtail (docker_sd).
	q.Set("query", `{container=~".+"}`)
	q.Set("start", fmt.Sprintf("%d", start.UnixNano()))
	q.Set("end", fmt.Sprintf("%d", end.UnixNano()))
	q.Set("limit", "500")
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("loki %d: %s", resp.StatusCode, string(body))
	}

	var data lokiResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	nStreams := len(data.Data.Result)
	nInserted := 0
	for _, stream := range data.Data.Result {
		service := stream.Stream["service"]
		if service == "" {
			service = stream.Stream["job"]
		}
		for _, v := range stream.Values {
			if len(v) < 2 {
				continue
			}
			tsStr, line := v[0], v[1]
			var parsed struct {
				Level     string `json:"level"`
				Service   string `json:"service"`
				RequestID string `json:"request_id"`
				Message   string `json:"message"`
			}
			_ = json.Unmarshal([]byte(line), &parsed)
			if parsed.Service != "" {
				service = parsed.Service
			}
			message := parsed.Message
			if message == "" && line != "" {
				message = line
			}
			parseKeyValueLine(line, &parsed.Level, &parsed.RequestID, &parsed.Service)
			if parsed.Service != "" {
				service = parsed.Service
			}
			var ts time.Time
			if ns, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
				ts = time.Unix(0, ns)
			} else {
				ts, _ = time.Parse(time.RFC3339Nano, tsStr)
			}
			_, err := pool.Exec(ctx, `
				INSERT INTO obs.logs_index (ts, level, service, request_id, message, log_id)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, ts, parsed.Level, service, parsed.RequestID, message, uuid.New().String())
			if err != nil {
				continue
			}
			nInserted++
		}
	}
	// Логируем каждый прогон: при 0 видно, что Loki пустой; при >0 — что данные пошли в Postgres
	log.Info(ctx, "index run", logging.KV{"streams", nStreams}, logging.KV{"inserted", nInserted})
	return nil
}

var (
	reLevel     = regexp.MustCompile(`level=(\S+)`)
	reTraceID   = regexp.MustCompile(`traceID=([a-zA-Z0-9]+)`)
	reComponent = regexp.MustCompile(`component=(\S+)`)
	reService   = regexp.MustCompile(`service=(\S+)`)
)

// parseKeyValueLine извлекает level, traceID (request_id), service/component из строки в формате key=value (логи Loki, Go и т.д.).
func parseKeyValueLine(line string, level, requestID, service *string) {
	if line == "" {
		return
	}
	if level != nil && *level == "" {
		if m := reLevel.FindStringSubmatch(line); len(m) > 1 {
			*level = m[1]
		}
	}
	if requestID != nil && *requestID == "" {
		if m := reTraceID.FindStringSubmatch(line); len(m) > 1 {
			*requestID = m[1]
		}
	}
	if service != nil && *service == "" {
		if m := reService.FindStringSubmatch(line); len(m) > 1 {
			*service = m[1]
		} else if m := reComponent.FindStringSubmatch(line); len(m) > 1 {
			*service = m[1]
		}
	}
}
