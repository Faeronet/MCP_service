package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// CallExtract sends file to extract-tool (POST /extract). Returns (text, userMessage, error). On 422, userMessage is for the user.
func (b *Bot) CallExtract(ctx context.Context, data []byte, fileName string) (text, userErr string, err error) {
	if b.ExtractToolURL == "" {
		return "", "", fmt.Errorf("EXTRACT_TOOL_URL not set")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return "", "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", "", err
	}
	if err := w.Close(); err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.ExtractToolURL+"/extract", &buf)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	client := &http.Client{Timeout: 600 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 422 {
		var errBody struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(bodyBytes, &errBody)
		msg := errBody.Detail
		if msg == "" {
			msg = "Файл или архив нельзя обработать."
		}
		return "", msg, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("extract %d: %s", resp.StatusCode, string(bodyBytes))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return "", "", err
	}
	return out.Text, "", nil
}

// InsertAttachment saves attachment record (session_id, object_key, extracted_text, status=done).
func (b *Bot) InsertAttachment(ctx context.Context, sessionID uuid.UUID, objectKey, extractedText string) error {
	_, err := b.Pool.Exec(ctx,
		`INSERT INTO chat.attachments (session_id, object_key, extracted_text, status) VALUES ($1, $2, $3, 'done')`,
		sessionID, objectKey, extractedText)
	return err
}
