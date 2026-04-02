package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func (s *server) handleMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	composePath, composeErr := s.composeFile()
	envPath := s.envPath()
	_, envErr := os.Stat(envPath)

	w.Header().Set("Content-Type", "application/json")
	out := map[string]interface{}{
		"workdir":           s.workdir,
		"compose_project":   strings.TrimSpace(s.composeProject),
		"env_path":          envPath,
		"env_exists":        envErr == nil,
		"compose_path":      composePath,
		"compose_exists":    composeErr == nil,
	}
	if composeErr != nil {
		out["compose_error"] = composeErr.Error()
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (s *server) handleEnv(w http.ResponseWriter, r *http.Request) {
	envPath := s.envPath()
	switch r.Method {
	case http.MethodGet:
		b, err := os.ReadFile(envPath)
		if err != nil {
			if os.IsNotExist(err) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = w.Write([]byte{})
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "read: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(b)
	case http.MethodPut:
		// 2 MiB hard limit; enough for env files in this project.
		body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20+1))
		if err != nil {
			http.Error(w, `{"error":"body"}`, http.StatusBadRequest)
			return
		}
		tmp := envPath + ".tmp." + fmt.Sprintf("%d", time.Now().UnixNano())
		if err := os.WriteFile(tmp, body, 0600); err != nil {
			http.Error(w, `{"error":"write tmp"}`, http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmp, envPath); err != nil {
			_ = os.Remove(tmp)
			http.Error(w, `{"error":"rename"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	default:
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
	}
}

func trimSpaces(s string) string {
	return strings.TrimSpace(s)
}

