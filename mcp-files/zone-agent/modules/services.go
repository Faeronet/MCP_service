package modules

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type serviceRow struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	State   string `json:"state,omitempty"`
}

func (s *server) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	composePath, err := s.composeFile()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"services": []serviceRow{},
			"error":    err.Error(),
		})
		return
	}

	// services list from `docker compose config --services` gives stable order and includes multi-container.
	namesOut, err := runCmd(ctx, "docker", append(s.dockerComposeBaseArgs(composePath), "config", "--services")...)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"services": []serviceRow{},
			"error":    strings.TrimSpace(string(namesOut) + "\n" + err.Error()),
		})
		return
	}
	var names []string
	for _, line := range strings.Split(string(namesOut), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}

	psOut, psErr := runCmd(ctx, "docker", append(s.dockerComposeBaseArgs(composePath), "ps", "-a", "--format", "json")...)
	stateByService := parseComposePsJSON(psOut)
	if psErr != nil && len(stateByService) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"services": []serviceRow{},
			"error":    strings.TrimSpace(string(psOut) + "\n" + psErr.Error()),
		})
		return
	}

	rows := make([]serviceRow, 0, len(names))
	for _, n := range names {
		st := stateByService[n]
		rows = append(rows, serviceRow{
			Name:    n,
			Running: strings.EqualFold(st, "running"),
			State:   st,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"services": rows})
}

func parseComposePsJSON(raw []byte) map[string]string {
	out := make(map[string]string)
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return out
	}
	// JSON array
	if s[0] == '[' {
		var arr []map[string]interface{}
		if json.Unmarshal(raw, &arr) == nil {
			for _, o := range arr {
				svc, _ := o["Service"].(string)
				if svc == "" {
					svc, _ = o["service"].(string)
				}
				state, _ := o["State"].(string)
				if state == "" {
					state, _ = o["state"].(string)
				}
				if svc != "" {
					out[svc] = state
				}
			}
			return out
		}
	}

	// NDJSON fallback
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var o map[string]interface{}
		if json.Unmarshal([]byte(line), &o) != nil {
			continue
		}
		svc, _ := o["Service"].(string)
		if svc == "" {
			svc, _ = o["service"].(string)
		}
		state, _ := o["State"].(string)
		if state == "" {
			state, _ = o["state"].(string)
		}
		if svc != "" {
			mergeServiceState(out, svc, state)
		}
	}
	return out
}

func mergeServiceState(out map[string]string, svc, state string) {
	if strings.EqualFold(state, "running") {
		out[svc] = "running"
		return
	}
	if prev, ok := out[svc]; ok && strings.EqualFold(prev, "running") {
		return
	}
	if state != "" {
		out[svc] = state
	}
}

