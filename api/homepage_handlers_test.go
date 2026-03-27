package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestReadCPUStats verifies that readCPUStats can read and parse /proc/stat
// on Linux, returning non-zero totals and idle values within valid bounds.
func TestReadCPUStats(t *testing.T) {
	stats, err := readCPUStats()
	if err != nil {
		t.Fatalf("readCPUStats returned unexpected error: %v", err)
	}
	if stats.total == 0 {
		t.Error("expected non-zero total CPU jiffies")
	}
	if stats.idle > stats.total {
		t.Errorf("idle (%d) must not exceed total (%d)", stats.idle, stats.total)
	}
}

// TestGetCPUUsageInRange verifies that getCPUUsage returns a value in [0, 100].
func TestGetCPUUsageInRange(t *testing.T) {
	usage := getCPUUsage()
	if usage < 0 || usage > 100 {
		t.Errorf("getCPUUsage() = %d; want value in [0, 100]", usage)
	}
}

// TestGetSystemMetricsCPUInRange calls the HTTP handler and verifies the
// returned CPU field is in [0, 100].
func TestGetSystemMetricsCPUInRange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/get-server-metrics", nil)
	w := httptest.NewRecorder()

	GetSystemMetrics(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var m Metrics
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if m.CPU < 0 || m.CPU > 100 {
		t.Errorf("CPU = %d; want value in [0, 100]", m.CPU)
	}
}
