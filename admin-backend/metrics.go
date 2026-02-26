// +build linux

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const metricsHistorySize = 60

type GPUMetrics struct {
	Name    string  `json:"name"`
	GPUPct  int     `json:"gpu_pct"`
	VRAMPct int     `json:"vram_pct"`
}

type SystemMetrics struct {
	CPUPct   int `json:"cpu_pct"`
	RAMPct   int `json:"ram_pct"`
	DiskIOK  int `json:"disk_io_k"`
}

type HistoryPoint struct {
	TS      string         `json:"ts"`
	CPU     int            `json:"cpu"`
	RAM     int            `json:"ram"`
	DiskIO  int            `json:"disk_io"`
	GPUs    []GPUHistoryPt `json:"gpus,omitempty"`
	GPU     int            `json:"gpu,omitempty"` // first card for backward compat
	VRAM    int            `json:"vram,omitempty"`
}

type GPUHistoryPt struct {
	GPUPct  int `json:"gpu_pct"`
	VRAMPct int `json:"vram_pct"`
}

type metricsState struct {
	mu        sync.Mutex
	lastCPU   cpuSample
	lastDisk  diskSample
	history   []HistoryPoint
	lastGPUs  []GPUMetrics
}

type cpuSample struct {
	total, idle uint64
	at          time.Time
}

type diskSample struct {
	readSectors, writeSectors uint64
	at                        time.Time
}

var metricsStore metricsState

func readProcStatCPU() (total, idle uint64, err error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		for i := 1; i < len(fields); i++ {
			u, _ := strconv.ParseUint(fields[i], 10, 64)
			total += u
		}
		idle, _ = strconv.ParseUint(fields[4], 10, 64)
		return total, idle, nil
	}
	return 0, 0, fmt.Errorf("cpu line not found")
}

func readProcMeminfo() (totalKB, availableKB uint64, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	var total, available uint64
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fmt.Sscanf(line, "MemTotal:%d", &total)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fmt.Sscanf(line, "MemAvailable:%d", &available)
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found")
	}
	return total, available, nil
}

func readProcDiskstats() (readSectors, writeSectors uint64, err error) {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return 0, 0, err
	}
	var totalRead, totalWrite uint64
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		var major, minor, rdCount, rdSectors, wrCount, wrSectors uint64
		var name string
		n, _ := fmt.Sscanf(sc.Text(), "%d %d %s %d %*d %d %*d %d %*d %d", &major, &minor, &name, &rdCount, &rdSectors, &wrCount, &wrSectors)
		if n >= 7 && (strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "vd")) {
			totalRead += rdSectors
			totalWrite += wrSectors
		}
	}
	return totalRead, totalWrite, nil
}

func collectCPU(prev *cpuSample) (pct int, next cpuSample) {
	total, idle, err := readProcStatCPU()
	if err != nil {
		return 0, *prev
	}
	next = cpuSample{total: total, idle: idle, at: time.Now()}
	if prev.at.IsZero() || prev.total == 0 {
		return 0, next
	}
	dt := total - prev.total
	di := idle - prev.idle
	if dt == 0 {
		return 0, next
	}
	usage := (float64(dt-di) / float64(dt)) * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return int(usage + 0.5), next
}

func collectRAM() int {
	total, available, err := readProcMeminfo()
	if err != nil || total == 0 {
		return 0
	}
	used := total - available
	pct := (float64(used) / float64(total)) * 100
	if pct > 100 {
		pct = 100
	}
	return int(pct + 0.5)
}

func collectDiskIO(prev *diskSample) (ioK int, next diskSample) {
	rd, wr, err := readProcDiskstats()
	if err != nil {
		return 0, *prev
	}
	next = diskSample{readSectors: rd, writeSectors: wr, at: time.Now()}
	if prev.at.IsZero() {
		return 0, next
	}
	elapsed := next.at.Sub(prev.at).Seconds()
	if elapsed <= 0 {
		return 0, next
	}
	deltaRead := int64(rd - prev.readSectors)
	deltaWrite := int64(wr - prev.writeSectors)
	if deltaRead < 0 || deltaWrite < 0 {
		return 0, next
	}
	bytesPerSec := (float64(deltaRead+deltaWrite) * 512) / elapsed
	return int(bytesPerSec / 1024), next
}

