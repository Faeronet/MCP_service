package modules

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

func (s *server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()

	containers := s.collectZoneDockerMetrics(ctx)
	zid := strings.TrimSpace(os.Getenv("ZONE_AGENT_ZONE_ID"))
	if zid == "" {
		zid = strings.TrimSpace(s.composeProject)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"zone_id":    zid,
		"containers": containers,
	})
}
