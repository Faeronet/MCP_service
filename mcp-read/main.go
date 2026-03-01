package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
	"github.com/telegram-ai-assistant/root/pkg/ratelimit"

	"github.com/redis/go-redis/v9"
)

var log = logging.New("mcp-read")

const retrievalCacheTTL = 60 * time.Second

func main() {
	ctx := context.Background()
	qdrantURL := strings.TrimSuffix(config.LoadString("QDRANT_URL", "http://qdrant:6333"), "/")
	redisAddr := config.LoadString("REDIS_ADDR", "redis:6379")
	embedAPIBase := config.LoadString("EMBEDDING_BINDING_HOST", "")
	if embedAPIBase == "" {
		embedAPIBase = config.LoadString("EMBED_API_URL", "")
	}
	if embedAPIBase == "" {
		embedAPIBase = strings.TrimSuffix(config.LoadString("VLLM_OPENAI_BASE", "http://vllm:8000/v1"), "/")
	} else {
		embedAPIBase = strings.TrimSuffix(embedAPIBase, "/")
	}
	embedAPIKey := config.LoadString("EMBEDDING_BINDING_API_KEY", "")
	embedModel := config.LoadString("EMBEDDING_MODEL", "BAAI/bge-m3")
	rerankAPIURL := config.LoadString("RERANK_BINDING_HOST", "")
	if rerankAPIURL == "" {
		rerankAPIURL = config.LoadString("RERANK_API_URL", "")
	}
	rerankAPIURL = strings.TrimSuffix(rerankAPIURL, "/")
	if rerankAPIURL == "" {
		rerankAPIURL = "http://rerank:8787/api/v1"
		log.Info(ctx, "rerank URL not set, using default", logging.KV{"url", rerankAPIURL})
	} else {
		log.Info(ctx, "rerank configured", logging.KV{"url", rerankAPIURL})
	}
	rerankAPIKey := config.LoadString("RERANK_BINDING_API_KEY", "")
	rerankModel := config.LoadString("RERANK_MODEL", "BAAI/bge-reranker-v2-m3")
	maxRerank := config.LoadInt("MAX_INFLIGHT_RERANK", 16)
	maxEmbed := config.LoadInt("MAX_INFLIGHT_EMBED", 32)

	var rdb *redis.Client
	if redisAddr != "" {
		rdb = redis.NewClient(&redis.Options{Addr: redisAddr})
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Warn(ctx, "redis not available, using in-memory cache", logging.KV{"error", err})
			rdb = nil
		}
	}

	rerankLimiter := ratelimit.NewInFlight(maxRerank)
	embedLimiter := ratelimit.NewInFlight(maxEmbed)

	handler := &MCPReadHandler{
		qdrantURL:      qdrantURL,
		embedAPIBase:   embedAPIBase,
		embedAPIKey:    embedAPIKey,
		rerankAPIURL:   rerankAPIURL,
		rerankAPIKey:   rerankAPIKey,
		redis:          rdb,
		embedModel:     embedModel,
		rerankModel:    rerankModel,
		rerankLimiter:  rerankLimiter,
		embedLimiter:   embedLimiter,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/mcp/build_context", requestIDMiddleware(handler.BuildContext))

	srv := &http.Server{Addr: ":8082", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Info(ctx, "mcp-read listening on :8082")
	_ = srv.ListenAndServe()
}

func requestIDMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		ctx := logging.WithRequestID(r.Context(), reqID)
		w.Header().Set("X-Request-ID", reqID)
		next(w, r.WithContext(ctx))
	}
}

type MCPReadHandler struct {
	qdrantURL         string
	embedAPIBase      string
	embedAPIKey       string
	rerankAPIURL      string
	rerankAPIKey      string
	redis             *redis.Client
	embedModel        string
	rerankModel       string
	rerankLimiter     *ratelimit.InFlight
	embedLimiter      *ratelimit.InFlight
	embedModelIDMu    sync.Mutex
	embedModelIDCached string
}

type BuildContextRequest struct {
	QueryText       string `json:"query_text"`
	ACLToken        string `json:"acl_token"`
	TokenBudget     int    `json:"token_budget"`
	Mode            string `json:"mode"`
	AttachmentsText string `json:"attachments_text"`
}

type BuildContextResponse struct {
	Context string `json:"context"`
	Error   string `json:"error,omitempty"`
}

type qdrantSearchReq struct {
	Vector   []float32 `json:"vector"`
	Limit    uint64    `json:"limit"`
	WithPayload *bool   `json:"with_payload,omitempty"`
}

type qdrantSearchResult struct {
	Result []struct {
		Payload map[string]interface{} `json:"payload"`
	} `json:"result"`
}

// scrollReq/scrollResp для запроса соседних чанков по chunk_id (prev/next)
type qdrantScrollReq struct {
	Filter      map[string]interface{} `json:"filter,omitempty"`
	Limit       *uint32                `json:"limit,omitempty"`
	WithPayload *bool                  `json:"with_payload,omitempty"`
}
type qdrantScrollResp struct {
	Result struct {
		Points []struct {
			Payload map[string]interface{} `json:"payload"`
		} `json:"points"`
	} `json:"result"`
}

