package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const metricsHistorySize = 60

type GPUMetrics struct {
	Name        string  `json:"name"`
	GPUPct      int     `json:"gpu_pct"`
	VRAMPct     int     `json:"vram_pct"`
	VRAMUsedGB  float64 `json:"vram_used_gb"`
	VRAMTotalGB float64 `json:"vram_total_gb"`
}

type SystemMetrics struct {
	CPUPct      int     `json:"cpu_pct"`
	RAMPct     int     `json:"ram_pct"`
	RamUsedGB  float64 `json:"ram_used_gb"`
	RamTotalGB float64 `json:"ram_total_gb"`
	DiskIOK     int     `json:"disk_io_k"`
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
	mu          sync.Mutex
	lastCPU     cpuSample
	lastCgroup  cgroupCPUSample
	lastDisk    diskSample
	history     []HistoryPoint
	lastGPUs    []GPUMetrics
}

type cpuSample struct {
	total, idle uint64
	at          time.Time
}

type cgroupCPUSample struct {
	usageUsec     uint64
	systemJiffies uint64
	at            time.Time
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

// getProcStatTotalJiffies returns the sum of all "cpu " line fields (total jiffies across all cores).
func getProcStatTotalJiffies() (uint64, error) {
	total, _, err := readProcStatCPU()
	return total, err
}

// readCgroupCPUUsageV2 reads usage_usec from cgroup v2 cpu.stat (container CPU).
func readCgroupCPUUsageV2() (usageUsec uint64, ok bool) {
	cgroupPath := getCgroupPathV2()
	if cgroupPath == "" {
		return 0, false
	}
	statPath := filepath.Join("/sys/fs/cgroup", cgroupPath, "cpu.stat")
	statData, err := os.ReadFile(statPath)
	if err != nil {
		return 0, false
	}
	sc := bufio.NewScanner(bytes.NewReader(statData))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "usage_usec ") {
			fmt.Sscanf(line, "usage_usec %d", &usageUsec)
			return usageUsec, true
		}
	}
	return 0, false
}

// readCgroupCPUUsageV1 reads cpuacct.usage (nanoseconds) from cgroup v1.
func readCgroupCPUUsageV1() (usageNsec uint64, ok bool) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return 0, false
	}
	var cpuPath string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, ":cpuacct,") || strings.Contains(line, ":cpu,") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 3 {
				cpuPath = strings.TrimPrefix(parts[2], "/")
				break
			}
		}
	}
	if cpuPath == "" {
		return 0, false
	}
	for _, root := range []string{"/sys/fs/cgroup/cpu,cpuacct", "/sys/fs/cgroup/cpuacct", "/sys/fs/cgroup/cpu"} {
		usagePath := filepath.Join(root, cpuPath, "cpuacct.usage")
		b, err := os.ReadFile(usagePath)
		if err != nil {
			continue
		}
		fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &usageNsec)
		return usageNsec, true
	}
	return 0, false
}

// getCgroupPathV2 returns the cgroup path from "0::/path" in /proc/self/cgroup, or "" if not v2.
func getCgroupPathV2() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "0::") {
			p := strings.TrimPrefix(line, "0::")
			p = strings.TrimPrefix(p, "/")
			return p
		}
	}
	return ""
}

// readCgroupMemoryV2 reads memory.current and memory.max from cgroup v2 (container RAM).
func readCgroupMemoryV2() (usedBytes, limitBytes uint64, ok bool) {
	path := getCgroupPathV2()
	if path == "" {
		return 0, 0, false
	}
	base := filepath.Join("/sys/fs/cgroup", path)
	currentPath := filepath.Join(base, "memory.current")
	maxPath := filepath.Join(base, "memory.max")
	bCurrent, err := os.ReadFile(currentPath)
	if err != nil {
		return 0, 0, false
	}
	usedBytes, _ = strconv.ParseUint(strings.TrimSpace(string(bCurrent)), 10, 64)
	bMax, err := os.ReadFile(maxPath)
	if err != nil {
		return usedBytes, 0, true
	}
	maxStr := strings.TrimSpace(string(bMax))
	if maxStr == "max" {
		limitBytes = 0
	} else {
		limitBytes, _ = strconv.ParseUint(maxStr, 10, 64)
	}
	return usedBytes, limitBytes, true
}

