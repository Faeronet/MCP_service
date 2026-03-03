package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
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
const maxSearchRounds = 5

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
	rerankMinScore := 0.8
	if s := config.LoadString("RERANK_MIN_SCORE", "0.8"); s != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil && f >= 0 && f <= 1 {
			rerankMinScore = f
		}
	}
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
		qdrantURL:       qdrantURL,
		embedAPIBase:    embedAPIBase,
		embedAPIKey:     embedAPIKey,
		rerankAPIURL:    rerankAPIURL,
		rerankAPIKey:    rerankAPIKey,
		rerankMinScore:  rerankMinScore,
		redis:           rdb,
		embedModel:      embedModel,
		rerankModel:     rerankModel,
		rerankLimiter:   rerankLimiter,
		embedLimiter:    embedLimiter,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/mcp/build_context", requestIDMiddleware(handler.BuildContext))
	mux.HandleFunc("/mcp/all_names", requestIDMiddleware(handler.AllNames))

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
	qdrantURL          string
	embedAPIBase       string
	embedAPIKey        string
	rerankAPIURL       string
	rerankAPIKey       string
	rerankMinScore     float64
	redis              *redis.Client
	embedModel         string
	rerankModel        string
	rerankLimiter      *ratelimit.InFlight
	embedLimiter       *ratelimit.InFlight
	embedModelIDMu     sync.Mutex
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

// chunkInfo — чанк с именем и связями для фильтра по дате и форматирования контекста.
type chunkInfo struct {
	Name       string
	Text       string
	ChunkID    string
	RelatedIDs []string
	PrevID     string
	NextID     string
}

// Форматы: "24 марта", "24 января", "24.03", "24.03.2025"
var reDateMonthRu = regexp.MustCompile(`(\d{1,2})\s+(января|февраля|марта|апреля|мая|июня|июля|августа|сентября|октября|ноября|декабря)`)
var reDateDot = regexp.MustCompile(`(\d{1,2})\.(\d{1,2})(?:\.(\d{2,4}))?`)

func extractDateFromQuery(query string) (dateStr string, ok bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return "", false
	}
	if m := reDateMonthRu.FindString(q); m != "" {
		return m, true // "24 марта", "24 января" и т.д.
	}
	if m := reDateDot.FindString(q); m != "" {
		return m, true // "24.03" or "24.03.2025"
	}
	return "", false
}

// queryDayLessThan10 возвращает true, если в dateStr день месяца < 10 (например "5 марта", "09.03").
func queryDayLessThan10(dateStr string) bool {
	day := parseDayFromDateStr(dateStr)
	return day > 0 && day < 10
}

// parseDayFromDateStr извлекает день (1–31) из строки даты "5 марта" или "24.03.2025". 0 если не распознано.
func parseDayFromDateStr(s string) int {
	s = strings.TrimSpace(s)
	if m := reDateMonthRu.FindStringSubmatch(s); len(m) >= 2 {
		if d, err := strconv.Atoi(m[1]); err == nil && d >= 1 && d <= 31 {
			return d
		}
	}
	if m := reDateDot.FindStringSubmatch(s); len(m) >= 2 {
		if d, err := strconv.Atoi(m[1]); err == nil && d >= 1 && d <= 31 {
			return d
		}
	}
	return 0
}