// OpenAI-compatible embeddings request/response
type embedReq struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	EncodingFormat string `json:"encoding_format,omitempty"`
}
type embedResp struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}
type modelsResp struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// embedModelID возвращает id модели с сервера (GET /v1/models), чтобы избежать 404 из-за несовпадения имени.
func (h *MCPReadHandler) embedModelID(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.embedAPIBase+"/models", nil)
	if err != nil {
		return h.embedModel
	}
	if h.embedAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.embedAPIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return h.embedModel
	}
	defer resp.Body.Close()
	var out modelsResp
	if json.NewDecoder(resp.Body).Decode(&out) != nil || len(out.Data) == 0 {
		return h.embedModel
	}
	return out.Data[0].ID
}

// embedQuery возвращает вектор запроса через vLLM /v1/embeddings; при ошибке или если модель не задана — нулевой вектор.
func (h *MCPReadHandler) embedQuery(ctx context.Context, query string) []float32 {
	fallbackDim := config.LoadInt("EMBEDDING_DIMENSION", 1024)
	if query == "" || h.embedModel == "" {
		return make([]float32, fallbackDim)
	}
	modelID := h.embedModelID(ctx)
	body := embedReq{Model: modelID, Input: query, EncodingFormat: "float"}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.embedAPIBase+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		log.Warn(ctx, "embed request build", logging.KV{"error", err})
		return make([]float32, fallbackDim)
	}
	req.Header.Set("Content-Type", "application/json")
	if h.embedAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.embedAPIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warn(ctx, "embed request", logging.KV{"error", err})
		return make([]float32, fallbackDim)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Warn(ctx, "embed non-200", logging.KV{"status", resp.StatusCode})
		return make([]float32, fallbackDim)
	}
	var emb embedResp
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

func (h *MCPReadHandler) BuildContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	var req BuildContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.TokenBudget <= 0 {
		req.TokenBudget = 4000
	}

	normalized := normalizeQuery(req.QueryText)
	cacheKey := "retrieval:" + normalized + ":v1"
	if h.redis != nil {
		val, err := h.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: val})
			return
		}
	}

	if err := h.embedLimiter.Acquire(ctx); err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: ""})
		return
	}
	defer h.embedLimiter.Release()

	collectionName := "chunks"
	// Вектор запроса через vLLM /v1/embeddings; при ошибке — нулевой вектор (поиск всё равно выполнится)
	vec := h.embedQuery(ctx, req.QueryText)
	trueVal := true
	body := qdrantSearchReq{Vector: vec, Limit: 20, WithPayload: &trueVal}
	payload, _ := json.Marshal(body)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, h.qdrantURL+"/collections/"+collectionName+"/points/search", bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Warn(ctx, "qdrant request failed, returning empty context", logging.KV{"error", err})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: ""})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Warn(ctx, "qdrant non-200, returning empty context", logging.KV{"status", resp.StatusCode})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: ""})
		return
	}
	var searchRes qdrantSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		log.Warn(ctx, "qdrant decode failed, returning empty context", logging.KV{"error", err})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: ""})
		return
	}

	var texts []string
	mainChunkIDs := make(map[string]struct{})
	neighborIDs := make(map[string]struct{})
	for _, p := range searchRes.Result {
		if t, ok := p.Payload["text"].(string); ok {
			texts = append(texts, t)
		}
		if cid, ok := p.Payload["chunk_id"].(string); ok && cid != "" {
			mainChunkIDs[cid] = struct{}{}
		}
		for _, key := range []string{"prev_chunk_id", "next_chunk_id"} {
			if v, ok := p.Payload[key].(string); ok && v != "" {
				neighborIDs[v] = struct{}{}
			}
		}
	}
	// Убираем из соседей те, что уже в результатах поиска
	for id := range mainChunkIDs {
		delete(neighborIDs, id)
	}
	// Подтягиваем тексты соседних чанков (связи prev/next)
	if len(neighborIDs) > 0 {
		neighborTexts := h.fetchChunksByID(ctx, collectionName, neighborIDs)
		if len(neighborTexts) > 0 {
			texts = append(texts, neighborTexts...)
			log.Info(ctx, "neighbor chunks added to context", logging.KV{"count", len(neighborTexts)}, logging.KV{"requested", len(neighborIDs)})
		}
	}
	if len(texts) > 0 && h.rerankAPIURL != "" && h.rerankModel != "" {
		if err := h.rerankLimiter.Acquire(ctx); err == nil {
			texts = h.rerank(ctx, req.QueryText, texts)
			h.rerankLimiter.Release()
		}
	} else if len(texts) > 0 && (h.rerankAPIURL == "" || h.rerankModel == "") {
		log.Info(ctx, "rerank skipped (no url or model)", logging.KV{"rerank_url_set", h.rerankAPIURL != ""})
	}
	contextText := strings.Join(texts, "\n\n")
	if len(contextText) > req.TokenBudget*4 {
		contextText = contextText[:req.TokenBudget*4]
	}
	if req.AttachmentsText != "" {
		attach := strings.TrimSpace(req.AttachmentsText)
		if len(attach) > req.TokenBudget*2 {
			attach = attach[:req.TokenBudget*2]
		}
		contextText = attach + "\n\n" + contextText
	}

	if h.redis != nil {
		_ = h.redis.Set(ctx, cacheKey, contextText, retrievalCacheTTL).Err()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: contextText})
}