func collectGPUs() []GPUMetrics {
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,utilization.gpu,memory.used,memory.total", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var gpus []GPUMetrics
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 4)
		if len(parts) < 4 {
			continue
		}
		name := strings.Trim(strings.TrimSpace(parts[0]), "\"")
		gpuPct, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		memUsedStr := strings.TrimSpace(parts[2])
		memTotalStr := strings.TrimSpace(parts[3])
		memUsedStr = strings.TrimSuffix(memUsedStr, " MiB")
		memTotalStr = strings.TrimSuffix(memTotalStr, " MiB")
		memUsed, _ := strconv.ParseFloat(memUsedStr, 64)
		memTotal, _ := strconv.ParseFloat(memTotalStr, 64)
		vramPct := 0
		if memTotal > 0 {
			vramPct = int((memUsed / memTotal) * 100)
		}
		if vramPct > 100 {
			vramPct = 100
		}
		if gpuPct > 100 {
			gpuPct = 100
		}
		gpus = append(gpus, GPUMetrics{Name: name, GPUPct: gpuPct, VRAMPct: vramPct})
	}
	return gpus
}

// mockMetrics returns placeholder data when real collection is unavailable.
func mockMetrics(now time.Time) (SystemMetrics, []GPUMetrics, []HistoryPoint) {
	cpu := 20 + int(now.Unix()%40)
	ram := 35 + int(now.Unix()%30)
	diskIO := 1500 + int(now.Unix()%2000)
	gpu := 10 + int(now.Unix()%50)
	vram := 25 + int(now.Unix()%35)
	if cpu > 100 {
		cpu = 100
	}
	if ram > 100 {
		ram = 100
	}
	if gpu > 100 {
		gpu = 100
	}
	if vram > 100 {
		vram = 100
	}
	system := SystemMetrics{CPUPct: cpu, RAMPct: ram, DiskIOK: diskIO}
	gpus := []GPUMetrics{{Name: "N/A", GPUPct: gpu, VRAMPct: vram}}
	var history []HistoryPoint
	for i := 59; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * time.Second)
		ht := t.Unix()
		history = append(history, HistoryPoint{
			TS:     t.Format(time.RFC3339),
			CPU:    15 + (ht % 45),
			RAM:    30 + (ht % 40),
			DiskIO: 1000 + (ht % 3000),
			GPU:    5 + (ht % 60),
			VRAM:   20 + (ht % 50),
			GPUs:   []GPUHistoryPt{{GPUPct: 5 + (ht % 60), VRAMPct: 20 + (ht % 50)}},
		})
	}
	return system, gpus, history
}

// CollectMetrics gathers current system and GPU metrics and appends to history.
// On non-Linux or when /proc is unavailable, returns mock data.
func CollectMetrics() (system SystemMetrics, gpus []GPUMetrics, history []HistoryPoint) {
	now := time.Now()
	if runtime.GOOS != "linux" {
		return mockMetrics(now)
	}
	if _, _, err := readProcStatCPU(); err != nil {
		return mockMetrics(now)
	}

	metricsStore.mu.Lock()
	defer metricsStore.mu.Unlock()

	system.CPUPct, metricsStore.lastCPU = collectCPU(&metricsStore.lastCPU)
	system.RAMPct = collectRAM()
	system.DiskIOK, metricsStore.lastDisk = collectDiskIO(&metricsStore.lastDisk)

	gpus = collectGPUs()
	if len(gpus) == 0 {
		gpus = metricsStore.lastGPUs
		if gpus == nil {
			gpus = []GPUMetrics{{Name: "N/A", GPUPct: 0, VRAMPct: 0}}
		}
	} else {
		metricsStore.lastGPUs = gpus
	}

	hp := HistoryPoint{
		TS:     now.Format(time.RFC3339),
		CPU:    system.CPUPct,
		RAM:    system.RAMPct,
		DiskIO: system.DiskIOK,
		GPUs:   make([]GPUHistoryPt, len(gpus)),
	}
	if len(gpus) > 0 {
		hp.GPU = gpus[0].GPUPct
		hp.VRAM = gpus[0].VRAMPct
	}
	for i := range gpus {
		hp.GPUs[i] = GPUHistoryPt{GPUPct: gpus[i].GPUPct, VRAMPct: gpus[i].VRAMPct}
	}

	metricsStore.history = append(metricsStore.history, hp)
	if len(metricsStore.history) > metricsHistorySize {
		metricsStore.history = metricsStore.history[len(metricsStore.history)-metricsHistorySize:]
	}
	history = make([]HistoryPoint, len(metricsStore.history))
	copy(history, metricsStore.history)
	return system, gpus, history
}
