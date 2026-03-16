package modules

import (
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telegram-ai-assistant/root/pkg/ratelimit"
)

const RetrievalCacheTTL = 60 * time.Second
const MaxSearchRounds = 5

const (
	CollectionChunks             = "chunks"
	CollectionZnakZodiaka        = "znak_zodiaka"
	CollectionObitanie           = "obitanie"
	CollectionKachestvaEnergii   = "kachestva_energii"
	CollectionIskazheniyaEnergii = "iskazheniya_energii"
	CollectionSpecificnost       = "specificnost"
	CollectionEmocionalnoe       = "emocionalnoe"
	CollectionIntellektualnye    = "intellektualnye"
	CollectionAstralnyiDuh       = "astralnyi_duh"
	CollectionOther              = "other"
)

type Handler struct {
	QdrantURL           string
	UseFullTextSearch   bool   // true = полнотекстовый поиск в Qdrant, векторный и проверки по словам отключены
	EmbedAPIBase        string
	EmbedAPIKey        string
	RerankAPIURL       string
	RerankAPIKey       string
	RerankMinScore     float64
	Redis              *redis.Client
	EmbedModel         string
	RerankModel        string
	RerankLimiter      *ratelimit.InFlight
	EmbedLimiter       *ratelimit.InFlight
	EmbedModelIDMu     sync.Mutex
	EmbedModelIDCached string
}

type BuildContextRequest struct {
	QueryText       string `json:"query_text"`
	ACLToken        string `json:"acl_token"`
	TokenBudget     int    `json:"token_budget"`
	Mode            string `json:"mode"`
	AttachmentsText string `json:"attachments_text"`
}

type BuildContextResponse struct {
	Context             string   `json:"context"`
	ChunkIDs            []string `json:"chunk_ids,omitempty"`
	ContextKind         string   `json:"context_kind,omitempty"`
	ContextRef          string   `json:"context_ref,omitempty"`
	SearchCollection    string   `json:"search_collection,omitempty"`
	CollectionsSearched []string `json:"collections_searched,omitempty"`
	QueryForFilter      string   `json:"query_for_filter,omitempty"`
	Error               string   `json:"error,omitempty"`
}

type ChunkInfo struct {
	Name           string
	Text           string
	SearchableText string
	ChunkID        string
	RelatedIDs     []string
	PrevID         string
	NextID         string
}

type GetFullContextRequest struct {
	ChunkID    string `json:"chunk_id"`
	Collection string `json:"collection"`
}

type QdrantSearchReq struct {
	Vector      []float32 `json:"vector"`
	Limit       uint64    `json:"limit"`
	WithPayload *bool     `json:"with_payload,omitempty"`
}

type QdrantSearchResult struct {
	Result []struct {
		Payload map[string]interface{} `json:"payload"`
	} `json:"result"`
}

type QdrantScrollReq struct {
	Filter      map[string]interface{} `json:"filter,omitempty"`
	Limit       *uint32                `json:"limit,omitempty"`
	WithPayload *bool                  `json:"with_payload,omitempty"`
	Offset      interface{}            `json:"offset,omitempty"`
}

type QdrantScrollResp struct {
	Result struct {
		Points         []struct{ Payload map[string]interface{} } `json:"points"`
		NextPageOffset interface{}                               `json:"next_page_offset"`
	} `json:"result"`
}
