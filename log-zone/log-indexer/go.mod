module github.com/telegram-ai-assistant/root/log-zone/log-indexer

go 1.23.0

replace github.com/telegram-ai-assistant/root/pkg => ../pkg

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.5.1
	github.com/telegram-ai-assistant/root/pkg v0.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	golang.org/x/crypto v0.16.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)
