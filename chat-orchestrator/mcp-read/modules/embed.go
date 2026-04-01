package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logEmbed = logging.New("mcp-read.embed")

type EmbedClient struct {
	APIBase   string
	APIKey    string
	Model     string
	mu        sync.Mutex
	modelID   string
}

func NewEmbedClient(apiBase, apiKey, model string) *EmbedClient {
	return &EmbedClient{APIBase: apiBase, APIKey: apiKey, Model: model}
}

func (c *EmbedClient) ModelID(ctx context.Context) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.modelID != "" {
		return c.modelID
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.APIBase+"/models", nil)
	if err != nil {
		return c.Model
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return c.Model
	}
	defer resp.Body.Close()
	var out struct {
		Data []struct{ ID string `json:"id"` } `json:"data"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || len(out.Data) == 0 {
		return c.Model
	}
	c.modelID = out.Data[0].ID
	return c.modelID
}

func (c *EmbedClient) EmbedQuery(ctx context.Context, query string) []float32 {
	fallbackDim := config.LoadInt("EMBEDDING_DIMENSION", 1024)
	if query == "" || c.Model == "" {
		return make([]float32, fallbackDim)
	}
	modelID := c.ModelID(ctx)
	body := map[string]string{
		"model":           modelID,
		"input":            query,
		"encoding_format": "float",
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.APIBase+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		logEmbed.Warn(ctx, "embed request build", logging.KV{"error", err})
		return make([]float32, fallbackDim)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logEmbed.Warn(ctx, "embed request", logging.KV{"error", err})
		return make([]float32, fallbackDim)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logEmbed.Warn(ctx, "embed non-200", logging.KV{"status", resp.StatusCode})
		return make([]float32, fallbackDim)
	}
	var emb struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emb); err != nil || len(emb.Data) == 0 {
		return make([]float32, fallbackDim)
	}
	vec64 := emb.Data[0].Embedding
	vec := make([]float32, len(vec64))
	for i, v := range vec64 {
		vec[i] = float32(v)
	}
	return vec
}
