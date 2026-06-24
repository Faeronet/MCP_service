package modules

import (
	"context"
	"sort"
	"strings"

	"github.com/telegram-ai-assistant/root/pkg/ratelimit"
)

func isTriggerCollection(name string) bool {
	return name == CollectionEmocionalnoe || name == CollectionIntellektualnye || name == CollectionAstralnyiDuh
}

func searchOneCollectionNoDate(
	ctx context.Context,
	qdrantClient *QdrantClient,
	rerankClient *RerankClient,
	rerankLimiter *ratelimit.InFlight,
	collectionName string,
	vec []float32,
	queryForSearch string,
	tokenBudget int,
	rerankMinScore float64,
	useFullTextSearch bool,
	embedEnabled bool,
) (contextText string, chunkIDs []string, found bool) {
	if tokenBudget <= 0 {
		tokenBudget = 4000
	}
	queryTrim := strings.TrimSpace(queryForSearch)
	limitItems := 0
	if isTriggerCollection(collectionName) {
		limitItems = MaxChunksTriggerCollections
	}
	// Для эмоц./интел./астральный дух всегда полнотекстовый поиск по индексированным полям payload
	useFTS := useFullTextSearch || isTriggerCollection(collectionName)

	if useFTS {
		// Полнотекстовый поиск: без векторов и без проверки по границам слов
		limit := uint32(100)
		if limitItems > 0 {
			limit = uint32(limitItems * 4)
			if limit > 20 {
				limit = 20
			}
		}
		items, err := qdrantClient.ScrollWithFullTextFilter(ctx, collectionName, queryTrim, limit)
		if err == nil {
			if len(items) == 0 && queryTrim != "" {
				items = qdrantClient.ScrollAllChunksContaining(ctx, collectionName, queryTrim)
			}
			if len(items) > 0 {
				if limitItems > 0 && queryTrim != "" {
					queryLower := strings.ToLower(queryTrim)
					words := strings.Fields(queryLower)
					wordsRequired := FilterSignificantWords(words)
					var containing []ChunkInfo
					for _, c := range items {
						chunkLower := strings.ToLower(c.Text + " " + c.SearchableText + " " + c.Name)
						allFound := true
						for _, w := range wordsRequired {
							if w == "" {
								continue
							}
							if !chunkContainsQueryWord(chunkLower, w) {
								allFound = false
								break
							}
						}
						if allFound {
							containing = append(containing, c)
						}
					}
					if len(containing) > 0 {
						items = containing
					}
				}
				mainChunkIDs := make(map[string]struct{})
				neighborIDs := make(map[string]struct{})
				for _, c := range items {
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
				if limitItems == 0 && len(neighborIDs) > 0 {
					neighbors := qdrantClient.FetchChunkPayloadsByID(ctx, collectionName, neighborIDs)
					items = append(items, neighbors...)
				}
				texts := make([]string, len(items))
				for i := range items {
					texts[i] = items[i].Text
				}
				if rerankClient != nil {
					if err := rerankLimiter.Acquire(ctx); err == nil {
						_, order, topScore := rerankClient.RerankWithScoreAndOrder(ctx, queryForSearch, texts)
						rerankLimiter.Release()
						if topScore >= rerankMinScore && order != nil && len(order) == len(items) {
							ordered := make([]ChunkInfo, len(items))
							for i, idx := range order {
								ordered[i] = items[idx]
							}
							items = ordered
						}
					}
				}
				if limitItems > 0 && len(items) > limitItems {
					items = items[:limitItems]
				}
				ctxText, ids := buildContextFromChunks(items, tokenBudget)
				return ctxText, ids, true
			}
		}
		// FTS ничего не нашёл — пробуем векторный поиск и scroll по словам ниже
	}

	if !embedEnabled || len(vec) == 0 {
		if queryTrim != "" {
			scrollItems := qdrantClient.ScrollAllChunksContaining(ctx, collectionName, queryTrim)
			if len(scrollItems) > 0 {
				if limitItems > 0 && len(scrollItems) > limitItems {
					scrollItems = scrollItems[:limitItems]
				}
				ctxText, ids := buildContextFromChunks(scrollItems, tokenBudget)
				return ctxText, ids, true
			}
		}
		return "", nil, false
	}

	for round := 1; round <= MaxSearchRounds; round++ {
		limit := uint64(20 * round)
		if limit > 100 {
			limit = 100
		}
		items, err := qdrantClient.SearchPoints(ctx, collectionName, vec, limit)
		if err != nil || len(items) == 0 {
			continue
		}
		mainChunkIDs := make(map[string]struct{})
		neighborIDs := make(map[string]struct{})
		for _, c := range items {
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
			neighbors := qdrantClient.FetchChunkPayloadsByID(ctx, collectionName, neighborIDs)
			items = append(items, neighbors...)
		}

		texts := make([]string, len(items))
		for i := range items {
			texts[i] = items[i].Text
		}
		var topScore float64
		var order []int
		if rerankClient != nil {
			if err := rerankLimiter.Acquire(ctx); err != nil {
				continue
			}
			_, order, topScore = rerankClient.RerankWithScoreAndOrder(ctx, queryForSearch, texts)
			rerankLimiter.Release()
			if topScore < rerankMinScore {
				continue
			}
			if order != nil && len(order) == len(items) {
				ordered := make([]ChunkInfo, len(items))
				for i, idx := range order {
					ordered[i] = items[idx]
				}
				items = ordered
			}
		}

		if queryTrim != "" {
			queryLower := strings.ToLower(queryTrim)
			words := strings.Fields(queryLower)
			wordsRequired := FilterSignificantWords(words)
			var containing []ChunkInfo
			for _, c := range items {
				chunkLower := strings.ToLower(c.Text + " " + c.SearchableText + " " + c.Name)
				allFound := true
				for _, w := range wordsRequired {
					if w == "" {
						continue
					}
					if !chunkContainsQueryWord(chunkLower, w) {
						allFound = false
						break
					}
				}
				if allFound {
					containing = append(containing, c)
				}
			}
			if len(containing) == 0 {
				continue
			}
			items = containing
		}
		if limitItems > 0 && len(items) > limitItems {
			items = items[:limitItems]
		}
		ctxText, ids := buildContextFromChunks(items, tokenBudget)
		return ctxText, ids, true
	}

	if queryTrim != "" {
		scrollItems := qdrantClient.ScrollAllChunksContaining(ctx, collectionName, queryTrim)
		if len(scrollItems) > 0 {
			if limitItems > 0 && len(scrollItems) > limitItems {
				scrollItems = scrollItems[:limitItems]
			}
			ctxText, ids := buildContextFromChunks(scrollItems, tokenBudget)
			return ctxText, ids, true
		}
	}
	return "", nil, false
}

func buildContextFromChunks(items []ChunkInfo, tokenBudget int) (string, []string) {
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
	ctxText := b.String()
	if len(ctxText) > tokenBudget*4 {
		ctxText = ctxText[:tokenBudget*4]
	}
	chunkIDSet := make(map[string]struct{})
	for _, c := range items {
		if c.ChunkID != "" {
			chunkIDSet[c.ChunkID] = struct{}{}
		}
	}
	ids := make([]string, 0, len(chunkIDSet))
	for id := range chunkIDSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ctxText, ids
}
