package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetSSLStatus_NoCerts verifies that GetSSLStatus returns configured=false
// when the Let's Encrypt directory does not exist.
func TestGetSSLStatus_NoCerts(t *testing.T) {
	// Point to a non-existent directory so no certs are found.
	origDir := letsEncryptLiveDir
	letsEncryptLiveDir = filepath.Join(t.TempDir(), "nonexistent")
	defer func() { letsEncryptLiveDir = origDir }()

	req := httptest.NewRequest(http.MethodGet, "/ssl-status", nil)
	w := httptest.NewRecorder()

	GetSSLStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var status SSLStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if status.Configured {
		t.Error("expected Configured=false when no certs exist")
	}
	if len(status.Domains) != 0 {
		t.Errorf("expected no domains, got %v", status.Domains)
	}
}

// TestGetSSLStatus_WithCerts verifies that GetSSLStatus returns configured=true
// and lists the domain directories when certificate directories exist.
func TestGetSSLStatus_WithCerts(t *testing.T) {
	// Create a temporary directory tree that mimics /etc/letsencrypt/live/.
	liveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(liveDir, "example.com"), 0755); err != nil {
		t.Fatalf("failed to create fake cert dir: %v", err)
	}
	// The README file that certbot creates should be ignored.
	if err := os.WriteFile(filepath.Join(liveDir, "README"), []byte("readme"), 0644); err != nil {
		t.Fatalf("failed to create fake README: %v", err)
	}

	origDir := letsEncryptLiveDir
	letsEncryptLiveDir = liveDir
	defer func() { letsEncryptLiveDir = origDir }()

	req := httptest.NewRequest(http.MethodGet, "/ssl-status", nil)
	w := httptest.NewRecorder()

	GetSSLStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var status SSLStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !status.Configured {
		t.Error("expected Configured=true when cert directory exists")
	}
	if len(status.Domains) != 1 || status.Domains[0] != "example.com" {
		t.Errorf("expected domains=[example.com], got %v", status.Domains)
	}
}

// TestGetSSLStatus_MethodNotAllowed verifies that non-GET requests receive 405.
func TestGetSSLStatus_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ssl-status", nil)
	w := httptest.NewRecorder()

	GetSSLStatus(w, req)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Result().StatusCode)
	}
}

// TestConfigureSSLStream_MethodNotAllowed verifies that non-POST requests get 405.
func TestConfigureSSLStream_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/configure-ssl-stream", nil)
	w := httptest.NewRecorder()

	ConfigureSSLStream(w, req)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Result().StatusCode)
	}
}

// TestConfigureSSLStream_MissingFields verifies that requests with missing
// required fields receive 400 Bad Request.
func TestConfigureSSLStream_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing domain", `{"email":"a@b.com","agree_tos":true}`},
		{"missing email", `{"domain":"example.com","agree_tos":true}`},
		{"tos not agreed", `{"domain":"example.com","email":"a@b.com","agree_tos":false}`},
		{"empty body", `{}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/configure-ssl-stream",
				strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			ConfigureSSLStream(w, req)

			if w.Result().StatusCode != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", tc.name, w.Result().StatusCode)
			}
		})
	}
}

// TestConfigureSSLStream_InvalidJSON verifies that malformed JSON bodies receive 400.
func TestConfigureSSLStream_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/configure-ssl-stream",
		strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ConfigureSSLStream(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Result().StatusCode)
	}
}

// TestConfigureSSLStream_Success verifies that ConfigureSSLStream streams
// installer output and sends the "done" event when the script succeeds.
func TestConfigureSSLStream_Success(t *testing.T) {
	// Override configureSSLCmd to run a simple echo script instead of certbot.
	origCmd := configureSSLCmd
	configureSSLCmd = func(domain, email string) *exec.Cmd {
		return exec.Command("sh", "-c",
			"echo [INFO] fake certbot running; echo [SUCCESS] done")
	}
	defer func() { configureSSLCmd = origCmd }()

	body := `{"domain":"test.example.com","email":"admin@example.com","agree_tos":true}`
	req := httptest.NewRequest(http.MethodPost, "/configure-ssl-stream",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ConfigureSSLStream(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	responseBody := w.Body.String()
	if !strings.Contains(responseBody, "event: done") {
		t.Errorf("expected 'event: done' in response, got:\n%s", responseBody)
	}
	if !strings.Contains(responseBody, "test.example.com") {
		t.Errorf("expected domain in done event, got:\n%s", responseBody)
	}
}

// TestConfigureSSLStream_Failure verifies that ConfigureSSLStream streams an
// "error" event when the installer script exits with a non-zero status.
func TestConfigureSSLStream_Failure(t *testing.T) {
	origCmd := configureSSLCmd
	configureSSLCmd = func(domain, email string) *exec.Cmd {
		return exec.Command("sh", "-c", "echo [ERROR] something went wrong; exit 1")
	}
	defer func() { configureSSLCmd = origCmd }()

	body := `{"domain":"fail.example.com","email":"admin@example.com","agree_tos":true}`
	req := httptest.NewRequest(http.MethodPost, "/configure-ssl-stream",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ConfigureSSLStream(w, req)

	responseBody := w.Body.String()
	if !strings.Contains(responseBody, "event: error") {
		t.Errorf("expected 'event: error' in response, got:\n%s", responseBody)
	}
}
