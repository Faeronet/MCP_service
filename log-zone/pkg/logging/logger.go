package logging

import (
	"context"
	"encoding/json"
	"os"
	"time"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Entry struct {
	TS         time.Time              `json:"ts"`
	Level      Level                  `json:"level"`
	Service    string                 `json:"service"`
	RequestID  string                 `json:"request_id,omitempty"`
	Message    string                 `json:"message"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
}

type Logger struct {
	service string
}

func New(service string) *Logger {
	return &Logger{service: service}
}

func (l *Logger) requestID(ctx context.Context) string {
	if id, _ := ctx.Value(ctxKeyRequestID).(string); id != "" {
		return id
	}
	return ""
}

func (l *Logger) log(ctx context.Context, level Level, msg string, fields map[string]interface{}) {
	e := Entry{
		TS:        time.Now().UTC(),
		Level:     level,
		Service:   l.service,
		RequestID: l.requestID(ctx),
		Message:   msg,
		Fields:    fields,
	}
	if e.Fields == nil {
		e.Fields = make(map[string]interface{})
	}
	_ = json.NewEncoder(os.Stdout).Encode(e)
}

func (l *Logger) Debug(ctx context.Context, msg string, fields ...KV) { l.log(ctx, LevelDebug, msg, kvToMap(fields)) }
func (l *Logger) Info(ctx context.Context, msg string, fields ...KV)  { l.log(ctx, LevelInfo, msg, kvToMap(fields)) }
func (l *Logger) Warn(ctx context.Context, msg string, fields ...KV) { l.log(ctx, LevelWarn, msg, kvToMap(fields)) }
func (l *Logger) Error(ctx context.Context, msg string, fields ...KV) { l.log(ctx, LevelError, msg, kvToMap(fields)) }

type KV struct{ K string; V interface{} }

func kvToMap(kv []KV) map[string]interface{} {
	m := make(map[string]interface{}, len(kv))
	for _, p := range kv {
		m[p.K] = p.V
	}
	return m
}

type ctxKey string

const ctxKeyRequestID ctxKey = "request_id"

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}
