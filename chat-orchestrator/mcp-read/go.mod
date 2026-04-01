module github.com/telegram-ai-assistant/root/chat-orchestrator/mcp-read

go 1.23.0

replace github.com/telegram-ai-assistant/root/pkg => ../pkg

require (
	github.com/google/uuid v1.6.0
	github.com/redis/go-redis/v9 v9.4.0
	github.com/telegram-ai-assistant/root/pkg v0.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)