// fetchChunksByID возвращает тексты чанков по списку chunk_id (для связей prev/next).
func (h *MCPReadHandler) fetchChunksByID(ctx context.Context, collectionName string, chunkIDSet map[string]struct{}) []string {
	if len(chunkIDSet) == 0 {
		return nil
	}
	ids := make([]string, 0, len(chunkIDSet))
	for id := range chunkIDSet {
		ids = append(ids, id)
	}
	// Qdrant scroll: filter chunk_id in ids
	limit := uint32(100)
	withPayload := true
	body := qdrantScrollReq{
		Filter: map[string]interface{}{
			"should": []map[string]interface{}{
				{"key": "chunk_id", "match": map[string]interface{}{"any": ids}},
			},
		},
		Limit:       &limit,
		WithPayload: &withPayload,
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.qdrantURL+"/collections/"+collectionName+"/points/scroll", bytes.NewReader(payload))
	if err != nil {
		log.Warn(ctx, "scroll request build failed", logging.KV{"error", err})
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warn(ctx, "scroll request failed", logging.KV{"error", err})
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Warn(ctx, "scroll non-200", logging.KV{"status", resp.StatusCode})
		return nil
	}
	var scrollRes qdrantScrollResp
	if err := json.NewDecoder(resp.Body).Decode(&scrollRes); err != nil {
		log.Warn(ctx, "scroll decode failed", logging.KV{"error", err})
		return nil
	}
	var out []string
	for _, pt := range scrollRes.Result.Points {
		if t, ok := pt.Payload["text"].(string); ok && t != "" {
			out = append(out, t)
		}
	}
	return out
}

func normalizeQuery(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

// rerank вызывает RERANK_API_URL и переупорядочивает texts по relevance_score. При ошибке возвращает texts без изменений.
// Поддерживает форматы: (1) documents[] + results/scores, (2) documents[{id,text}] + data[{id,similarity}] (s-kostyaev/reranker).
func (h *MCPReadHandler) rerank(ctx context.Context, query string, texts []string) []string {
	if len(texts) == 0 {
		return texts
	}
	// Формат reranker API: documents как [{id, text}], ответ {data: [{id, similarity}]}
	docsWithID := make([]map[string]interface{}, len(texts))
	for i, t := range texts {
		docsWithID[i] = map[string]interface{}{"id": i, "text": t}
	}
	body := map[string]interface{}{
		"query":     query,
		"documents": docsWithID,
		"model":     h.rerankModel,
	}
	payload, _ := json.Marshal(body)
	rerankURL := h.rerankAPIURL
	if rerankURL != "" {
		rerankURL = strings.TrimSuffix(rerankURL, "/")
		if !strings.Contains(rerankURL, "/rerank") {
			if strings.HasSuffix(rerankURL, "/api/v1") {
				rerankURL = rerankURL + "/rerank"
			} else {
				rerankURL = rerankURL + "/api/v1/rerank"
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rerankURL, bytes.NewReader(payload))
	if err != nil {
		log.Warn(ctx, "rerank request build", logging.KV{"error", err})
		return texts
	}
	req.Header.Set("Content-Type", "application/json")
	if h.rerankAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.rerankAPIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warn(ctx, "rerank request failed", logging.KV{"error", err}, logging.KV{"url", rerankURL})
		return texts
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Warn(ctx, "rerank non-200", logging.KV{"status", resp.StatusCode}, logging.KV{"body", string(body)})
		return texts
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
		log.Warn(ctx, "rerank decode failed", logging.KV{"error", err})
		return texts
	}
	// Формат data[] (reranker): уже отсортировано по similarity desc
	if len(result.Data) > 0 {
		log.Info(ctx, "rerank applied", logging.KV{"docs", len(texts)}, logging.KV{"url", rerankURL})
		out := make([]string, 0, len(result.Data))
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
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	// Формат 1: scores[i] = релевантность i-го документа
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
		out := make([]string, len(texts))
		for i, p := range ps {
			out[i] = texts[p.i]
		}
		return out
	}
	// Формат 2: results[] с index и relevance_score
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
		out := make([]string, 0, len(texts))
		for _, p := range ps {
			if p.i >= 0 && p.i < len(texts) {
				out = append(out, texts[p.i])
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return texts
}
