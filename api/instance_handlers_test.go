package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

// setupNginxConfig writes a temporary nginx config file and overrides the package-level
// nginxConfigPath. It returns a cleanup function that restores the original value.
func setupNginxConfig(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "opencloud")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp nginx config: %v", err)
	}
	orig := nginxConfigPath
	nginxConfigPath = cfgPath
	t.Cleanup(func() { nginxConfigPath = orig })
	return cfgPath
}

// setupReloadNginx replaces the package-level reloadNginx func with a no-op and
// returns a cleanup function that restores the original.
func setupReloadNginx(t *testing.T, reloadErr error) {
	t.Helper()
	orig := reloadNginx
	reloadNginx = func() error { return reloadErr }
	t.Cleanup(func() { reloadNginx = orig })
}

// saveLedgerState reads the current service ledger and registers a t.Cleanup that
// restores it when the test finishes, preventing test state from leaking.
func saveLedgerState(t *testing.T) {
	t.Helper()
	origLedger, err := service_ledger.ReadServiceLedger()
	if err != nil {
		// Ledger does not exist yet; clean up any file created during the test.
		t.Cleanup(func() {
			// Best-effort: nothing useful to restore.
		})
		return
	}
	t.Cleanup(func() {
		if writeErr := service_ledger.WriteServiceLedger(origLedger); writeErr != nil {
			t.Logf("saveLedgerState: failed to restore service ledger: %v", writeErr)
		}
	})
}

// sampleNginxConf is a minimal nginx config containing a server_name directive.
const sampleNginxConf = `server {
    listen 80;
    server_name _;
    location / {
        proxy_pass http://localhost:3000;
    }
}
`

// TestIsValidDomain checks the domain validation helper for a range of inputs.
func TestIsValidDomain(t *testing.T) {
	cases := []struct {
		domain string
		want   bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"my-server.example.co.uk", true},
		{"192.168.1.1", true},
		{"_", true},
		{"*.example.com", true},
		{"::1", false},      // starts with colon
		{"", false},         // empty
		{"a b.com", false},  // space
		{"a;b.com", false},  // semicolon
		{"a\nb.com", false}, // newline – injection risk
		{string(make([]byte, 254)), false}, // too long
	}

	for _, tc := range cases {
		got := isValidDomain(tc.domain)
		if got != tc.want {
			t.Errorf("isValidDomain(%q) = %v; want %v", tc.domain, got, tc.want)
		}
	}
}

// TestGetInstanceDomainHandlerMethodNotAllowed verifies that non-GET requests are rejected.
func TestGetInstanceDomainHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/get-instance-domain", nil)
	w := httptest.NewRecorder()
	GetInstanceDomainHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestSetInstanceDomainHandlerMethodNotAllowed verifies that non-POST requests are rejected.
func TestSetInstanceDomainHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/set-instance-domain", nil)
	w := httptest.NewRecorder()
	SetInstanceDomainHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestSetInstanceDomainHandlerMissingDomain verifies that an empty domain returns 400.
func TestSetInstanceDomainHandlerMissingDomain(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"domain": ""})
	req := httptest.NewRequest(http.MethodPost, "/set-instance-domain", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	SetInstanceDomainHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestSetInstanceDomainHandlerInvalidDomain verifies that invalid domain values return 400.
func TestSetInstanceDomainHandlerInvalidDomain(t *testing.T) {
	cases := []string{"a b.com", "a;b", "bad\ndomain"}
	for _, d := range cases {
		body, _ := json.Marshal(map[string]string{"domain": d})
		req := httptest.NewRequest(http.MethodPost, "/set-instance-domain", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		SetInstanceDomainHandler(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("domain %q: expected 400, got %d", d, w.Code)
		}
	}
}

// TestSetInstanceDomainHandlerSuccess verifies a successful domain update round-trip.
func TestSetInstanceDomainHandlerSuccess(t *testing.T) {
	cfgPath := setupNginxConfig(t, sampleNginxConf)
	setupReloadNginx(t, nil)
	saveLedgerState(t)

	body, _ := json.Marshal(map[string]string{"domain": "cloud.example.com"})
	req := httptest.NewRequest(http.MethodPost, "/set-instance-domain", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	SetInstanceDomainHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the nginx config file was updated.
	cfgBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read nginx config: %v", err)
	}
	if !bytes.Contains(cfgBytes, []byte("server_name cloud.example.com;")) {
		t.Errorf("nginx config not updated; got:\n%s", string(cfgBytes))
	}

	// Verify the response body.
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["domain"] != "cloud.example.com" {
		t.Errorf("response domain = %q; want %q", resp["domain"], "cloud.example.com")
	}
}

// TestGetInstanceDomainHandlerReturnsStoredDomain verifies that after saving a domain
// the GET handler returns it.
func TestGetInstanceDomainHandlerReturnsStoredDomain(t *testing.T) {
	setupNginxConfig(t, sampleNginxConf)
	setupReloadNginx(t, nil)
	saveLedgerState(t)

	// First, set the domain via the handler.
	setBody, _ := json.Marshal(map[string]string{"domain": "myinstance.example.com"})
	setReq := httptest.NewRequest(http.MethodPost, "/set-instance-domain", bytes.NewReader(setBody))
	setReq.Header.Set("Content-Type", "application/json")
	setW := httptest.NewRecorder()
	SetInstanceDomainHandler(setW, setReq)
	if setW.Code != http.StatusOK {
		t.Fatalf("set domain: expected 200, got %d: %s", setW.Code, setW.Body.String())
	}

	// Now retrieve it via the GET handler.
	getReq := httptest.NewRequest(http.MethodGet, "/get-instance-domain", nil)
	getW := httptest.NewRecorder()
	GetInstanceDomainHandler(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get domain: expected 200, got %d: %s", getW.Code, getW.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(getW.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["domain"] != "myinstance.example.com" {
		t.Errorf("get domain = %q; want %q", resp["domain"], "myinstance.example.com")
	}
}

// TestUpdateNginxServerName verifies that the server_name line is replaced correctly.
func TestUpdateNginxServerName(t *testing.T) {
	cfgPath := setupNginxConfig(t, sampleNginxConf)

	if err := updateNginxServerName("example.com"); err != nil {
		t.Fatalf("updateNginxServerName returned error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read nginx config: %v", err)
	}

	if !bytes.Contains(data, []byte("server_name example.com;")) {
		t.Errorf("expected server_name example.com; in config, got:\n%s", string(data))
	}
}

// TestUpdateNginxServerNameMissingFile verifies that a missing config file returns an error.
func TestUpdateNginxServerNameMissingFile(t *testing.T) {
	orig := nginxConfigPath
	nginxConfigPath = "/nonexistent/path/opencloud"
	t.Cleanup(func() { nginxConfigPath = orig })

	err := updateNginxServerName("example.com")
	if err == nil {
		t.Fatal("expected error for missing nginx config file, got nil")
	}
}

