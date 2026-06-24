package modules

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logHandler = logging.New("mcp-read.handler")

// Server — HTTP-обработчик MCP read (build_context, full_context, all_names).
type Server struct {
	Config *Handler
	Embed  *EmbedClient
	Rerank *RerankClient
	Qdrant *QdrantClient
}

func normalizeQuery(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func RequestIDMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

func (s *Server) GetFullContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	var req GetFullContextRequest
	if r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&req)
	} else {
		req.ChunkID = r.URL.Query().Get("chunk_id")
		req.Collection = r.URL.Query().Get("collection")
	}
	chunkID := strings.TrimSpace(req.ChunkID)
	collection := strings.TrimSpace(req.Collection)
	if chunkID == "" || collection == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"context": "", "error": "chunk_id and collection required"})
		return
	}
	contextText := s.Qdrant.GetFullDocumentForChunkID(ctx, collection, chunkID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"context": contextText})
}

func (s *Server) AllNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	chunks, err := s.Qdrant.ScrollAllPages(ctx, "chunks", MaxScrollPagesForNames, ScrollPageSize)
	if err != nil {
		logHandler.Warn(ctx, "all_names scroll", logging.KV{"error", err})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(BuildContextResponse{Context: ""})
		return
	}
	nameToContent := make(map[string]string)
	for _, c := range chunks {
		n := strings.TrimSpace(c.Name)
		if n == "" {
			continue
		}
		if _, seen := nameToContent[n]; seen {
			continue
		}
		content := strings.TrimSpace(c.Text)
		if content == "" {
			nameToContent[n] = ""
		} else {
			nameToContent[n] = content
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

func (s *Server) BuildContext(w http.ResponseWriter, r *http.Request) {
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
	cacheKey := "retrieval:" + normalized + ":v5"
	if s.Config.Redis != nil {
		val, err := s.Config.Redis.Get(ctx, cacheKey).Result()
		if err == nil {
			var cached struct {
				Context             string   `json:"context"`
				ChunkIDs            []string `json:"chunk_ids"`
				ContextKind         string   `json:"context_kind"`
				ContextRef          string   `json:"context_ref"`
				SearchCollection    string   `json:"search_collection"`
				CollectionsSearched []string `json:"collections_searched"`
				QueryForFilter      string   `json:"query_for_filter"`
			}
			if json.Unmarshal([]byte(val), &cached) == nil {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(BuildContextResponse{
					Context: cached.Context, ChunkIDs: cached.ChunkIDs, ContextKind: cached.ContextKind, ContextRef: cached.ContextRef, SearchCollection: cached.SearchCollection, CollectionsSearched: cached.CollectionsSearched, QueryForFilter: cached.QueryForFilter,
				})
				return
			}
		}
	}

	if s.Embed != nil && s.Embed.Enabled() {
		if err := s.Config.EmbedLimiter.Acquire(ctx); err != nil {
			logHandler.Warn(ctx, "build_context: embed limiter full", logging.KV{"error", err})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(BuildContextResponse{
				Context: "",
				Error:   "embed_limit",
			})
			return
		}
		defer s.Config.EmbedLimiter.Release()
	}

	dateStr, hasDate := extractDateFromQuery(req.QueryText)
	queryForSearch := strings.TrimSpace(req.QueryText)
	triggerColl := CollectionForQuery(req.QueryText)
	// Если в запросе есть дата, но сработал триггер эмоц./интел./астральный дух — ищем в целевой коллекции, а не только в chunks
	if hasDate && (triggerColl == CollectionEmocionalnoe || triggerColl == CollectionIntellektualnye || triggerColl == CollectionAstralnyiDuh) {
		hasDate = false
	}

	// Коллекции emocionalnoe, intellektualnye, astralnyi_duh — только по триггерам (не в стандартном списке)
	defaultCollectionsOrder := []string{CollectionChunks, CollectionKachestvaEnergii, CollectionIskazheniyaEnergii, CollectionOther, CollectionSpecificnost, CollectionZnakZodiaka}
	var collectionsOrder []string
	var collectionsSearched []string
	if hasDate {
		collectionsOrder = []string{CollectionChunks}
		collectionsSearched = []string{CollectionChunks}
		logHandler.Info(ctx, "build_context: date in query, force chunks", logging.KV{"date", dateStr})
	} else {
		if triggerColl != "" && triggerColl != CollectionChunks {
			collectionsOrder = make([]string, 0, 1+len(defaultCollectionsOrder))
			collectionsOrder = append(collectionsOrder, triggerColl)
			for _, c := range defaultCollectionsOrder {
				if c != triggerColl {
					collectionsOrder = append(collectionsOrder, c)
				}
			}
			queryForSearch = StripRoutingKeywords(req.QueryText, triggerColl)
			logHandler.Info(ctx, "build_context: trigger, search first", logging.KV{"collection", triggerColl}, logging.KV{"query", req.QueryText}, logging.KV{"query_for_filter", queryForSearch})
		} else {
			collectionsOrder = defaultCollectionsOrder
		}
	}

	var vec []float32
	embedEnabled := s.Embed != nil && s.Embed.Enabled()
	if embedEnabled {
		if !hasDate {
			if triggerColl != "" && triggerColl != CollectionChunks {
				vec = s.Embed.EmbedQuery(ctx, strings.TrimSpace(req.QueryText))
			} else {
				vec = s.Embed.EmbedQuery(ctx, queryForSearch)
			}
		} else {
			vec = s.Embed.EmbedQuery(ctx, queryForSearch)
		}
	}

	var contextText string
	var chunkIDs []string
	var successCollection string
	var found bool
	var contextKind string
	var contextRef string

	if !hasDate {
		for _, col := range collectionsOrder {
			collectionsSearched = append(collectionsSearched, col)
		}
		type collResult struct {
			contextText string
			chunkIDs    []string
		}
		results := make(map[string]*collResult)
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, col := range collectionsOrder {
			wg.Add(1)
			go func(c string) {
				defer wg.Done()
				logHandler.Info(ctx, "build_context: trying collection (parallel)", logging.KV{"collection", c})
				ctxText, ids, ok := searchOneCollectionNoDate(ctx, s.Qdrant, s.Rerank, s.Config.RerankLimiter, c, vec, queryForSearch, req.TokenBudget, s.Config.RerankMinScore, s.Config.UseFullTextSearch || !embedEnabled, embedEnabled)
				if ok {
					mu.Lock()
					results[c] = &collResult{contextText: ctxText, chunkIDs: ids}
					mu.Unlock()
				}
			}(col)
		}
		wg.Wait()
		for _, col := range collectionsOrder {
			if r := results[col]; r != nil {
				contextText = r.contextText
				chunkIDs = r.chunkIDs
				successCollection = col
				found = true
				logHandler.Info(ctx, "build_context: using result by priority", logging.KV{"collection", col})
				break
			}
		}
		if found && len(chunkIDs) > 0 {
			skipFullContext := successCollection == CollectionEmocionalnoe || successCollection == CollectionIntellektualnye || successCollection == CollectionAstralnyiDuh
			if len(chunkIDs) == 1 && !skipFullContext {
				expanded := s.Qdrant.ExpandChunkIDsToFullContext(ctx, successCollection, chunkIDs)
				if expanded != "" {
					contextText = expanded
					contextKind = "full"
					contextRef = chunkIDs[0]
				} else {
					contextKind = "chunks"
					// contextText уже заполнен из results
				}
			} else {
				contextKind = "chunks"
				// contextText уже заполнен из results, не подставляем полный контекст
			}
		}
	} else {
		collectionsSearched = []string{CollectionChunks}
		items := s.Qdrant.ScrollAllChunksContaining(ctx, CollectionChunks, dateStr)
		if len(items) == 0 {
			if alt := dateStrToAlternateForm(dateStr); alt != "" {
				items = s.Qdrant.ScrollAllChunksContaining(ctx, CollectionChunks, alt)
			}
		}
		if len(items) > 0 {
			var withDate []ChunkInfo
			for _, c := range items {
				if chunkContainsDate(c.Text, dateStr) {
					withDate = append(withDate, c)
				}
			}
			if len(withDate) > 0 && queryDayLessThan10(dateStr) {
				filtered := withDate[:0]
				for _, c := range withDate {
					if chunkHasDateWithDayLessThan10(c.Text) {
						filtered = append(filtered, c)
					}
				}
				withDate = filtered
			}
			if len(withDate) > 0 {
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
				linked := s.Qdrant.FetchChunkPayloadsByID(ctx, CollectionChunks, linkSet)
				var b strings.Builder
				for _, c := range linked {
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
				chunkIDs = chunkIDsFromChunks(linked)
				successCollection = CollectionChunks
				found = true
			}
		}
	}

	if !found && !hasDate {
		queryTrim := strings.TrimSpace(queryForSearch)
		if queryTrim != "" {
			logHandler.Info(ctx, "build_context: vector search failed, falling back to scroll chunks", logging.KV{"query", queryTrim})
			items := s.Qdrant.ScrollAllChunksContaining(ctx, CollectionChunks, queryTrim)
			if len(items) > 0 {
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
				chunkIDs = chunkIDsFromChunks(items)
				successCollection = CollectionChunks
				found = true
			}
		}
	}

	if !found {
		errKind := "chunk_not_found"
		if hasDate {
			errKind = "date_not_found"
			logHandler.Info(ctx, "build_context: date not found", logging.KV{"date", dateStr})
		} else {
			logHandler.Info(ctx, "build_context: not found", logging.KV{"query_for_filter", queryForSearch})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(BuildContextResponse{
			Context: "", Error: errKind, SearchCollection: successCollection, CollectionsSearched: collectionsSearched, QueryForFilter: queryForSearch, ContextKind: contextKind, ContextRef: contextRef,
		})
		return
	}

	if req.AttachmentsText != "" {
		attach := strings.TrimSpace(req.AttachmentsText)
		if len(attach) > req.TokenBudget*2 {
			attach = attach[:req.TokenBudget*2]
		}
		contextText = attach + "\n\n" + contextText
	}

	if s.Config.Redis != nil {
		cachePayload, _ := json.Marshal(map[string]interface{}{
			"context":              contextText,
			"chunk_ids":            chunkIDs,
			"context_kind":         contextKind,
			"context_ref":          contextRef,
			"search_collection":    successCollection,
			"collections_searched": collectionsSearched,
			"query_for_filter":     queryForSearch,
		})
		_ = s.Config.Redis.Set(ctx, cacheKey, string(cachePayload), RetrievalCacheTTL).Err()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BuildContextResponse{
		Context: contextText, ChunkIDs: chunkIDs, ContextKind: contextKind, ContextRef: contextRef, SearchCollection: successCollection, CollectionsSearched: collectionsSearched, QueryForFilter: queryForSearch,
	})
}

func chunkIDsFromChunks(chunks []ChunkInfo) []string {
	chunkIDSet := make(map[string]struct{})
	for _, c := range chunks {
		if c.ChunkID != "" {
			chunkIDSet[c.ChunkID] = struct{}{}
		}
	}
	ids := make([]string, 0, len(chunkIDSet))
	for id := range chunkIDSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
