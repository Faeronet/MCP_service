package modules

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var mcpPromptCatalog = []struct {
	ID    string
	File  string
	Title string
}{
	{"query_extract", "query_extract.txt", "Search query extract (Prompt A)"},
	{"answer", "answer.txt", "Answer / RAG (Prompt B)"},
	{"reminder_compose", "reminder_compose.txt", "Reminder compose (Prompt C)"},
}

func (s *server) mcpPromptsDir() string {
	d := strings.TrimSpace(os.Getenv("ZONE_AGENT_MCP_PROMPTS_DIR"))
	if d == "" {
		return filepath.Join(s.workdir, "mcp-proxy", "prompts")
	}
	if filepath.IsAbs(d) {
		return d
	}
	return filepath.Join(s.workdir, d)
}

func mcpPromptFileByID(id string) (file string, title string, ok bool) {
	id = strings.TrimSpace(id)
	for _, e := range mcpPromptCatalog {
		if e.ID == id {
			return e.File, e.Title, true
		}
	}
	return "", "", false
}

// handleMcpProxyPrompts: GET /v1/mcp-proxy-prompts (list), GET|PUT /v1/mcp-proxy-prompts/{id}
func (s *server) handleMcpProxyPrompts(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/mcp-proxy-prompts")
	rest = strings.Trim(rest, "/")
	if rest == "" {
		s.handleMcpPromptsList(w, r)
		return
	}
	s.handleMcpPromptOne(w, r, rest)
}

func (s *server) handleMcpPromptsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	dir := s.mcpPromptsDir()
	type row struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		File   string `json:"file"`
		Exists bool   `json:"exists"`
		Size   int64  `json:"size"`
	}
	rows := make([]row, 0, len(mcpPromptCatalog))
	var dirErr string
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		dirErr = "directory missing: " + dir
	}
	for _, e := range mcpPromptCatalog {
		p := filepath.Join(dir, e.File)
		st, err := os.Stat(p)
		exists := err == nil && !st.IsDir()
		var sz int64
		if exists {
			sz = st.Size()
		}
		rows = append(rows, row{ID: e.ID, Title: e.Title, File: e.File, Exists: exists, Size: sz})
	}
	w.Header().Set("Content-Type", "application/json")
	out := map[string]interface{}{"prompts": rows, "dir": dir}
	if dirErr != "" {
		out["error"] = dirErr
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (s *server) handleMcpPromptOne(w http.ResponseWriter, r *http.Request, id string) {
	file, _, ok := mcpPromptFileByID(id)
	if !ok {
		http.Error(w, `{"error":"unknown prompt id"}`, http.StatusNotFound)
		return
	}
	dir := s.mcpPromptsDir()
	path := filepath.Join(dir, file)
	switch r.Method {
	case http.MethodGet:
		b, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(b)
	case http.MethodPut:
		if _, err := os.Stat(dir); err != nil {
			http.Error(w, `{"error":"prompts directory missing"}`, http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			http.Error(w, `{"error":"read body"}`, http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			http.Error(w, `{"error":"write failed"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	default:
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
	}
}