// chunkHasDateWithDayLessThan10 проверяет, есть ли в тексте чанка дата с днём < 10.
func chunkHasDateWithDayLessThan10(text string) bool {
	// Проверяем формат "N месяца"
	for _, sub := range reDateMonthRu.FindAllStringSubmatch(text, -1) {
		if len(sub) >= 2 {
			if d, err := strconv.Atoi(sub[1]); err == nil && d >= 1 && d < 10 {
				return true
			}
		}
	}
	// Проверяем формат N.M или N.M.YYYY (день — первое число)
	for _, sub := range reDateDot.FindAllStringSubmatch(text, -1) {
		if len(sub) >= 2 {
			if d, err := strconv.Atoi(sub[1]); err == nil && d >= 1 && d < 10 {
				return true
			}
		}
	}
	return false
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

// scrollReq/scrollResp для запроса соседних чанков по chunk_id (prev/next) и для AllNames.
type qdrantScrollReq struct {
	Filter      map[string]interface{} `json:"filter,omitempty"`
	Limit       *uint32                `json:"limit,omitempty"`
	WithPayload *bool                  `json:"with_payload,omitempty"`
	Offset      interface{}            `json:"offset,omitempty"`
}
type qdrantScrollResp struct {
	Result struct {
		Points         []struct {
			Payload map[string]interface{} `json:"payload"`
		} `json:"points"`
		NextPageOffset interface{} `json:"next_page_offset"`
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
	cacheKey := "retrieval:" + normalized + ":v2"
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
	vec := h.embedQuery(ctx, req.QueryText)
	trueVal := true
	dateStr, hasDate := extractDateFromQuery(req.QueryText)
	if hasDate {
		log.Info(ctx, "build_context: date in query", logging.KV{"date", dateStr})
	}

	var contextText string
	var found bool
	for round := 1; round <= maxSearchRounds; round++ {
		limit := uint64(20 * round)
		if limit > 100 {
			limit = 100
		}
		body := qdrantSearchReq{Vector: vec, Limit: limit, WithPayload: &trueVal}
		payload, _ := json.Marshal(body)
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, h.qdrantURL+"/collections/"+collectionName+"/points/search", bytes.NewReader(payload))
		httpReq.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			log.Warn(ctx, "qdrant request failed", logging.KV{"error", err}, logging.KV{"round", round})
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Warn(ctx, "qdrant non-200", logging.KV{"status", resp.StatusCode}, logging.KV{"round", round})
			continue
		}
		var searchRes qdrantSearchResult
		if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
			resp.Body.Close()
			log.Warn(ctx, "qdrant decode failed", logging.KV{"error", err})
			continue
		}
		resp.Body.Close()

		var items []chunkInfo
		mainChunkIDs := make(map[string]struct{})
		neighborIDs := make(map[string]struct{})
		for _, p := range searchRes.Result {
			c := payloadToChunkInfo(p.Payload)
			if c.Text != "" || c.ChunkID != "" {
				items = append(items, c)
			}
			if c.ChunkID != "" {
				mainChunkIDs[c.ChunkID] = struct{}{}
			}
			if c.PrevID != "" {
				neighborIDs[c.PrevID] = struct{}{}
			}
			if c.NextID != "" {
				neighborIDs[c.NextID] = struct{}{}
			}
		}
		for id := range mainChunkIDs {
			delete(neighborIDs, id)
		}
		if len(neighborIDs) > 0 {
			neighbors := h.fetchChunkPayloadsByID(ctx, collectionName, neighborIDs)
			items = append(items, neighbors...)
		}
		if len(items) == 0 {
			continue
		}

		texts := make([]string, len(items))
		for i := range items {
			texts[i] = items[i].Text
		}

		var topScore float64
		var order []int
		if h.rerankAPIURL != "" && h.rerankModel != "" {
			if err := h.rerankLimiter.Acquire(ctx); err != nil {
				continue
			}
			_, order, topScore = h.rerankWithScoreAndOrder(ctx, req.QueryText, texts)
			h.rerankLimiter.Release()
			log.Info(ctx, "build_context: round rerank", logging.KV{"round", round}, logging.KV{"top_score", topScore}, logging.KV{"docs", len(texts)})
			if topScore < h.rerankMinScore {
				continue
			}
			if order != nil && len(order) == len(items) {
				ordered := make([]chunkInfo, len(items))
				for i, idx := range order {
					ordered[i] = items[idx]
				}
				items = ordered
			}
		} else {
			topScore = 1.0
		}

		if hasDate {
			// Найти чанки, в тексте которых есть дата из запроса
			var withDate []chunkInfo
			for i := range items {
				if strings.Contains(items[i].Text, dateStr) {
					withDate = append(withDate, items[i])
				}
			}
			// Если в запросе день < 10, оставляем только чанки, где дата в тексте тоже с днём < 10
			if len(withDate) > 0 && queryDayLessThan10(dateStr) {
				var filtered []chunkInfo
				for _, c := range withDate {
					if chunkHasDateWithDayLessThan10(c.Text) {
						filtered = append(filtered, c)
					}
				}
				withDate = filtered
			}
			if len(withDate) == 0 {
				log.Info(ctx, "build_context: date not in chunks or day filter", logging.KV{"date", dateStr}, logging.KV{"round", round})
				continue
			}
			// Собрать ID: чанки с датой + их связи (related, prev, next)
			linkSet := make(map[string]struct{})
			for _, c := range withDate {
				linkSet[c.ChunkID] = struct{}{}
				for _, id := range c.RelatedIDs {
					linkSet[id] = struct{}{}
				}
				if c.PrevID != "" {
					linkSet[c.PrevID] = struct{}{}
				}
				if c.NextID != "" {
					linkSet[c.NextID] = struct{}{}
				}
			}
			// Подгрузить все связанные чанки по linkSet (в items могут быть только prev/next)
			linked := h.fetchChunkPayloadsByID(ctx, collectionName, linkSet)
			items = linked
		}

		var b strings.Builder
		for _, c := range items {
			if c.Name != "" {
				b.WriteString("Имя: ")
				b.WriteString(c.Name)
				b.WriteString("\n")
			}
			b.WriteString(c.Text)
			b.WriteString("\n\n")
		}
		contextText = b.String()
		if len(contextText) > req.TokenBudget*4 {
			contextText = contextText[:req.TokenBudget*4]
		}
		found = true
		break
	}

	if !found {
		if hasDate {
			log.Info(ctx, "build_context: date not found after rounds", logging.KV{"rounds", maxSearchRounds}, logging.KV{"date", dateStr})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: "", Error: "date_not_found"})
			return
		}
		log.Info(ctx, "build_context: chunk not found after rounds", logging.KV{"rounds", maxSearchRounds})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: "", Error: "chunk_not_found"})
		return
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

