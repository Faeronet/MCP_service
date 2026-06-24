package modules

import (
	"net/http"

	"github.com/telegram-ai-assistant/root/pkg/config"
)

func setOpenRouterHeaders(req *http.Request) {
	config.SetOpenRouterHeaders(req)
}
