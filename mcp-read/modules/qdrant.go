package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/telegram-ai-assistant/root/pkg/logging"
)

const maxChainsForFullContext = 50

var logQdrant = logging.New("mcp-read.qdrant")

type QdrantClient struct {
	QdrantURL string
}

func NewQdrantClient(qdrantURL string) *QdrantClient {
	return &QdrantClient{QdrantURL: qdrantURL}
}

func (c *QdrantClient) FetchChunkPayloadsByID(ctx context.Context, collectionName string, chunkIDSet map[string]struct{}) []ChunkInfo {
	if len(chunkIDSet) == 0 {
		return nil
	}
	ids := make([]string, 0, len(chunkIDSet))
	for id := range chunkIDSet {
		ids = append(ids, id)
	}
	limit := uint32(100)
	withPayload := true
	body := QdrantScrollReq{
		Filter: map[string]interface{}{
			"should": []map[string]interface{}{
				{"key": "chunk_id", "match": map[string]interface{}{"any": ids}},
			},
		},
		Limit:       &limit,
		WithPayload: &withPayload,
	}
	payloadBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.QdrantURL+"/collections/"+collectionName+"/points/scroll", bytes.NewReader(payloadBytes))
	if err != nil {
		logQdrant.Warn(ctx, "scroll request build failed", logging.KV{"error", err})
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logQdrant.Warn(ctx, "scroll request failed", logging.KV{"error", err})
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logQdrant.Warn(ctx, "scroll non-200", logging.KV{"status", resp.StatusCode})
		return nil
	}
	var scrollRes QdrantScrollResp
	if err := json.NewDecoder(resp.Body).Decode(&scrollRes); err != nil {
		logQdrant.Warn(ctx, "scroll decode failed", logging.KV{"error", err})
		return nil
	}
	var out []ChunkInfo
	for _, pt := range scrollRes.Result.Points {
		chunk := payloadToChunkInfo(pt.Payload)
		if chunk.Text != "" || chunk.ChunkID != "" {
			out = append(out, chunk)
		}
	}
	return out
}

func (c *QdrantClient) GetChunkByID(ctx context.Context, collectionName, chunkID string) *ChunkInfo {
	if chunkID == "" {
		return nil
	}
	infos := c.FetchChunkPayloadsByID(ctx, collectionName, map[string]struct{}{chunkID: {}})
	if len(infos) == 0 {
		return nil
	}
	return &infos[0]
}

func (c *QdrantClient) GetFullDocumentForChunkID(ctx context.Context, collectionName, chunkID string) string {
	visited := make(map[string]struct{})
	var prevParts []string
	curID := chunkID
	for i := 0; i < maxChainsForFullContext && curID != ""; i++ {
		if _, ok := visited[curID]; ok {
			break
		}
		visited[curID] = struct{}{}
		chunk := c.GetChunkByID(ctx, collectionName, curID)
		if chunk == nil {
			break
		}
		prevParts = append([]string{strings.TrimSpace(chunk.Text)}, prevParts...)
		curID = chunk.PrevID
	}
	chunk := c.GetChunkByID(ctx, collectionName, chunkID)
	if chunk == nil {
		return strings.Join(prevParts, "\n\n")
	}
	parts := prevParts
	curID = chunk.NextID
	for i := 0; i < maxChainsForFullContext && curID != ""; i++ {
		if _, ok := visited[curID]; ok {
			break
		}
		visited[curID] = struct{}{}
		nextC := c.GetChunkByID(ctx, collectionName, curID)
		if nextC == nil {
			break
		}
		parts = append(parts, strings.TrimSpace(nextC.Text))
		curID = nextC.NextID
	}
	return strings.Join(parts, "\n\n")
}

func (c *QdrantClient) ExpandChunkIDsToFullContext(ctx context.Context, collectionName string, chunkIDs []string) string {
	if len(chunkIDs) == 0 || len(chunkIDs) > 2 {
		return ""
	}
	var parts []string
	seen := make(map[string]struct{})
	for _, id := range chunkIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		doc := c.GetFullDocumentForChunkID(ctx, collectionName, id)
		if doc != "" {
			parts = append(parts, doc)
		}
	}
	return strings.Join(parts, "\n\n")
}

