package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type Storage struct {
	UsedStorage      string `json:"UsedStorage"`
	AvailableStorage string `json:"AvailableStorage"`
	TotalStorage string `json:"TotalStorage"`
	PercentageUsed   string `json:"PercentageUsed"`
}

type Metrics struct {
	Storage Storage `json:"STORAGE"`
	CPU     int     `json:"CPU"`
	Memory  int     `json:"MEMORY"`
}

// cpuStats holds the idle and total CPU jiffies read from /proc/stat.
type cpuStats struct {
	idle  uint64
	total uint64
}

// readCPUStats reads the aggregate CPU line from /proc/stat and returns idle
// and total jiffies. It returns an error if the file cannot be read or parsed.
func readCPUStats() (cpuStats, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStats{}, fmt.Errorf("reading /proc/stat: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		// fields: cpu user nice system idle iowait irq softirq [steal ...]
		if len(fields) < 5 {
			return cpuStats{}, fmt.Errorf("unexpected /proc/stat format")
		}
		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			v, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				return cpuStats{}, fmt.Errorf("parsing /proc/stat field %d: %w", i, err)
			}
			total += v
			if i == 4 { // idle column
				idle = v
			}
		}
		return cpuStats{idle: idle, total: total}, nil
	}
	return cpuStats{}, fmt.Errorf("cpu line not found in /proc/stat")
}

// cachedCPU holds the most recently computed CPU usage percentage and is
// refreshed in the background by startCPUPoller so that HTTP handlers never
// block on a sampling sleep.
var (
	cachedCPUMu    sync.Mutex
	cachedCPUValue int
	cpuPollerOnce  sync.Once
)

// startCPUPoller launches a single background goroutine that refreshes the
// cached CPU usage every 5 seconds. It is safe to call multiple times.
func startCPUPoller() {
	cpuPollerOnce.Do(func() {
		go func() {
			prev, err := readCPUStats()
			if err != nil {
				fmt.Fprintf(os.Stderr, "CPU poller: initial read error: %v\n", err)
			}
			for {
				time.Sleep(5 * time.Second)
				cur, err := readCPUStats()
				if err != nil {
					fmt.Fprintf(os.Stderr, "CPU poller: read error: %v\n", err)
					continue
				}
				totalDelta := cur.total - prev.total
				var pct int
				if totalDelta > 0 {
					idleDelta := cur.idle - prev.idle
					pct = int(100 * (totalDelta - idleDelta) / totalDelta)
				}
				cachedCPUMu.Lock()
				cachedCPUValue = pct
				cachedCPUMu.Unlock()
				prev = cur
			}
		}()
	})
}

// getCPUUsage returns the most recently cached CPU usage percentage. It starts
// the background poller on first call. On startup (before the first 5-second
// interval has elapsed) it falls back to a single short sample so the initial
// page load always sees a real value rather than 0.
func getCPUUsage() int {
	startCPUPoller()

	cachedCPUMu.Lock()
	v := cachedCPUValue
	cachedCPUMu.Unlock()

	if v != 0 {
		return v
	}

	// Poller has not yet completed its first cycle; take a quick sample.
	s1, err := readCPUStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading CPU stats: %v\n", err)
		return 0
	}
	time.Sleep(200 * time.Millisecond)
	s2, err := readCPUStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading CPU stats: %v\n", err)
		return 0
	}
	totalDelta := s2.total - s1.total
	if totalDelta == 0 {
		return 0
	}
	idleDelta := s2.idle - s1.idle
	return int(100 * (totalDelta - idleDelta) / totalDelta)
}

func getStorageMetrics() (float64, float64, float64) {
	/*
		getStorageMetrics retrieves disk usage statistics for the current working directory.

		Returns:
  			- usedGB:        The amount of storage currently used (in gigabytes)
  			- availableGB:   The amount of storage available (in gigabytes)
  			- totalGB:       The total storage capacity (in gigabytes)

		The function uses the unix.Statfs system call to gather filesystem information.
		If an error occurs (for example, failing to get the working directory or filesystem stats),
		the function prints the error to stderr and returns zeros for all values.
	*/

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting pwd: %v\n", err)
		return 0, 0, 0
	}

	var statfs unix.Statfs_t
	err = unix.Statfs(wd, &statfs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting statfs for %s: %v\n", wd, err)
		return 0, 0, 0
	}

	// Convert free bytes to GB with decimals
	freeBytes := float64(statfs.Bavail) * float64(statfs.Bsize)
	availableStorage := freeBytes / (1000 * 1000 * 1000) // divide by GiB (binary GB)	

	totalStorage := (float64(statfs.Blocks) * float64(statfs.Bsize)) / (1000 * 1000 * 1000)
	
	return (totalStorage-availableStorage), availableStorage, totalStorage
}


func GetSystemMetrics(w http.ResponseWriter, r *http.Request) {
	/*
		This function gets system metrics to populate the dashboard with. 
		These metrics include CPU usage, Memory, and Disk Usage

		This function returns a json payload of the metrics it collects
	*/

	usedStorage, availableStorage, totalStorage := getStorageMetrics()

	ret := Metrics{
		Storage: Storage{
			UsedStorage:      	fmt.Sprintf("%.2f", usedStorage),
			AvailableStorage: 	fmt.Sprintf("%.2f", availableStorage),
			TotalStorage:   	fmt.Sprintf("%.2f", totalStorage),
			PercentageUsed:   	fmt.Sprintf("%.2f", (usedStorage/(totalStorage))*100),
		},
		CPU:    getCPUUsage(),
		Memory: 101,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(ret)

}
