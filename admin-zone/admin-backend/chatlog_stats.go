package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type llmInflightSample struct {
	TS    time.Time `json:"ts"`
	Count int       `json:"count"`
}

type chatStatsState struct {
	mu      sync.Mutex
	samples []llmInflightSample
}

func newChatStatsState() *chatStatsState {
	return &chatStatsState{samples: make([]llmInflightSample, 0, 1500)}
}

func (s *chatStatsState) add(ts time.Time, count int) {
	if s == nil {
		return
	}
	if count < 0 {
		count = 0
	}
	cutoff := ts.Add(-24 * time.Hour)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = append(s.samples, llmInflightSample{TS: ts.UTC(), Count: count})
	keepFrom := 0
	for i := 0; i < len(s.samples); i++ {
		if s.samples[i].TS.After(cutoff) || s.samples[i].TS.Equal(cutoff) {
			keepFrom = i
			break
		}
	}
	if keepFrom > 0 {
		s.samples = append([]llmInflightSample(nil), s.samples[keepFrom:]...)
	}
}

func (s *chatStatsState) last24h(now time.Time) []llmInflightSample {
	if s == nil {
		return nil
	}
	cutoff := now.Add(-24 * time.Hour)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]llmInflightSample, 0, len(s.samples))
	for _, p := range s.samples {
		if p.TS.After(cutoff) || p.TS.Equal(cutoff) {
			out = append(out, p)
		}
	}
	return out
}

func (h *Handler) fetchLLMInflightNow(ctx context.Context) int {
	base := strings.TrimSpace(h.MCPProxyURL)
	if base == "" {
		return 0
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(base, "/")+"/metrics/llm", nil)
	if err != nil {
		return 0
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	var out struct {
		LLMInflight int `json:"llm_inflight"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0
	}
	if out.LLMInflight < 0 {
		return 0
	}
	return out.LLMInflight
}

func (h *Handler) sampleLLMInflight() {
	if h == nil || h.ChatStats == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.ChatStats.add(time.Now().UTC(), h.fetchLLMInflightNow(ctx))
}

func (h *Handler) StartChatLogStatsSampler() {
	if h == nil {
		return
	}
	h.sampleLLMInflight()
	go func() {
		t := time.NewTicker(1 * time.Minute)
		defer t.Stop()
		for range t.C {
			h.sampleLLMInflight()
		}
	}()
}

// ChatLogStats returns total chats, current in-flight LLM, and 24h in-flight series.
func (h *Handler) ChatLogStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	var totalChats int64
	_ = h.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM chat.sessions`).Scan(&totalChats)

	inflight := h.fetchLLMInflightNow(ctx)
	now := time.Now().UTC()
	if h.ChatStats != nil {
		h.ChatStats.add(now, inflight)
	}
	series := []map[string]interface{}{}
	for _, p := range h.ChatStats.last24h(now) {
		series = append(series, map[string]interface{}{
			"ts":    p.TS.Format(time.RFC3339),
			"count": p.Count,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total_chats":    totalChats,
		"llm_inflight":   inflight,
		"series_24h":     series,
	})
}

