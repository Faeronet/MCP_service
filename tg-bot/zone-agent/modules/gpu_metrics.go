package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type gpuMetricJSON struct {
	Name         string  `json:"name"`
	GPUPct       int     `json:"gpu_pct"`
	VRAMPct      int     `json:"vram_pct"`
	VRAMUsedGB   float64 `json:"vram_used_gb"`
	VRAMTotalGB  float64 `json:"vram_total_gb"`
}

func (s *server) handleGPUMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	gpus, err := s.probeGPUsViaDockerExec(ctx)
	w.Header().Set("Content-Type", "application/json")
	if len(gpus) == 0 {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"gpus":  []gpuMetricJSON{},
			"error": msg,
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "gpus": gpus})
}

func gpuProbeServices() []string {
	raw := strings.TrimSpace(os.Getenv("ZONE_AGENT_GPU_PROBE_SERVICES"))
	if raw == "" {
		return []string{"vllm", "vllm-embed", "rerank"}
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"vllm", "vllm-embed", "rerank"}
	}
	return out
}

// probeGPUsViaDockerExec запускает nvidia-smi внутри контейнера зоны с GPU (образ zone-agent не содержит драйверов).
func (s *server) probeGPUsViaDockerExec(ctx context.Context) ([]gpuMetricJSON, error) {
	wantProj := normalizeComposeProjectLabel(s.composeProject)
	if wantProj == "" {
		return nil, fmt.Errorf("compose project empty")
	}
	client := getDockerMetricsHTTPClient()
	body, _, ok := tryDockerList(client)
	if !ok || len(body) == 0 {
		return nil, fmt.Errorf("docker list failed")
	}
	var list []dockerListItem
	if json.Unmarshal(body, &list) != nil {
		return nil, fmt.Errorf("docker list json")
	}
	probeSvcs := gpuProbeServices()
	var cid string
	for _, c := range list {
		proj := composeProjectOf(c.Labels)
		if proj != wantProj {
			continue
		}
		svc := composeServiceOf(c.Labels)
		for _, want := range probeSvcs {
			if strings.EqualFold(strings.TrimSpace(svc), strings.TrimSpace(want)) {
				cid = c.ID
				break
			}
		}
		if cid != "" {
			break
		}
	}
	if cid == "" {
		return nil, fmt.Errorf("no gpu probe container for project %s (services %v)", wantProj, probeSvcs)
	}

	namesOut, err := runCmd(ctx, "docker", "exec", cid, "nvidia-smi", "-L")
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi -L: %w", err)
	}
	names := parseNvidiaSmiListLines(string(namesOut))

	metricsOut, err := runCmd(ctx, "docker", "exec", cid, "nvidia-smi",
		"--query-gpu=utilization.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits")
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi query: %w", err)
	}
	lines := splitNonEmptyLines(string(metricsOut))
	if len(lines) == 0 {
		return nil, fmt.Errorf("nvidia-smi empty csv")
	}
	out := make([]gpuMetricJSON, 0, len(lines))
	for i, line := range lines {
		gpuPct, memUsed, memTotal, ok := parseNvidiaCSVMetricsLine(line)
		if !ok {
			continue
		}
		name := "GPU " + strconv.Itoa(i)
		if i < len(names) && names[i] != "" {
			name = names[i]
		}
		vramPct := 0
		if memTotal > 0 {
			vramPct = int((memUsed / memTotal) * 100)
			if vramPct > 100 {
				vramPct = 100
			}
		}
		if gpuPct > 100 {
			gpuPct = 100
		}
		out = append(out, gpuMetricJSON{
			Name:        name,
			GPUPct:      gpuPct,
			VRAMPct:     vramPct,
			VRAMUsedGB:  memUsed / 1024,
			VRAMTotalGB: memTotal / 1024,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("parse nvidia-smi csv failed")
	}
	return out, nil
}

func parseNvidiaSmiListLines(s string) []string {
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, ": "); idx >= 0 {
			name := strings.TrimSpace(line[idx+2:])
			if end := strings.Index(name, " ("); end >= 0 {
				name = strings.TrimSpace(name[:end])
			}
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

func splitNonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseNvidiaCSVMetricsLine(line string) (gpuPct int, memUsedMiB, memTotalMiB float64, ok bool) {
	parts := strings.Split(strings.TrimSpace(line), ",")
	if len(parts) < 3 {
		return 0, 0, 0, false
	}
	gpuPct, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	memUsedMiB, _ = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	memTotalMiB, _ = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	return gpuPct, memUsedMiB, memTotalMiB, true
}
