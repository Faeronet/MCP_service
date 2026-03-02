package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var (
	// Исключаем из сбора: admin-web-ui, postgres, логирование (loki, promtail, grafana), migrate.
	containerSkipSubstrings = []string{"admin-web-ui", "postgres", "migrate", "loki", "promtail", "grafana"}
)

func getDockerSocketPath() string {
	if p := os.Getenv("DOCKER_SOCKET_PATH"); p != "" {
		return p
	}
	return "/var/run/docker.sock"
}

func getDockerAPIVersion() string {
	if v := os.Getenv("DOCKER_API_VERSION"); v != "" {
		return strings.TrimPrefix(v, "v")
	}
	return "1.41"
}

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
		socketPath := getDockerSocketPath()
		dockerHTTPClient = &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		}
	})
	return dockerHTTPClient
}

// skipContainer возвращает true, если контейнер исключаем: postgres, логирование (loki, promtail, grafana), admin-web-ui, migrate.
func skipContainer(name, image string) bool {
	lower := strings.ToLower(name + " " + image)
	for _, sub := range containerSkipSubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// tryContainersList запрашивает список контейнеров; возвращает (body, apiVer, ok, lastErr).
func tryContainersList(client *http.Client, apiVersions []string) ([]byte, string, bool, error) {
	var lastErr error
	for _, apiVer := range apiVersions {
		path := "http://localhost/v" + apiVer + "/containers/json"
		req, err := http.NewRequest(http.MethodGet, path, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Host", "localhost")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return body, apiVer, true, nil
		}
		lastErr = fmt.Errorf("api v%s: %d %s", apiVer, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil, "", false, lastErr
}

// CollectContainerMetrics собирает CPU и RAM по всем запущенным контейнерам, кроме postgres, логирования, admin-web-ui.
func CollectContainerMetrics() []ContainerMetrics {
	ctx := context.Background()
	client := getDockerHTTPClient()
	// Пробуем версии от новой к старой (некоторые демоны отдают 400 для старых версий).
	apiVersions := []string{"1.45", "1.44", "1.43", "1.42", "1.41", "1.40", "1.39", getDockerAPIVersion()}
	body, apiVer, ok, listErr := tryContainersList(client, apiVersions)
	if !ok || body == nil {
		errStr := ""
		if listErr != nil {
			errStr = listErr.Error()
		}
		log.Warn(ctx, "container metrics: docker list failed", logging.KV{"error", errStr}, logging.KV{"socket", getDockerSocketPath()})
		return nil
	}
	var containers []dockerContainer
	if json.Unmarshal(body, &containers) != nil {
		log.Warn(ctx, "container metrics: docker list json decode failed", logging.KV{"api_version", apiVer})
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

		statsReq, _ := http.NewRequest(http.MethodGet, "http://localhost/v"+apiVer+"/containers/"+c.ID+"/stats?stream=0", nil)
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
