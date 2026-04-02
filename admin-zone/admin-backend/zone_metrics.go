package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type zoneMetricsAgentContainer struct {
	Name       string                  `json:"name"`
	CPUPct     int                     `json:"cpu_pct"`
	RAMPct     int                     `json:"ram_pct"`
	RamUsedGB  float64                 `json:"ram_used_gb"`
	RamLimitGB float64                 `json:"ram_limit_gb"`
	History    []ContainerHistoryPoint `json:"history"`
}

type zoneMetricsAgentResponse struct {
	ZoneID     string                      `json:"zone_id"`
	Containers []zoneMetricsAgentContainer `json:"containers"`
}

func (h *Handler) collectContainersForMonitor(ctx context.Context) []ContainerMetrics {
	if len(h.ZoneAgents) == 0 {
		return CollectContainerMetrics()
	}
	var mu sync.Mutex
	out := make([]ContainerMetrics, 0, 128)
	var wg sync.WaitGroup
	timeout := 25 * time.Second
	for _, z := range h.ZoneAgents {
		if strings.TrimSpace(z.AgentURL) == "" {
			continue
		}
		wg.Add(1)
		go func(zc ZoneAgentConfig) {
			defer wg.Done()
			ctx2, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx2, http.MethodGet, zc.AgentURL+"/v1/metrics", nil)
			if err != nil {
				return
			}
			req.Header.Set("X-Zone-Agent-Secret", zc.Secret)
			resp, err := h.zoneAgentClient(timeout).Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				return
			}
			var zm zoneMetricsAgentResponse
			if json.Unmarshal(body, &zm) != nil {
				return
			}
			zoneLabel := strings.TrimSpace(zm.ZoneID)
			if zoneLabel == "" {
				zoneLabel = zc.ID
			}
			mu.Lock()
			for _, c := range zm.Containers {
				display := fmt.Sprintf("%s / %s", zoneLabel, strings.TrimSpace(c.Name))
				hist := append([]ContainerHistoryPoint(nil), c.History...)
				out = append(out, ContainerMetrics{
					Name:       display,
					CPUPct:     c.CPUPct,
					RAMPct:     c.RAMPct,
					RamUsedGB:  c.RamUsedGB,
					RamLimitGB: c.RamLimitGB,
					History:    hist,
				})
			}
			mu.Unlock()
		}(z)
	}
	wg.Wait()
	if len(out) == 0 {
		return CollectContainerMetrics()
	}
	return out
}
