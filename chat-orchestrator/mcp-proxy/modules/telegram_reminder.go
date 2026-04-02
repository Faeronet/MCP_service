package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logTGRem = logging.New("mcp-proxy.telegram-reminder")

// TelegramSendPhoto sends photo with caption (caption appears below image in Telegram).
func (s *Server) TelegramSendPhoto(ctx context.Context, chatID int64, imagePath, caption string) (messageID int, err error) {
	token := strings.TrimSpace(s.TelegramBotToken)
	if token == "" {
		return 0, fmt.Errorf("no bot token")
	}
	f, err := os.Open(imagePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	// Подпись ≤1024 символов (Telegram); длинный текст доклеивается отдельными sendMessage в HandleSchedulerDeliver.
	_ = w.WriteField("caption", caption)
	part, err := w.CreateFormFile("photo", filepath.Base(imagePath))
	if err != nil {
		return 0, err
	}
	if _, err = io.Copy(part, f); err != nil {
		return 0, err
	}
	if err = w.Close(); err != nil {
		return 0, err
	}

	url := "https://api.telegram.org/bot" + token + "/sendPhoto"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		logTGRem.Warn(ctx, "sendPhoto failed", logging.KV{"status", resp.StatusCode}, logging.KV{"body", string(raw)})
		return 0, fmt.Errorf("telegram sendPhoto %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return 0, err
	}
	return out.Result.MessageID, nil
}
