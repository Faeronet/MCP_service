package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
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
	embedModel := config.LoadString("EMBEDDING_MODEL", "")
	rerankModel := config.LoadString("RERANK_MODEL", "")
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
		qdrantURL:     qdrantURL,
		redis:         rdb,
		embedModel:    embedModel,
		rerankModel:   rerankModel,
		rerankLimiter: rerankLimiter,
		embedLimiter:  embedLimiter,
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
	qdrantURL     string
	redis         *redis.Client
	embedModel    string
	rerankModel   string
	rerankLimiter *ratelimit.InFlight
	embedLimiter  *ratelimit.InFlight
}

type BuildContextRequest struct {
	QueryText       string `json:"query_text"`
	ACLToken        string `json:"acl_token"`
	TokenBudget     int    `json:"token_budget"`
	Mode            string `json:"mode"`
	AttachmentsText string `json:"attachments_text_optional"`
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
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Error: "embed rate limit"})
		return
	}
	defer h.embedLimiter.Release()

	// When embed model not set: return empty context (graceful)
	if h.embedModel == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: ""})
		return
	}

	collectionName := "chunks"
	// Dummy vector for MVP (replace with real embed API call)
	vec := make([]float32, 384)
	trueVal := true
	body := qdrantSearchReq{Vector: vec, Limit: 20, WithPayload: &trueVal}
	payload, _ := json.Marshal(body)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, h.qdrantURL+"/collections/"+collectionName+"/points/search", bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Error(ctx, "qdrant request", logging.KV{"error", err})
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Error: "QDRANT_UNAVAILABLE"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Error: "COLLECTION_NOT_READY"})
		return
	}
	var searchRes qdrantSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Error: "QDRANT_QUERY_FAILED"})
		return
	}

	var texts []string
	for _, p := range searchRes.Result {
		if t, ok := p.Payload["text"].(string); ok {
			texts = append(texts, t)
		}
	}
	contextText := strings.Join(texts, "\n\n")
	if len(contextText) > req.TokenBudget*4 {
		contextText = contextText[:req.TokenBudget*4]
	}

	if h.redis != nil {
		_ = h.redis.Set(ctx, cacheKey, contextText, retrievalCacheTTL).Err()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: contextText})
}

func normalizeQuery(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}
