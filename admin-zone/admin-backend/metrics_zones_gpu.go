package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type zoneGPUJSON struct {
	OK   bool `json:"ok"`
	GPUs []struct {
		Name         string  `json:"name"`
		GPUPct       int     `json:"gpu_pct"`
		VRAMPct      int     `json:"vram_pct"`
		VRAMUsedGB   float64 `json:"vram_used_gb"`
		VRAMTotalGB  float64 `json:"vram_total_gb"`
	} `json:"gpus"`
}

func gpuLocalIsPlaceholder(gpus []GPUMetrics) bool {
	if len(gpus) == 0 {
		return true
	}
	if len(gpus) == 1 && gpus[0].Name == "N/A" {
		return true
	}
	for _, g := range gpus {
		if g.GPUPct > 0 || g.VRAMPct > 0 || g.VRAMUsedGB > 0.02 || g.VRAMTotalGB > 0.5 {
			return false
		}
	}
	return true
}

func (h *Handler) fetchGPUMetricsFromZoneAgents() []GPUMetrics {
	if h == nil || len(h.ZoneAgents) == 0 {
		return nil
	}
	prefer := strings.Split(strings.TrimSpace(os.Getenv("MONITOR_GPU_ZONES")), ",")
	seen := make(map[string]bool)
	var ordered []ZoneAgentConfig
	for _, id := range prefer {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		z := h.zoneByID(id)
		if z != nil {
			ordered = append(ordered, *z)
			seen[z.ID] = true
		}
	}
	for _, z := range h.ZoneAgents {
		if z.ID == "" || z.AgentURL == "" {
			continue
		}
		if !seen[z.ID] {
			ordered = append(ordered, z)
		}
	}
	timeout := 15 * time.Second
	for _, z := range ordered {
		g := h.fetchOneZoneGPUMetrics(&z, timeout)
		if len(g) > 0 {
			return g
		}
	}
	return nil
}

func (h *Handler) fetchOneZoneGPUMetrics(z *ZoneAgentConfig, timeout time.Duration) []GPUMetrics {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, z.AgentURL+"/v1/metrics/gpu", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("X-Zone-Agent-Secret", z.Secret)
	resp, err := h.zoneAgentClient(timeout).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var parsed zoneGPUJSON
	if json.Unmarshal(body, &parsed) != nil || !parsed.OK || len(parsed.GPUs) == 0 {
		return nil
	}
	out := make([]GPUMetrics, 0, len(parsed.GPUs))
	for _, g := range parsed.GPUs {
		out = append(out, GPUMetrics{
			Name:         g.Name,
			GPUPct:       g.GPUPct,
			VRAMPct:      g.VRAMPct,
			VRAMUsedGB:   g.VRAMUsedGB,
			VRAMTotalGB:  g.VRAMTotalGB,
		})
	}
	return out
}

func mergeGPUWithZoneAgents(h *Handler, local []GPUMetrics) []GPUMetrics {
	if h == nil || !gpuLocalIsPlaceholder(local) {
		return local
	}
	z := h.fetchGPUMetricsFromZoneAgents()
	if len(z) > 0 {
		return z
	}
	return local
}
