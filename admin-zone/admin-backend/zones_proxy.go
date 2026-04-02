package main

import (
	"context"
	"encoding/json"
	"io"
	stdlog "log"
	"net/http"
	"strings"
	"time"
)

// ZoneAgentConfig задаётся в ZONE_AGENTS (JSON-массив): прокси админки к zone-agent, развёрнутому в зоне (контейнер + bind-mount).
type ZoneAgentConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	AgentURL string `json:"agent_url"`
	Secret   string `json:"secret"`
}

func parseZoneAgentsJSON(raw string) []ZoneAgentConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if len(raw) >= 2 {
		if (raw[0] == '\'' && raw[len(raw)-1] == '\'') || (raw[0] == '"' && raw[len(raw)-1] == '"') {
			raw = strings.TrimSpace(raw[1 : len(raw)-1])
		}
	}
	var out []ZoneAgentConfig
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		stdlog.Printf("ZONE_AGENTS JSON parse error: %v", err)
		return nil
	}
	for i := range out {
		out[i].ID = strings.TrimSpace(out[i].ID)
		out[i].Name = strings.TrimSpace(out[i].Name)
		out[i].AgentURL = strings.TrimSuffix(strings.TrimSpace(out[i].AgentURL), "/")
		out[i].Secret = strings.TrimSpace(out[i].Secret)
		if out[i].Name == "" {
			out[i].Name = out[i].ID
		}
	}
	return out
}

func (h *Handler) zoneByID(id string) *ZoneAgentConfig {
	id = strings.TrimSpace(id)
	for i := range h.ZoneAgents {
		if h.ZoneAgents[i].ID == id {
			return &h.ZoneAgents[i]
		}
	}
	return nil
}

// ZonesRoutes: GET /api/zones; GET|PUT /api/zones/{id}/env; GET /api/zones/{id}/services; POST /api/zones/{id}/rebuild
func (h *Handler) ZonesRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "/api/zones" {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		h.zonesList(w, r)
		return
	}
	if !strings.HasPrefix(path, "/api/zones/") {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(path, "/api/zones/")
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" {
		http.NotFound(w, r)
		return
	}
	z := h.zoneByID(id)
	if z == nil {
		http.Error(w, `{"error":"unknown zone"}`, http.StatusNotFound)
		return
	}
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}
	switch sub {
	case "env":
		switch r.Method {
		case http.MethodGet:
			h.zoneProxyEnvGet(w, r, z)
		case http.MethodPut:
			h.zoneProxyEnvPut(w, r, z)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
		return
	case "services":
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		h.zoneProxyServices(w, r, z)
		return
	case "rebuild":
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		h.zoneProxyRebuild(w, r, z)
		return
	case "meta":
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		h.zoneProxyMeta(w, r, z)
		return
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) zonesList(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		AgentOK bool   `json:"agent_ok"`
		Hint    string `json:"hint,omitempty"`
	}
	out := make([]row, 0, len(h.ZoneAgents))
	client := &http.Client{Timeout: 3 * time.Second}
	for _, z := range h.ZoneAgents {
		if z.ID == "" || z.AgentURL == "" {
			continue
		}
		rr := row{ID: z.ID, Name: z.Name, AgentOK: false}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, z.AgentURL+"/healthz", nil)
		if err != nil {
			rr.Hint = err.Error()
			out = append(out, rr)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			rr.Hint = err.Error()
			out = append(out, rr)
			continue
		}
		resp.Body.Close()
		rr.AgentOK = resp.StatusCode == http.StatusOK
		if !rr.AgentOK {
			rr.Hint = "health check failed"
		}
		out = append(out, rr)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"zones": out})
}

func (h *Handler) zoneAgentClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func (h *Handler) zoneDo(ctx context.Context, z *ZoneAgentConfig, method, path string, body io.Reader, contentType string, timeout time.Duration) (*http.Response, error) {
	u := z.AgentURL + path
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Zone-Agent-Secret", z.Secret)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return h.zoneAgentClient(timeout).Do(req)
}

func (h *Handler) zoneProxyEnvGet(w http.ResponseWriter, r *http.Request, z *ZoneAgentConfig) {
	resp, err := h.zoneDo(r.Context(), z, http.MethodGet, "/v1/env", nil, "", 30*time.Second)
	if err != nil {
		http.Error(w, `{"error":"agent unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *Handler) zoneProxyEnvPut(w http.ResponseWriter, r *http.Request, z *ZoneAgentConfig) {
	body := io.LimitReader(r.Body, 2<<20)
	resp, err := h.zoneDo(r.Context(), z, http.MethodPut, "/v1/env", body, "text/plain; charset=utf-8", 30*time.Second)
	if err != nil {
		http.Error(w, `{"error":"agent unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *Handler) zoneProxyServices(w http.ResponseWriter, r *http.Request, z *ZoneAgentConfig) {
	resp, err := h.zoneDo(r.Context(), z, http.MethodGet, "/v1/services", nil, "", 90*time.Second)
	if err != nil {
		http.Error(w, `{"error":"agent unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *Handler) zoneProxyRebuild(w http.ResponseWriter, r *http.Request, z *ZoneAgentConfig) {
	resp, err := h.zoneDo(r.Context(), z, http.MethodPost, "/v1/rebuild", r.Body, "application/json", 46*time.Minute)
	if err != nil {
		http.Error(w, `{"error":"agent unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *Handler) zoneProxyMeta(w http.ResponseWriter, r *http.Request, z *ZoneAgentConfig) {
	resp, err := h.zoneDo(r.Context(), z, http.MethodGet, "/v1/meta", nil, "", 15*time.Second)
	if err != nil {
		http.Error(w, `{"error":"agent unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
