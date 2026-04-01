package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type BotClient struct {
	base   string
	secret string
	client *http.Client
}

func NewBotClient(base, secret string) *BotClient {
	return &BotClient{
		base:   base,
		secret: secret,
		client: &http.Client{Timeout: 3 * time.Minute},
	}
}

type composeReq struct {
	AngelName   string `json:"angel_name"`
	ContextText string `json:"context_text"`
	RequestID   string `json:"request_id"`
}

type composeRes struct {
	ReminderText string `json:"reminder_text"`
	Error        string `json:"error,omitempty"`
}

type deliverReq struct {
	TelegramID int64  `json:"telegram_id"`
	ChatID     int64  `json:"chat_id"`
	Text       string `json:"text"`
}

type deliverRes struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (c *BotClient) Compose(ctx context.Context, angelName, contextText, requestID string) (string, error) {
	if c.base == "" {
		return "", fmt.Errorf("MCP_PROXY_URL empty")
	}
	body, _ := json.Marshal(composeReq{
		AngelName:   angelName,
		ContextText: contextText,
		RequestID:   requestID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/scheduler/compose", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("X-Scheduler-Secret", c.secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("compose %d: %s", resp.StatusCode, string(raw))
	}
	var out composeRes
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("%s", out.Error)
	}
	return out.ReminderText, nil
}

func (c *BotClient) Deliver(ctx context.Context, telegramID, chatID int64, text string) error {
	body, _ := json.Marshal(deliverReq{
		TelegramID: telegramID,
		ChatID:     chatID,
		Text:       text,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/scheduler/deliver", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("X-Scheduler-Secret", c.secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deliver %d: %s", resp.StatusCode, string(raw))
	}
	var out deliverRes
	_ = json.Unmarshal(raw, &out)
	if out.Error != "" {
		return fmt.Errorf("%s", out.Error)
	}
	return nil
}
