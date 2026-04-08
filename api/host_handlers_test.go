package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- GetHostInfo tests -------------------------------------------------------

// TestGetHostInfoMethodNotAllowed verifies that POST requests are rejected.
func TestGetHostInfoMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/host/info", nil)
	w := httptest.NewRecorder()

	GetHostInfo(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestGetHostInfoReturnsJSON verifies that the endpoint returns a valid JSON
// payload containing at least user, hostname, and cwd fields.
func TestGetHostInfoReturnsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/host/info", nil)
	w := httptest.NewRecorder()

	GetHostInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d — body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var info HostInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if info.User == "" {
		t.Error("expected non-empty user")
	}
	if info.Hostname == "" {
		t.Error("expected non-empty hostname")
	}
	if info.Cwd == "" {
		t.Error("expected non-empty cwd")
	}
}