const maxScrollPagesForNames = 50
const scrollPageSize = 200

// AllNames возвращает контекст: все уникальные name из chunks + содержимое (текст) каждого чанка для запроса "[name] all".
func (h *MCPReadHandler) AllNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	collectionName := "chunks"
	// name -> текст чанка (первое вхождение по имени, без повторов)
	nameToContent := make(map[string]string)
	var offset interface{}
	pageLimit := uint32(scrollPageSize)
	withPayload := true
	for page := 0; page < maxScrollPagesForNames; page++ {
		body := qdrantScrollReq{Limit: &pageLimit, WithPayload: &withPayload}
		if offset != nil {
			body.Offset = offset
		}
		payload, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.qdrantURL+"/collections/"+collectionName+"/points/scroll", bytes.NewReader(payload))
		if err != nil {
			log.Warn(ctx, "all_names scroll build", logging.KV{"error", err})
			break
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Warn(ctx, "all_names scroll request", logging.KV{"error", err})
			break
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Warn(ctx, "all_names scroll non-200", logging.KV{"status", resp.StatusCode})
			break
		}
		var scrollRes qdrantScrollResp
		if err := json.NewDecoder(resp.Body).Decode(&scrollRes); err != nil {
			resp.Body.Close()
			log.Warn(ctx, "all_names scroll decode", logging.KV{"error", err})
			break
		}
		resp.Body.Close()
		for _, pt := range scrollRes.Result.Points {
			n, okName := pt.Payload["name"].(string)
			if !okName || strings.TrimSpace(n) == "" {
				continue
			}
			n = strings.TrimSpace(n)
			if _, seen := nameToContent[n]; seen {
				continue
			}
			text, _ := pt.Payload["text"].(string)
			nameToContent[n] = strings.TrimSpace(text)
		}
		offset = scrollRes.Result.NextPageOffset
		if offset == nil {
			break
		}
	}
	names := make([]string, 0, len(nameToContent))
	for n := range nameToContent {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		b.WriteString("Имя: ")
		b.WriteString(n)
		b.WriteString("\n")
		if nameToContent[n] != "" {
			b.WriteString(nameToContent[n])
			b.WriteString("\n\n")
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: b.String()})
}

