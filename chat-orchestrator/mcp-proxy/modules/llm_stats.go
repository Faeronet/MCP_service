package modules

import (
	"encoding/json"
	"net/http"
	"time"
)

// HandleLLMStats returns current in-flight LLM requests.
func (s *Server) HandleLLMStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	current := 0
	if s != nil && s.LlmLimiter != nil {
		current = s.LlmLimiter.Current()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":               true,
		"llm_inflight":     current,
		"sampled_at":       time.Now().UTC().Format(time.RFC3339),
	})
}
