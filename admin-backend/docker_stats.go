package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	adminWebUISkip   = "admin-web-ui"
	dockerSocketPath = "/var/run/docker.sock"
	dockerAPIVersion = "v1.41"
)

// ContainerMetrics — CPU и RAM по контейнеру (для монитора).
type ContainerMetrics struct {
	Name       string  `json:"name"`
	CPUPct     int     `json:"cpu_pct"`
	RAMPct     int     `json:"ram_pct"`
	RamUsedGB  float64 `json:"ram_used_gb,omitempty"`
	RamLimitGB float64 `json:"ram_limit_gb,omitempty"`
}

type dockerContainer struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
}

type dockerStatsResponse struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
}

type containerCPUCache struct {
	totalUsage  uint64
	systemUsage uint64
	at          time.Time
}

var (
	dockerHTTPClient   *http.Client
	dockerClientOnce   sync.Once
	containerCPUMu     sync.Mutex
	containerCPUPrev   map[string]containerCPUCache
)

func getDockerHTTPClient() *http.Client {
	dockerClientOnce.Do(func() {
		dockerHTTPClient = &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("unix", dockerSocketPath)
				},
			},
		}
	})
	return dockerHTTPClient
}

// skipContainer возвращает true, если контейнер нужно исключить (admin-web-ui).
func skipContainer(name, image string) bool {
	lower := strings.ToLower(name + " " + image)
	return strings.Contains(lower, adminWebUISkip)
}

// CollectContainerMetrics собирает CPU и RAM по всем запущенным контейнерам, кроме admin-web-ui.
func CollectContainerMetrics() []ContainerMetrics {
	client := getDockerHTTPClient()
	req, err := http.NewRequest(http.MethodGet, "http://localhost/"+dockerAPIVersion+"/containers/json", nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	var containers []dockerContainer
	if json.Unmarshal(body, &containers) != nil {
		return nil
	}

	containerCPUMu.Lock()
	if containerCPUPrev == nil {
		containerCPUPrev = make(map[string]containerCPUCache)
	}
	containerCPUMu.Unlock()

	var result []ContainerMetrics
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		image := c.Image
		if skipContainer(name, image) {
			continue
		}
		if name == "" {
			if len(c.ID) >= 12 {
				name = c.ID[:12]
			} else {
				name = c.ID
			}
		}

		statsReq, _ := http.NewRequest(http.MethodGet, "http://localhost/"+dockerAPIVersion+"/containers/"+c.ID+"/stats?stream=0", nil)
		statsResp, err := client.Do(statsReq)
		if err != nil {
			continue
		}
		statsBody, _ := io.ReadAll(statsResp.Body)
		_ = statsResp.Body.Close()

		var res dockerStatsResponse
		if json.Unmarshal(statsBody, &res) != nil {
			continue
		}

		cpuPct := 0
		containerCPUMu.Lock()
		prev, ok := containerCPUPrev[c.ID]
		containerCPUMu.Unlock()
		if ok && res.CPUStats.SystemCPUUsage > prev.systemUsage && res.CPUStats.CPUUsage.TotalUsage >= prev.totalUsage {
			deltaCPU := res.CPUStats.CPUUsage.TotalUsage - prev.totalUsage
			deltaSys := res.CPUStats.SystemCPUUsage - prev.systemUsage
			if deltaSys > 0 {
				cpuPct = int(float64(deltaCPU)/float64(deltaSys)*100 + 0.5)
				if cpuPct > 100 {
					cpuPct = 100
				}
			}
		}
		containerCPUMu.Lock()
		containerCPUPrev[c.ID] = containerCPUCache{
			totalUsage:  res.CPUStats.CPUUsage.TotalUsage,
			systemUsage: res.CPUStats.SystemCPUUsage,
			at:          time.Now(),
		}
		containerCPUMu.Unlock()

		ramUsage := res.MemoryStats.Usage
		ramLimit := res.MemoryStats.Limit
		ramPct := 0
		if ramLimit > 0 {
			ramPct = int(float64(ramUsage)/float64(ramLimit)*100 + 0.5)
			if ramPct > 100 {
				ramPct = 100
			}
		}
		usedGB := float64(ramUsage) / (1024 * 1024 * 1024)
		limitGB := float64(ramLimit) / (1024 * 1024 * 1024)
		if ramLimit == 0 {
			limitGB = 0
		}

		result = append(result, ContainerMetrics{
			Name:       name,
			CPUPct:     cpuPct,
			RAMPct:     ramPct,
			RamUsedGB:  usedGB,
			RamLimitGB: limitGB,
		})
	}
	return result
}
