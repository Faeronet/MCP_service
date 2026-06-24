package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logRerank = logging.New("mcp-read.rerank")

type RerankClient struct {
	APIURL string
	APIKey string
	Model  string
}

func NewRerankClient(apiURL, apiKey, model string) *RerankClient {
	if strings.TrimSpace(model) == "" {
		return nil
	}
	return &RerankClient{APIURL: apiURL, APIKey: apiKey, Model: model}
}

func (c *RerankClient) RerankWithScoreAndOrder(ctx context.Context, query string, texts []string) ([]string, []int, float64) {
	if len(texts) == 0 {
		return texts, nil, 0
	}
	docsWithID := make([]map[string]interface{}, len(texts))
	for i, t := range texts {
		docsWithID[i] = map[string]interface{}{"id": i, "text": t}
	}
	body := map[string]interface{}{
		"query":     query,
		"documents": docsWithID,
		"model":     c.Model,
	}
	payload, _ := json.Marshal(body)
	rerankURL := strings.TrimSuffix(c.APIURL, "/")
	if rerankURL != "" && !strings.HasSuffix(rerankURL, "/rerank") {
		if strings.HasSuffix(rerankURL, "/api/v1") {
			rerankURL = rerankURL + "/rerank"
		} else {
			rerankURL = rerankURL + "/api/v1/rerank"
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rerankURL, bytes.NewReader(payload))
	if err != nil {
		logRerank.Warn(ctx, "rerank request build", logging.KV{"error", err})
		return texts, nil, 0
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	config.SetOpenRouterHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logRerank.Warn(ctx, "rerank request failed", logging.KV{"error", err}, logging.KV{"url", rerankURL})
		return texts, nil, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logRerank.Warn(ctx, "rerank non-200", logging.KV{"status", resp.StatusCode}, logging.KV{"body", string(body)})
		return texts, nil, 0
	}
	var result struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
		Scores []float64 `json:"scores"`
		Data   []struct {
			ID         interface{} `json:"id"`
			Similarity float64     `json:"similarity"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logRerank.Warn(ctx, "rerank decode failed", logging.KV{"error", err})
		return texts, nil, 0
	}

	var topScore float64
	var order []int
	if len(result.Data) > 0 {
		topScore = result.Data[0].Similarity
		out := make([]string, 0, len(result.Data))
		order = make([]int, 0, len(result.Data))
		for _, d := range result.Data {
			var idx int
			switch v := d.ID.(type) {
			case float64:
				idx = int(v)
			case int:
				idx = v
			default:
				continue
			}
			if idx >= 0 && idx < len(texts) {
				out = append(out, texts[idx])
				order = append(order, idx)
			}
		}
		if len(out) > 0 {
			return out, order, topScore
		}
	}
	if len(result.Scores) >= len(texts) {
		type pair struct{ i int; s float64 }
		ps := make([]pair, len(texts))
		for i := range texts {
			ps[i] = pair{i, result.Scores[i]}
		}
		for i := 0; i < len(ps); i++ {
			for j := i + 1; j < len(ps); j++ {
				if ps[j].s > ps[i].s {
					ps[i], ps[j] = ps[j], ps[i]
				}
			}
		}
		topScore = ps[0].s
		out := make([]string, len(texts))
		order = make([]int, len(texts))
		for i, p := range ps {
			out[i] = texts[p.i]
			order[i] = p.i
		}
		return out, order, topScore
	}
	if len(result.Results) > 0 {
		ps := make([]struct{ i int; s float64 }, len(result.Results))
		for i, r := range result.Results {
			ps[i] = struct{ i int; s float64 }{r.Index, r.RelevanceScore}
		}
		for i := 0; i < len(ps); i++ {
			for j := i + 1; j < len(ps); j++ {
				if ps[j].s > ps[i].s {
					ps[i], ps[j] = ps[j], ps[i]
				}
			}
		}
		topScore = ps[0].s
		out := make([]string, 0, len(ps))
		order = make([]int, 0, len(ps))
		for _, p := range ps {
			if p.i >= 0 && p.i < len(texts) {
				out = append(out, texts[p.i])
				order = append(order, p.i)
			}
		}
		if len(out) > 0 {
			return out, order, topScore
		}
	}
	return texts, nil, 0
}