// EnsureFullTextIndex создаёт payload-индекс типа "text" с токенизатором "multilingual" для поля content.
// Multilingual даёт лемматизацию (яблоко ↔ яблоки и т.п.). Идемпотентно.
func (c *QdrantClient) EnsureFullTextIndex(ctx context.Context, collectionName string) {
	body := []byte(`{"field_name":"content","field_schema":{"type":"text","tokenizer":"multilingual","min_token_len":1,"max_token_len":50,"lowercase":true}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.QdrantURL+"/collections/"+collectionName+"/index", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
	// 200 = создан, 409 = уже есть — оба ок
}

// ScrollWithFullTextFilter возвращает чанки по полнотекстовому совпадению по полю content.
func (c *QdrantClient) ScrollWithFullTextFilter(ctx context.Context, collectionName, queryText string, limit uint32) ([]ChunkInfo, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, nil
	}
	c.EnsureFullTextIndex(ctx, collectionName)
	withPayload := true
	body := QdrantScrollReq{
		Filter: map[string]interface{}{
			"must": []map[string]interface{}{
				{"key": "content", "match": map[string]interface{}{"text": queryText}},
			},
		},
		Limit:       &limit,
		WithPayload: &withPayload,
	}
	payloadBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.QdrantURL+"/collections/"+collectionName+"/points/scroll", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var scrollRes QdrantScrollResp
	if err := json.NewDecoder(resp.Body).Decode(&scrollRes); err != nil {
		return nil, err
	}
	var out []ChunkInfo
	for _, pt := range scrollRes.Result.Points {
		chunk := payloadToChunkInfo(pt.Payload)
		if chunk.Text != "" || chunk.ChunkID != "" {
			out = append(out, chunk)
		}
	}
	return out, nil
}

func (c *QdrantClient) SearchPoints(ctx context.Context, collectionName string, vec []float32, limit uint64) ([]ChunkInfo, error) {
	trueVal := true
	body := QdrantSearchReq{Vector: vec, Limit: limit, WithPayload: &trueVal}
	payloadBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.QdrantURL+"/collections/"+collectionName+"/points/search", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 404 {
			return nil, nil
		}
		return nil, err
	}
	var searchRes QdrantSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		return nil, err
	}
	var out []ChunkInfo
	for _, p := range searchRes.Result {
		chunk := payloadToChunkInfo(p.Payload)
		if chunk.Text != "" || chunk.ChunkID != "" {
			out = append(out, chunk)
		}
	}
	return out, nil
}

func (c *QdrantClient) ScrollAllChunksContaining(ctx context.Context, collectionName, queryText string) []ChunkInfo {
	queryLower := strings.ToLower(strings.TrimSpace(queryText))
	words := strings.Fields(queryLower)
	if len(words) == 0 {
		return nil
	}
	wordsRequired := FilterSignificantWords(words)

	var result []ChunkInfo
	var offset interface{}
	pageSize := uint32(200)
	withPayload := true
	maxPages := 100

	for page := 0; page < maxPages; page++ {
		body := QdrantScrollReq{
			Limit:       &pageSize,
			WithPayload: &withPayload,
		}
		if offset != nil {
			body.Offset = offset
		}
		payloadBytes, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.QdrantURL+"/collections/"+collectionName+"/points/scroll", bytes.NewReader(payloadBytes))
		if err != nil {
			break
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			break
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			break
		}
		var scrollRes QdrantScrollResp
		if err := json.NewDecoder(resp.Body).Decode(&scrollRes); err != nil {
			resp.Body.Close()
			break
		}
		resp.Body.Close()

		for _, pt := range scrollRes.Result.Points {
			chunk := payloadToChunkInfo(pt.Payload)
			chunkLower := strings.ToLower(chunk.Text + " " + chunk.SearchableText + " " + chunk.Name)
			allFound := true
			for _, w := range wordsRequired {
				if !chunkContainsQueryWord(chunkLower, w) {
					allFound = false
					break
				}
			}
			if allFound && (chunk.Text != "" || chunk.ChunkID != "") {
				result = append(result, chunk)
			}
		}

		offset = scrollRes.Result.NextPageOffset
		if offset == nil {
			break
		}
	}

	logQdrant.Info(ctx, "ScrollAllChunksContaining: done", logging.KV{"query", queryText}, logging.KV{"found", len(result)})
	return result
}

const MaxScrollPagesForNames = 50
const ScrollPageSize = 200

func (c *QdrantClient) ScrollAllPages(ctx context.Context, collectionName string, maxPages int, pageSize uint32) ([]ChunkInfo, error) {
	if maxPages <= 0 {
		maxPages = MaxScrollPagesForNames
	}
	if pageSize == 0 {
		pageSize = ScrollPageSize
	}
	withPayload := true
	var all []ChunkInfo
	var offset interface{}
	for page := 0; page < maxPages; page++ {
		body := QdrantScrollReq{Limit: &pageSize, WithPayload: &withPayload}
		if offset != nil {
			body.Offset = offset
		}
		payloadBytes, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.QdrantURL+"/collections/"+collectionName+"/points/scroll", bytes.NewReader(payloadBytes))
		if err != nil {
			return all, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return all, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return all, err
		}
		var scrollRes QdrantScrollResp
		if err := json.NewDecoder(resp.Body).Decode(&scrollRes); err != nil {
			resp.Body.Close()
			return all, err
		}
		resp.Body.Close()
		for _, pt := range scrollRes.Result.Points {
			chunk := payloadToChunkInfo(pt.Payload)
			if chunk.Text != "" || chunk.ChunkID != "" {
				all = append(all, chunk)
			}
		}
		offset = scrollRes.Result.NextPageOffset
		if offset == nil {
			break
		}
	}
	return all, nil
}

// chunkContainsQueryWord проверяет, что искомое слово w встречается в тексте как отдельное слово,
// а не внутри другого (например, «ангел» не совпадает с «архангел», «ангельский»).
func chunkContainsQueryWord(chunkLower, w string) bool {
	return w != "" && containsWordRu(chunkLower, w)
}
