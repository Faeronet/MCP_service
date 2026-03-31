package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const noteBridgeKey = "__mcp_schedule_from_note__"

type bridgeFromNoteItem struct {
	Validation string `json:"validation"`
	Name       string `json:"name"`
	Time       string `json:"time"`
	Part       string `json:"part,omitempty"`
	Message    string `json:"message,omitempty"`
}

type bridgeFromNoteRequest struct {
	TelegramUsername string               `json:"telegram_username,omitempty"`
	TelegramID       int64                `json:"telegram_id,omitempty"`
	Items            []bridgeFromNoteItem `json:"items"`
}

// maybeExtractBridgePayload распознаёт JSON из вложения с noteBridgeKey.
// Поддерживает 2 формата:
// 1) {"__mcp_schedule_from_note__":true,"items":[...]}
// 2) export timeData map + "__mcp_schedule_from_note__": true (каждый объект-строка -> item)
func maybeExtractBridgePayload(rawText string) (bridgeFromNoteRequest, bool, error) {
	var out bridgeFromNoteRequest
	text := strings.TrimSpace(rawText)
	if text == "" {
		return out, false, nil
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return out, false, nil
	}
	text = text[start : end+1]

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		return out, false, nil
	}
	keyRaw, ok := obj[noteBridgeKey]
	if !ok {
		return out, false, nil
	}
	var enabled bool
	if err := json.Unmarshal(keyRaw, &enabled); err != nil || !enabled {
		return out, false, fmt.Errorf("invalid %s", noteBridgeKey)
	}

	// Формат items
	if rawItems, hasItems := obj["items"]; hasItems {
		var items []bridgeFromNoteItem
		if err := json.Unmarshal(rawItems, &items); err != nil {
			return out, true, fmt.Errorf("invalid items format")
		}
		out.Items = normalizeBridgeItems(items)
		return out, true, nil
	}

	// Формат timeData map (скачанный JSON note)
	var items []bridgeFromNoteItem
	for k, raw := range obj {
		if k == noteBridgeKey {
			continue
		}
		var row struct {
			Value      string `json:"value"`
			Validation string `json:"validation"`
			Name       string `json:"name"`
			PageName   string `json:"pageName"`
			Part       string `json:"часть"`
			Message    string `json:"message"`
			Goal       string `json:"цель"`
			Show       bool   `json:"show"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			continue
		}
		if row.Show == false {
			// В выгрузке берём только включённые записи.
			continue
		}
		items = append(items, bridgeFromNoteItem{
			Validation: strings.TrimSpace(row.Validation),
			Name:       strings.TrimSpace(firstNonEmpty(row.Name, row.Validation)),
			Time:       strings.TrimSpace(row.Value),
			Part:       strings.TrimSpace(firstNonEmpty(row.Part, row.PageName)),
			Message:    strings.TrimSpace(firstNonEmpty(row.Goal, row.Message)),
		})
	}
	out.Items = normalizeBridgeItems(items)
	return out, true, nil
}

func normalizeBridgeItems(items []bridgeFromNoteItem) []bridgeFromNoteItem {
	out := make([]bridgeFromNoteItem, 0, len(items))
	for _, it := range items {
		it.Validation = strings.TrimSpace(it.Validation)
		it.Name = strings.TrimSpace(firstNonEmpty(it.Name, it.Validation))
		it.Time = strings.TrimSpace(it.Time)
		it.Part = strings.TrimSpace(it.Part)
		it.Message = strings.TrimSpace(it.Message)
		if it.Validation == "" || it.Time == "" {
			continue
		}
		out = append(out, it)
	}
	return out
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func (s *Server) forwardBridgeToScheduler(ctx context.Context, userID int64, payload bridgeFromNoteRequest) (ChatResponse, error) {
	body := bridgeFromNoteRequest{
		TelegramID: userID, // для файла из TG заранее известен telegram_id пользователя
		Items:      payload.Items,
	}
	bb, _ := json.Marshal(body)
	url := strings.TrimSuffix(s.SchedulerURL, "/") + "/schedule/from-note"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bb))
	if err != nil {
		return ChatResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var sch struct {
		Accepted       bool     `json:"accepted"`
		ScheduledCount int      `json:"scheduled_count"`
		Errors         []string `json:"errors"`
	}
	_ = json.Unmarshal(respBody, &sch)
	if resp.StatusCode >= 300 || !sch.Accepted {
		msg := "Не удалось запланировать уведомления из JSON."
		if len(sch.Errors) > 0 {
			msg += " " + sch.Errors[0]
		}
		return ChatResponse{ReplyText: msg}, nil
	}
	return ChatResponse{
		ReplyText: fmt.Sprintf("JSON принят. Запланировано записей: %d.", sch.ScheduledCount),
	}, nil
}

