package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type rebuildReq struct {
	Service string `json:"service"`
	All     bool   `json:"all"`
}

func (s *server) handleRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	var req rebuildReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	composePath, err := s.composeFile()
	if err != nil {
		http.Error(w, `{"error":"no compose"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Minute)
	defer cancel()
	base := append([]string{"docker"}, s.dockerComposeBaseArgs(composePath)...)

	var logs strings.Builder
	var runErr error

	if req.All {
		namesOut, err := runCmd(ctx, base[0], append(base[1:], "config", "--services")...)
		if err != nil {
			writeRebuildJSON(w, false, appendRunLog(&logs, namesOut, err))
			return
		}
		var names []string
		for _, line := range strings.Split(string(namesOut), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				names = append(names, line)
			}
		}
		for _, svc := range names {
			logs.WriteString(fmt.Sprintf("=== up --build %s ===\n", svc))
			out, err := runCmd(ctx, base[0], append(base[1:], "up", "-d", "--build", svc)...)
			logs.Write(out)
			if err != nil {
				logs.WriteString("\n" + err.Error() + "\n")
				runErr = err
				break
			}
			logs.WriteString("\n")
		}
	} else {
		svc := strings.TrimSpace(req.Service)
		if svc == "" {
			http.Error(w, `{"error":"service required unless all"}`, http.StatusBadRequest)
			return
		}
		logs.WriteString(fmt.Sprintf("=== up --build %s ===\n", svc))
		out, err := runCmd(ctx, base[0], append(base[1:], "up", "-d", "--build", svc)...)
		logs.Write(out)
		if err != nil {
			logs.WriteString("\n" + err.Error() + "\n")
			runErr = err
		}
	}

	ok := runErr == nil
	writeRebuildJSON(w, ok, trimLog(logs.String(), 120000))
}

func writeRebuildJSON(w http.ResponseWriter, ok bool, logText string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": ok, "log": logText})
}

func appendRunLog(logs *strings.Builder, out []byte, err error) string {
	logs.Write(out)
	if err != nil {
		logs.WriteString("\n" + err.Error())
	}
	return trimLog(logs.String(), 120000)
}

