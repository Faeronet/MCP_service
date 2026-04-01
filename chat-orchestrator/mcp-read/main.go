package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telegram-ai-assistant/root/chat-orchestrator/mcp-read/modules"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
	"github.com/telegram-ai-assistant/root/pkg/ratelimit"
)

var log = logging.New("mcp-read")

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

	useFullText := config.LoadString("USE_FULLTEXT_SEARCH", "true") == "true" || config.LoadString("USE_FULLTEXT_SEARCH", "true") == "1"
	cfg := &modules.Handler{
		QdrantURL:         qdrantURL,
		UseFullTextSearch:  useFullText,
		Redis:              rdb,
		RerankMinScore:     rerankMinScore,
		RerankLimiter:      rerankLimiter,
		EmbedLimiter:       embedLimiter,
	}
	embedClient := modules.NewEmbedClient(embedAPIBase, embedAPIKey, embedModel)
	rerankClient := modules.NewRerankClient(rerankAPIURL, rerankAPIKey, rerankModel)
	qdrantClient := modules.NewQdrantClient(qdrantURL)

	srv := &modules.Server{
		Config: cfg,
		Embed:  embedClient,
		Rerank: rerankClient,
		Qdrant: qdrantClient,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/mcp/build_context", modules.RequestIDMiddleware(srv.BuildContext))
	mux.HandleFunc("/mcp/all_names", modules.RequestIDMiddleware(srv.AllNames))
	mux.HandleFunc("/mcp/full_context", modules.RequestIDMiddleware(srv.GetFullContext))

	httpSrv := &http.Server{Addr: ":8082", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Info(ctx, "mcp-read listening on :8082")
	_ = httpSrv.ListenAndServe()
}