// readCgroupMemoryV1 reads memory.usage_in_bytes and memory.limit_in_bytes from cgroup v1.
func readCgroupMemoryV1() (usedBytes, limitBytes uint64, ok bool) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return 0, 0, false
	}
	var memPath string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, ":memory:") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 3 {
				memPath = strings.TrimPrefix(parts[2], "/")
				break
			}
		}
	}
	if memPath == "" {
		return 0, 0, false
	}
	for _, root := range []string{"/sys/fs/cgroup/memory", "/sys/fs/cgroup/memory,cgroup"} {
		usagePath := filepath.Join(root, memPath, "memory.usage_in_bytes")
		limitPath := filepath.Join(root, memPath, "memory.limit_in_bytes")
		bUsage, err := os.ReadFile(usagePath)
		if err != nil {
			continue
		}
		usedBytes, _ = strconv.ParseUint(strings.TrimSpace(string(bUsage)), 10, 64)
		bLimit, err := os.ReadFile(limitPath)
		if err != nil {
			return usedBytes, 0, true
		}
		limitBytes, _ = strconv.ParseUint(strings.TrimSpace(string(bLimit)), 10, 64)
		return usedBytes, limitBytes, true
	}
	return 0, 0, false
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
		if n >= 7 && (strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "vd") || strings.HasPrefix(name, "xvd")) {
			totalRead += rdSectors
			totalWrite += wrSectors
		}
	}
	return totalRead, totalWrite, nil
}

const jiffiesToUsec = 10000 // USER_HZ=100 => 1 jiffy = 10ms = 10000 usec

func collectCPU(prev *cpuSample, prevCgroup *cgroupCPUSample) (pct int, next cpuSample, nextCgroup cgroupCPUSample) {
	now := time.Now()
	systemJiffies, _ := getProcStatTotalJiffies()

	// Prefer cgroup (container) CPU when available — reflects this process/container, not the whole host.
	if usageUsec, ok := readCgroupCPUUsageV2(); ok {
		nextCgroup = cgroupCPUSample{usageUsec: usageUsec, systemJiffies: systemJiffies, at: now}
		if !prevCgroup.at.IsZero() && prevCgroup.systemJiffies > 0 {
			deltaUsec := int64(usageUsec - prevCgroup.usageUsec)
			deltaJiffies := int64(systemJiffies - prevCgroup.systemJiffies)
			if deltaJiffies > 0 && deltaUsec >= 0 {
				// container usage as % of total system CPU
				pctVal := (float64(deltaUsec) * 100) / (float64(deltaJiffies) * float64(jiffiesToUsec))
				if pctVal < 0 {
					pctVal = 0
				}
				if pctVal > 100 {
					pctVal = 100
				}
				pct = int(pctVal + 0.5)
				return pct, *prev, nextCgroup
			}
		}
		return 0, *prev, nextCgroup
	}
	if usageNsec, ok := readCgroupCPUUsageV1(); ok {
		usageUsec := usageNsec / 1000
		nextCgroup = cgroupCPUSample{usageUsec: usageUsec, systemJiffies: systemJiffies, at: now}
		if !prevCgroup.at.IsZero() && prevCgroup.systemJiffies > 0 {
			deltaUsec := int64(usageUsec - prevCgroup.usageUsec)
			deltaJiffies := int64(systemJiffies - prevCgroup.systemJiffies)
			if deltaJiffies > 0 && deltaUsec >= 0 {
				pctVal := (float64(deltaUsec) * 100) / (float64(deltaJiffies) * float64(jiffiesToUsec))
				if pctVal < 0 {
					pctVal = 0
				}
				if pctVal > 100 {
					pctVal = 100
				}
				pct = int(pctVal + 0.5)
				return pct, *prev, nextCgroup
			}
		}
		return 0, *prev, nextCgroup
	}

	// Fallback: host CPU from /proc/stat
	total, idle, err := readProcStatCPU()
	if err != nil {
		return 0, *prev, nextCgroup
	}
	next = cpuSample{total: total, idle: idle, at: now}
	if prev.at.IsZero() || prev.total == 0 {
		return 0, next, nextCgroup
	}
	dt := total - prev.total
	di := idle - prev.idle
	if dt == 0 {
		return 0, next, nextCgroup
	}
	usage := (float64(dt-di) / float64(dt)) * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return int(usage + 0.5), next, nextCgroup
}

