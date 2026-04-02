package main

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

// Порог выше которого limit в Docker считаем «без лимита» (фактически машинная память).
const dockerMemoryUnlimitedThreshold uint64 = 1 << 42 // ~4 TiB

var hostMemTotalBytesOnce sync.Once
var hostMemTotalBytesCached uint64

// hostMemTotalBytes читает MemTotal из /proc/meminfo (в Linux-контейнере обычно это память хоста).
func hostMemTotalBytes() uint64 {
	hostMemTotalBytesOnce.Do(func() {
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
						hostMemTotalBytesCached = kb * 1024
					}
				}
				return
			}
		}
	})
	return hostMemTotalBytesCached
}

// dockerCPUPercentFromStats считает загрузку CPU по cpu_stats / precpu_stats из одного ответа stats (как Docker CLI).
func dockerCPUPercentFromStats(totalUsage, preTotal, sysUsage, preSys uint64, onlineCPUs uint32, percpuCount int) int {
	deltaCPU := int64(totalUsage) - int64(preTotal)
	deltaSys := int64(sysUsage) - int64(preSys)
	if deltaSys <= 0 || deltaCPU < 0 {
		return 0
	}
	n := percpuCount
	if n <= 0 {
		n = int(onlineCPUs)
	}
	if n <= 0 {
		n = 1
	}
	p := (float64(deltaCPU) / float64(deltaSys)) * float64(n) * 100.0
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return int(p + 0.5)
}

// dockerMemoryUsage picks RSS-style usage when main usage is 0 (cgroup v2 / некоторые драйверы).
func dockerMemoryUsage(usage uint64, stats map[string]uint64) uint64 {
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

// dockerRAMMetricsWithStats: ram_pct от лимита контейнера, если он задан; иначе — от памяти хоста (из /proc/meminfo).
func dockerRAMMetricsWithStats(usage, limit uint64, stats map[string]uint64) (ramPct int, ramUsedGB, ramLimitGB float64) {
	hostTotal := hostMemTotalBytes()
	u := dockerMemoryUsage(usage, stats)
	ramUsedGB = float64(u) / (1024 * 1024 * 1024)

	denom := limit
	if denom == 0 || denom > dockerMemoryUnlimitedThreshold {
		denom = hostTotal
		ramLimitGB = float64(denom) / (1024 * 1024 * 1024)
	} else {
		ramLimitGB = float64(limit) / (1024 * 1024 * 1024)
	}
	if denom == 0 {
		return 0, ramUsedGB, ramLimitGB
	}
	p := float64(u) / float64(denom) * 100
	if p > 100 {
		p = 100
	}
	if p < 0 {
		p = 0
	}
	return int(p + 0.5), ramUsedGB, ramLimitGB
}
