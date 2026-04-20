package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

// saveLedgerState reads the current service ledger and registers a t.Cleanup that
// restores it when the test finishes, preventing test state from leaking.
func saveLedgerState(t *testing.T) {
	t.Helper()
	origLedger, err := service_ledger.ReadServiceLedger()
	if err != nil {
		// Ledger does not exist yet; nothing to restore on cleanup.
		return
	}
	t.Cleanup(func() {
		if writeErr := service_ledger.WriteServiceLedger(origLedger); writeErr != nil {
			t.Logf("saveLedgerState: failed to restore service ledger: %v", writeErr)
		}
	})
}

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
		{"exam*ple.com", false}, // asterisk only allowed at start
		{"example*.com", false}, // asterisk only allowed at start
		{"", false},             // empty
		{"a b.com", false},      // space
		{"a;b.com", false},      // semicolon
		{"a\nb.com", false},     // newline – injection risk
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

// TestSetInstanceDomainHandlerSuccess verifies a successful domain save and checks that
// the response contains nginx configuration instructions.
func TestSetInstanceDomainHandlerSuccess(t *testing.T) {
	saveLedgerState(t)

	body, _ := json.Marshal(map[string]string{"domain": "cloud.example.com"})
	req := httptest.NewRequest(http.MethodPost, "/set-instance-domain", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	SetInstanceDomainHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp SetInstanceDomainResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if resp.Domain != "cloud.example.com" {
		t.Errorf("response domain = %q; want %q", resp.Domain, "cloud.example.com")
	}
	if !strings.Contains(resp.NginxEditCmd, "sudo") {
		t.Errorf("NginxEditCmd should contain sudo; got: %q", resp.NginxEditCmd)
	}
	if !strings.Contains(resp.NginxEditCmd, "/etc/nginx/sites-available/opencloud") {
		t.Errorf("NginxEditCmd should contain the config path; got: %q", resp.NginxEditCmd)
	}
	if resp.NginxConfigLine != "server_name cloud.example.com;" {
		t.Errorf("NginxConfigLine = %q; want %q", resp.NginxConfigLine, "server_name cloud.example.com;")
	}
	if resp.NginxReloadCmd == "" {
		t.Error("NginxReloadCmd should not be empty")
	}
	if !strings.Contains(resp.Instructions, "cloud.example.com") {
		t.Errorf("Instructions should mention the domain; got: %s", resp.Instructions)
	}
}

// TestGetInstanceDomainHandlerReturnsStoredDomain verifies that after saving a domain
// the GET handler returns it.
func TestGetInstanceDomainHandlerReturnsStoredDomain(t *testing.T) {
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

// TestBuildNginxInstructions verifies that the instructions mention the domain and key steps.
func TestBuildNginxInstructions(t *testing.T) {
	domain := "example.com"
	instructions := buildNginxInstructions(domain)

	checks := []string{
		domain,
		"/etc/nginx/sites-available/opencloud",
		"sudo nginx -t",
		"sudo systemctl reload nginx",
	}
	for _, check := range checks {
		if !strings.Contains(instructions, check) {
			t.Errorf("instructions missing %q; got:\n%s", check, instructions)
		}
	}
}

// TestIsValidEmail checks the email validation helper.
func TestIsValidEmail(t *testing.T) {
	cases := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"user.name+tag@sub.example.co.uk", true},
		{"admin@localhost.example", true},
		{"", false},
		{"notanemail", false},
		{"@example.com", false},
		{"user@", false},
		{"user@.com", false},
	}

	for _, tc := range cases {
		got := isValidEmail(tc.email)
		if got != tc.want {
			t.Errorf("isValidEmail(%q) = %v; want %v", tc.email, got, tc.want)
		}
	}
}

// TestBuildCertbotInstructions verifies that the certbot instructions contain
// the domain, email, and key certbot steps.
func TestBuildCertbotInstructions(t *testing.T) {
	domain := "cloud.example.com"
	email := "admin@example.com"
	instructions := buildCertbotInstructions(domain, email)

	checks := []string{
		domain,
		email,
		"certbot",
		"--nginx",
		"sudo certbot renew --dry-run",
		"sudo systemctl reload nginx",
	}
	for _, check := range checks {
		if !strings.Contains(instructions, check) {
			t.Errorf("certbot instructions missing %q; got:\n%s", check, instructions)
		}
	}
}

// TestGetSSLStatusHandlerMethodNotAllowed verifies that non-GET requests are rejected.
func TestGetSSLStatusHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/get-ssl-status", nil)
	w := httptest.NewRecorder()
	GetSSLStatusHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestConfigureSSLHandlerMethodNotAllowed verifies that non-POST requests are rejected.
func TestConfigureSSLHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/configure-ssl", nil)
	w := httptest.NewRecorder()
	ConfigureSSLHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestConfigureSSLHandlerMissingDomain verifies that a missing domain returns 400.
func TestConfigureSSLHandlerMissingDomain(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"domain":     "",
		"email":      "admin@example.com",
		"agreeToTos": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/configure-ssl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ConfigureSSLHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestConfigureSSLHandlerInvalidDomain verifies that an invalid domain returns 400.
func TestConfigureSSLHandlerInvalidDomain(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"domain":     "bad domain!",
		"email":      "admin@example.com",
		"agreeToTos": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/configure-ssl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ConfigureSSLHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestConfigureSSLHandlerMissingEmail verifies that a missing email returns 400.
func TestConfigureSSLHandlerMissingEmail(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"domain":     "cloud.example.com",
		"email":      "",
		"agreeToTos": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/configure-ssl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ConfigureSSLHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestConfigureSSLHandlerInvalidEmail verifies that an invalid email returns 400.
func TestConfigureSSLHandlerInvalidEmail(t *testing.T) {
	cases := []string{"notanemail", "@example.com", "user@", ""}
	for _, e := range cases {
		body, _ := json.Marshal(map[string]interface{}{
			"domain":     "cloud.example.com",
			"email":      e,
			"agreeToTos": true,
		})
		req := httptest.NewRequest(http.MethodPost, "/configure-ssl", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		ConfigureSSLHandler(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("email %q: expected 400, got %d", e, w.Code)
		}
	}
}

// TestConfigureSSLHandlerTosNotAgreed verifies that failing to agree to ToS returns 400.
func TestConfigureSSLHandlerTosNotAgreed(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"domain":     "cloud.example.com",
		"email":      "admin@example.com",
		"agreeToTos": false,
	})
	req := httptest.NewRequest(http.MethodPost, "/configure-ssl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ConfigureSSLHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestConfigureSSLHandlerSuccess verifies a successful SSL configuration request.
func TestConfigureSSLHandlerSuccess(t *testing.T) {
	saveLedgerState(t)

	body, _ := json.Marshal(map[string]interface{}{
		"domain":     "cloud.example.com",
		"email":      "admin@example.com",
		"agreeToTos": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/configure-ssl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ConfigureSSLHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ConfigureSSLResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if resp.Domain != "cloud.example.com" {
		t.Errorf("response domain = %q; want %q", resp.Domain, "cloud.example.com")
	}
	if resp.Email != "admin@example.com" {
		t.Errorf("response email = %q; want %q", resp.Email, "admin@example.com")
	}
	if !strings.Contains(resp.CertbotCmd, "certbot") {
		t.Errorf("CertbotCmd should contain certbot; got: %q", resp.CertbotCmd)
	}
	if !strings.Contains(resp.CertbotCmd, "cloud.example.com") {
		t.Errorf("CertbotCmd should contain domain; got: %q", resp.CertbotCmd)
	}
	if !strings.Contains(resp.CertbotCmd, "admin@example.com") {
		t.Errorf("CertbotCmd should contain email; got: %q", resp.CertbotCmd)
	}
	if resp.CertbotInstallCmd == "" {
		t.Error("CertbotInstallCmd should not be empty")
	}
	if resp.AutoRenewCmd == "" {
		t.Error("AutoRenewCmd should not be empty")
	}
	if !strings.Contains(resp.Instructions, "cloud.example.com") {
		t.Errorf("Instructions should mention the domain; got:\n%s", resp.Instructions)
	}
}

// TestGetSSLStatusHandlerReturnsStoredEmail verifies that after configuring SSL
// the GET handler returns the stored email.
func TestGetSSLStatusHandlerReturnsStoredEmail(t *testing.T) {
	saveLedgerState(t)

	// Configure SSL first.
	setBody, _ := json.Marshal(map[string]interface{}{
		"domain":     "cloud.example.com",
		"email":      "ssl@example.com",
		"agreeToTos": true,
	})
	setReq := httptest.NewRequest(http.MethodPost, "/configure-ssl", bytes.NewReader(setBody))
	setReq.Header.Set("Content-Type", "application/json")
	setW := httptest.NewRecorder()
	ConfigureSSLHandler(setW, setReq)
	if setW.Code != http.StatusOK {
		t.Fatalf("configure SSL: expected 200, got %d: %s", setW.Code, setW.Body.String())
	}

	// Retrieve SSL status.
	getReq := httptest.NewRequest(http.MethodGet, "/get-ssl-status", nil)
	getW := httptest.NewRecorder()
	GetSSLStatusHandler(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get SSL status: expected 200, got %d: %s", getW.Code, getW.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(getW.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["email"] != "ssl@example.com" {
		t.Errorf("get ssl email = %q; want %q", resp["email"], "ssl@example.com")
	}
}
