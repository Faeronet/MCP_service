package modules

import (
	"os"
	"time"

	"github.com/telegram-ai-assistant/root/pkg/config"
)

type Config struct {
	ListenAddr       string
	PostgresDSN      string
	McpProxyURL      string
	InternalSecret   string
	PollInterval     time.Duration
	DispatchInterval time.Duration
}

func LoadConfig() Config {
	poll := config.LoadDuration("SCHEDULER_POLL_INTERVAL", 10*time.Minute)
	if poll < time.Minute {
		poll = time.Minute
	}
	dispatch := config.LoadDuration("SCHEDULER_DISPATCH_INTERVAL", 30*time.Second)
	if dispatch < 5*time.Second {
		dispatch = 5 * time.Second
	}
	return Config{
		ListenAddr:       config.LoadString("SCHEDULER_LISTEN", ":8090"),
		PostgresDSN:      config.LoadPostgres().DSN,
		McpProxyURL:      trimSlash(config.LoadString("MCP_PROXY_URL", "http://mcp-proxy:8083")),
		InternalSecret:   os.Getenv("SCHEDULER_INTERNAL_SECRET"),
		PollInterval:     poll,
		DispatchInterval: dispatch,
	}
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
