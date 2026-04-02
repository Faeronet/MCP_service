package modules

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const metricsHistoryLen = 60

type metricsHistoryPoint struct {
	TS  string `json:"ts"`
	CPU int    `json:"cpu"`
	RAM int    `json:"ram"`
}

type containerMetricsOut struct {
	Name       string                `json:"name"`
	CPUPct     int                   `json:"cpu_pct"`
	RAMPct     int                   `json:"ram_pct"`
	RamUsedGB  float64               `json:"ram_used_gb,omitempty"`
	RamLimitGB float64               `json:"ram_limit_gb,omitempty"`
	History    []metricsHistoryPoint `json:"history,omitempty"`
}

type dockerListItem struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	Labels map[string]string `json:"Labels"`
}

type dockerStatsBody struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
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
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
}

type cpuPrev struct {
	totalUsage  uint64
	systemUsage uint64
	at          time.Time
}

var (
	dockerMetricsClient     *http.Client
	dockerMetricsClientOnce sync.Once
	metricsCPUMu            sync.Mutex
	metricsCPUPrev          map[string]cpuPrev
	metricsHistMu           sync.Mutex
	metricsHistByContainer  map[string][]metricsHistoryPoint
	nameSuffixRe           = regexp.MustCompile(`-\d+$`)
)

const dockerMemUnlimited uint64 = 1 << 42

var hostMemTotalOnce sync.Once
var hostMemTotalB uint64

func hostMemTotalBytesZA() uint64 {
	hostMemTotalOnce.Do(func() {
		b, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil && kb > 0 {
						hostMemTotalB = kb * 1024
					}
				}
				return
			}
		}
	})
	return hostMemTotalB
}

func dockerCPUPctZA(totalUsage, preTotal, sysUsage, preSys uint64, online uint32, percpuN int) int {
	dCPU := int64(totalUsage) - int64(preTotal)
	dSys := int64(sysUsage) - int64(preSys)
	if dSys <= 0 || dCPU < 0 {
		return 0
	}
	n := percpuN
	if n <= 0 {
		n = int(online)
	}
	if n <= 0 {
		n = 1
	}
	p := (float64(dCPU) / float64(dSys)) * float64(n) * 100.0
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return int(p + 0.5)
}

func dockerMemUsageZA(usage uint64, stats map[string]uint64) uint64 {
	if usage > 0 {
		return usage
	}
	if stats == nil {
		return 0
	}
	if v, ok := stats["total_rss"]; ok && v > 0 {
		return v
	}
	if v, ok := stats["rss"]; ok && v > 0 {
		return v
	}
	var sum uint64
	if v, ok := stats["anon"]; ok {
		sum += v
	}
	if v, ok := stats["file"]; ok {
		sum += v
	}
	if sum > 0 {
		return sum
	}
	return 0
}

func dockerRAMZA(usage, limit uint64, stats map[string]uint64) (pct int, usedGB, limGB float64) {
	host := hostMemTotalBytesZA()
	u := dockerMemUsageZA(usage, stats)
	usedGB = float64(u) / (1024 * 1024 * 1024)
	denom := limit
	if denom == 0 || denom > dockerMemUnlimited {
		denom = host
		limGB = float64(denom) / (1024 * 1024 * 1024)
	} else {
		limGB = float64(limit) / (1024 * 1024 * 1024)
	}
	if denom == 0 {
		return 0, usedGB, limGB
	}
	p := float64(u) / float64(denom) * 100
	if p > 100 {
		p = 100
	}
	if p < 0 {
		p = 0
	}
	return int(p + 0.5), usedGB, limGB
}

func dockerSockPath() string {
	if p := os.Getenv("DOCKER_SOCKET_PATH"); p != "" {
		return p
	}
	return "/var/run/docker.sock"
}

func dockerAPIVersions() []string {
	return []string{"1.45", "1.44", "1.43", "1.42", "1.41", "1.40", "1.39", "1.38"}
}

func getDockerMetricsHTTPClient() *http.Client {
	dockerMetricsClientOnce.Do(func() {
		dockerMetricsClient = &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", dockerSockPath())
				},
			},
		}
	})
	return dockerMetricsClient
}

func tryDockerList(client *http.Client) ([]byte, string, bool) {
	for _, v := range dockerAPIVersions() {
		req, err := http.NewRequest(http.MethodGet, "http://localhost/v"+v+"/containers/json", nil)
		if err != nil {
			continue
		}
		req.Header.Set("Host", "localhost")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return body, v, true
		}
	}
	return nil, "", false
}