func collectRAM() (pct int, usedGB, totalGB float64) {
	const bytesToGB = 1024 * 1024 * 1024
	// Prefer cgroup (container) memory when available.
	if usedB, limitB, ok := readCgroupMemoryV2(); ok {
		usedGB = float64(usedB) / bytesToGB
		if limitB > 0 {
			totalGB = float64(limitB) / bytesToGB
			pctVal := (float64(usedB) / float64(limitB)) * 100
			if pctVal > 100 {
				pctVal = 100
			}
			pct = int(pctVal + 0.5)
		} else {
			total, _, _ := readProcMeminfo()
			if total > 0 {
				totalGB = float64(total) / (1024 * 1024)
				pctVal := (float64(usedB) / (float64(total) * 1024)) * 100
				if pctVal > 100 {
					pctVal = 100
				}
				pct = int(pctVal + 0.5)
			}
		}
		return pct, usedGB, totalGB
	}
	if usedB, limitB, ok := readCgroupMemoryV1(); ok {
		usedGB = float64(usedB) / bytesToGB
		if limitB > 0 {
			totalGB = float64(limitB) / bytesToGB
			pctVal := (float64(usedB) / float64(limitB)) * 100
			if pctVal > 100 {
				pctVal = 100
			}
			pct = int(pctVal + 0.5)
		} else {
			total, _, _ := readProcMeminfo()
			if total > 0 {
				totalGB = float64(total) / (1024 * 1024)
				pctVal := (float64(usedB) / (float64(total) * 1024)) * 100
				if pctVal > 100 {
					pctVal = 100
				}
				pct = int(pctVal + 0.5)
			}
		}
		return pct, usedGB, totalGB
	}
	// Fallback: host memory from /proc/meminfo
	total, available, err := readProcMeminfo()
	if err != nil || total == 0 {
		return 0, 0, 0
	}
	used := total - available
	totalGB = float64(total) / (1024 * 1024)
	usedGB = float64(used) / (1024 * 1024)
	pctVal := (float64(used) / float64(total)) * 100
	if pctVal > 100 {
		pctVal = 100
	}
	return int(pctVal + 0.5), usedGB, totalGB
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

func nvidiaSmiPath() string {
	if p := os.Getenv("NVIDIA_SMI_PATH"); p != "" {
		return p
	}
	if p, err := exec.LookPath("nvidia-smi"); err == nil {
		return p
	}
	for _, p := range []string{"/usr/bin/nvidia-smi", "/usr/local/bin/nvidia-smi"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "nvidia-smi"
}

func collectGPUs() []GPUMetrics {
	path := nvidiaSmiPath()
	cmd := exec.Command(path, "--query-gpu=name,utilization.gpu,memory.used,memory.total", "--format=csv,noheader,nounits")
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
		vramUsedGB := memUsed / 1024
		vramTotalGB := memTotal / 1024
		gpus = append(gpus, GPUMetrics{Name: name, GPUPct: gpuPct, VRAMPct: vramPct, VRAMUsedGB: vramUsedGB, VRAMTotalGB: vramTotalGB})
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
	ramUsedGB := float64(ram) / 100 * 26
	ramTotalGB := 26.0
	system := SystemMetrics{CPUPct: cpu, RAMPct: ram, RamUsedGB: ramUsedGB, RamTotalGB: ramTotalGB, DiskIOK: diskIO}
	vramUsedGB := float64(vram) / 100 * 12
	vramTotalGB := 12.0
	gpus := []GPUMetrics{{Name: "N/A", GPUPct: gpu, VRAMPct: vram, VRAMUsedGB: vramUsedGB, VRAMTotalGB: vramTotalGB}}
	var history []HistoryPoint
	for i := 59; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * time.Second)
		ht := t.Unix()
		history = append(history, HistoryPoint{
			TS:     t.Format(time.RFC3339),
			CPU:    int(15 + (ht % 45)),
			RAM:    int(30 + (ht % 40)),
			DiskIO: int(1000 + (ht % 3000)),
			GPU:    int(5 + (ht % 60)),
			VRAM:   int(20 + (ht % 50)),
			GPUs:   []GPUHistoryPt{{GPUPct: int(5 + (ht % 60)), VRAMPct: int(20 + (ht % 50))}},
		})
	}
	return system, gpus, history
}

// GetUptimeSec returns system uptime in seconds (from /proc/uptime on Linux, 0 otherwise).
func GetUptimeSec() float64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	var uptime, idle float64
	fmt.Sscanf(string(data), "%f %f", &uptime, &idle)
	return uptime
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

	system.CPUPct, metricsStore.lastCPU, metricsStore.lastCgroup = collectCPU(&metricsStore.lastCPU, &metricsStore.lastCgroup)
	system.RAMPct, system.RamUsedGB, system.RamTotalGB = collectRAM()
	system.DiskIOK, metricsStore.lastDisk = collectDiskIO(&metricsStore.lastDisk)

	gpus = collectGPUs()
	if len(gpus) == 0 {
		gpus = metricsStore.lastGPUs
		if gpus == nil {
			gpus = []GPUMetrics{{Name: "N/A", GPUPct: 0, VRAMPct: 0, VRAMUsedGB: 0, VRAMTotalGB: 12}}
		}
	} else {
		metricsStore.lastGPUs = gpus
	}
	// Если nvidia-smi недоступен в контейнере, имя карты можно задать через GPU_NAME
	if len(gpus) > 0 {
		if name := strings.TrimSpace(os.Getenv("GPU_NAME")); name != "" && (gpus[0].Name == "" || gpus[0].Name == "N/A") {
			gpus[0].Name = name
		}
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
