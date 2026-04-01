package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadString returns env value or default.
func LoadString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// LoadInt returns env as int or default.
func LoadInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

// LoadDuration returns env as duration or default.
func LoadDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// LoadBool returns env as bool (1, true, yes) or default.
func LoadBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		v = strings.ToLower(strings.TrimSpace(v))
		return v == "1" || v == "true" || v == "yes"
	}
	return def
}

// Postgres config
type Postgres struct {
	DSN string
}

func LoadPostgres() Postgres {
	return Postgres{
		DSN: LoadString("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/assistant?sslmode=disable"),
	}
}

// MinIO config
type MinIO struct {
	Endpoint        string
	AccessKey       string
	SecretKey       string
	Bucket          string
	UseSSL          bool
	AttachmentsBucket string
}

func LoadMinIO() MinIO {
	return MinIO{
		Endpoint:          LoadString("MINIO_ENDPOINT", "minio:9000"),
		AccessKey:         LoadString("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey:         LoadString("MINIO_SECRET_KEY", "minioadmin"),
		Bucket:            LoadString("MINIO_BUCKET", "documents"),
		UseSSL:            LoadBool("MINIO_USE_SSL", false),
		AttachmentsBucket: LoadString("MINIO_ATTACHMENTS_BUCKET", "attachments"),
	}
}

// RabbitMQ config
type RabbitMQ struct {
	URL string
}

func LoadRabbitMQ() RabbitMQ {
	return RabbitMQ{
		URL: LoadString("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
	}
}

// Redis config (for rate limit / cache)
type Redis struct {
	Addr     string
	Password string
	DB       int
}

func LoadRedis() Redis {
	return Redis{
		Addr:     LoadString("REDIS_ADDR", "redis:6379"),
		Password: LoadString("REDIS_PASSWORD", ""),
		DB:       LoadInt("REDIS_DB", 0),
	}
}