func composeProjectOf(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	if v := strings.TrimSpace(labels["com.docker.compose.project"]); v != "" {
		return strings.ToLower(v)
	}
	return ""
}

func composeServiceOf(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	return strings.TrimSpace(labels["com.docker.compose.service"])
}

// collectZoneDockerMetrics lists containers for this compose project and returns CPU/RAM + short history.
func (s *server) collectZoneDockerMetrics(ctx context.Context) []containerMetricsOut {
	want := strings.ToLower(strings.TrimSpace(s.composeProject))
	if want == "" {
		return nil
	}
	client := getDockerMetricsHTTPClient()
	body, apiVer, ok := tryDockerList(client)
	if !ok || len(body) == 0 {
		return nil
	}
	var list []dockerListItem
	if json.Unmarshal(body, &list) != nil {
		return nil
	}

	metricsCPUMu.Lock()
	if metricsCPUPrev == nil {
		metricsCPUPrev = make(map[string]cpuPrev)
	}
	metricsCPUMu.Unlock()
	metricsHistMu.Lock()
	if metricsHistByContainer == nil {
		metricsHistByContainer = make(map[string][]metricsHistoryPoint)
	}
	metricsHistMu.Unlock()

	var out []containerMetricsOut
	for _, c := range list {
		if ctx.Err() != nil {
			break
		}
		proj := composeProjectOf(c.Labels)
		if proj != want {
			continue
		}
		svc := composeServiceOf(c.Labels)
		if svc == "" && len(c.Names) > 0 {
			svc = strings.TrimPrefix(c.Names[0], "/")
			svc = nameSuffixRe.ReplaceAllString(svc, "")
		}
		if svc == "" {
			continue
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
			"http://localhost/v"+apiVer+"/containers/"+c.ID+"/stats?stream=0", nil)
		req.Header.Set("Host", "localhost")
		statsResp, err := client.Do(req)
		if err != nil {
			continue
		}
		statsBody, _ := io.ReadAll(statsResp.Body)
		_ = statsResp.Body.Close()
		var res dockerStatsBody
		if json.Unmarshal(statsBody, &res) != nil {
			continue
		}

		cpuPct := dockerCPUPctZA(
			res.CPUStats.CPUUsage.TotalUsage,
			res.PreCPUStats.CPUUsage.TotalUsage,
			res.CPUStats.SystemCPUUsage,
			res.PreCPUStats.SystemCPUUsage,
			res.CPUStats.OnlineCPUs,
			len(res.CPUStats.CPUUsage.PercpuUsage),
		)
		if cpuPct == 0 {
			metricsCPUMu.Lock()
			prev, okp := metricsCPUPrev[c.ID]
			metricsCPUMu.Unlock()
			if okp && res.CPUStats.SystemCPUUsage > prev.systemUsage && res.CPUStats.CPUUsage.TotalUsage >= prev.totalUsage {
				deltaCPU := res.CPUStats.CPUUsage.TotalUsage - prev.totalUsage
				deltaSys := res.CPUStats.SystemCPUUsage - prev.systemUsage
				if deltaSys > 0 {
					cpuPct = int(float64(deltaCPU)/float64(deltaSys)*100 + 0.5)
					if cpuPct > 100 {
						cpuPct = 100
					}
				}
			}
		}
		metricsCPUMu.Lock()
		metricsCPUPrev[c.ID] = cpuPrev{
			totalUsage:  res.CPUStats.CPUUsage.TotalUsage,
			systemUsage: res.CPUStats.SystemCPUUsage,
			at:          time.Now(),
		}
		metricsCPUMu.Unlock()

		ramPct, usedGB, limitGB := dockerRAMZA(
			res.MemoryStats.Usage,
			res.MemoryStats.Limit,
			res.MemoryStats.Stats,
		)

		now := time.Now().Format(time.RFC3339)
		metricsHistMu.Lock()
		hist := metricsHistByContainer[c.ID]
		hist = append(hist, metricsHistoryPoint{TS: now, CPU: cpuPct, RAM: ramPct})
		if len(hist) > metricsHistoryLen {
			hist = hist[len(hist)-metricsHistoryLen:]
		}
		metricsHistByContainer[c.ID] = hist
		metricsHistMu.Unlock()

		out = append(out, containerMetricsOut{
			Name:       svc,
			CPUPct:     cpuPct,
			RAMPct:     ramPct,
			RamUsedGB:  usedGB,
			RamLimitGB: limitGB,
			History:    append([]metricsHistoryPoint(nil), hist...),
		})
	}
	return out
}