// fetchChunksByID возвращает тексты чанков по списку chunk_id (для обратной совместимости).
func (h *MCPReadHandler) fetchChunksByID(ctx context.Context, collectionName string, chunkIDSet map[string]struct{}) []string {
	infos := h.fetchChunkPayloadsByID(ctx, collectionName, chunkIDSet)
	out := make([]string, 0, len(infos))
	for _, c := range infos {
		if c.Text != "" {
			out = append(out, c.Text)
		}
	}
	return out
}

// fetchChunkPayloadsByID возвращает полные payload чанков по chunk_id (name, text, related_chunk_ids и т.д.).
func (h *MCPReadHandler) fetchChunkPayloadsByID(ctx context.Context, collectionName string, chunkIDSet map[string]struct{}) []chunkInfo {
	if len(chunkIDSet) == 0 {
		return nil
	}
	ids := make([]string, 0, len(chunkIDSet))
	for id := range chunkIDSet {
		ids = append(ids, id)
	}
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
	var out []chunkInfo
	for _, pt := range scrollRes.Result.Points {
		c := payloadToChunkInfo(pt.Payload)
		if c.Text != "" || c.ChunkID != "" {
			out = append(out, c)
		}
	}
	return out
}

func payloadToChunkInfo(p map[string]interface{}) chunkInfo {
	var c chunkInfo
	if t, ok := p["text"].(string); ok {
		c.Text = t
	}
	if n, ok := p["name"].(string); ok {
		c.Name = n
	}
	if id, ok := p["chunk_id"].(string); ok {
		c.ChunkID = id
	}
	if prev, ok := p["prev_chunk_id"].(string); ok {
		c.PrevID = prev
	}
	if next, ok := p["next_chunk_id"].(string); ok {
		c.NextID = next
	}
	if arr, ok := p["related_chunk_ids"].([]interface{}); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				c.RelatedIDs = append(c.RelatedIDs, s)
			}
		}
	}
	return c
}

func normalizeQuery(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

// rerankWithScoreAndOrder возвращает (reordered texts, order indices, topScore). order[i] = исходный индекс i-го по релевантности чанка.
func (h *MCPReadHandler) rerankWithScoreAndOrder(ctx context.Context, query string, texts []string) ([]string, []int, float64) {
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
		"model":     h.rerankModel,
	}
	payload, _ := json.Marshal(body)
	rerankURL := strings.TrimSuffix(h.rerankAPIURL, "/")
	if rerankURL != "" && !strings.HasSuffix(rerankURL, "/rerank") {
		if strings.HasSuffix(rerankURL, "/api/v1") {
			rerankURL = rerankURL + "/rerank"
		} else {
			rerankURL = rerankURL + "/api/v1/rerank"
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rerankURL, bytes.NewReader(payload))
	if err != nil {
		log.Warn(ctx, "rerank request build", logging.KV{"error", err})
		return texts, nil, 0
	}
	req.Header.Set("Content-Type", "application/json")
	if h.rerankAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.rerankAPIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warn(ctx, "rerank request failed", logging.KV{"error", err}, logging.KV{"url", rerankURL})
		return texts, nil, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Warn(ctx, "rerank non-200", logging.KV{"status", resp.StatusCode}, logging.KV{"body", string(body)})
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
		log.Warn(ctx, "rerank decode failed", logging.KV{"error", err})
		return texts, nil, 0
	}

	var topScore float64
	var order []int
	// Формат data[] (reranker): отсортировано по similarity desc
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
	// Формат scores[i]
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
	// Формат results[]
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
		out := make([]string, 0, len(texts))
		order = make([]int, 0, len(texts))
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

func (h *MCPReadHandler) rerankWithScore(ctx context.Context, query string, texts []string) ([]string, float64) {
	out, _, score := h.rerankWithScoreAndOrder(ctx, query, texts)
	return out, score
}

// rerank вызывает rerankWithScore и возвращает только переупорядоченные тексты (для обратной совместимости).
func (h *MCPReadHandler) rerank(ctx context.Context, query string, texts []string) []string {
	out, _ := h.rerankWithScore(ctx, query, texts)
	return out
}
