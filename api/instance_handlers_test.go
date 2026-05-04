package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

// setupTempHome creates a temporary home directory with the .opencloud/user
// subdirectory and redirects HOME to it.  It registers a t.Cleanup that
// restores the original HOME value.
func setupTempHome(t *testing.T) {
	t.Helper()
	tmpHome := t.TempDir()
	if err := os.MkdirAll(tmpHome+"/.opencloud/user", 0700); err != nil {
		t.Fatalf("setupTempHome: %v", err)
	}
	orig := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", orig) })
}

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

// TestGenerateCLITokenHandlerMethodNotAllowed verifies that non-POST requests are rejected.
func TestGenerateCLITokenHandlerMethodNotAllowed(t *testing.T) {
	setupTempHome(t)
	req := httptest.NewRequest(http.MethodGet, "/generate-cli-token", nil)
	w := httptest.NewRecorder()
	GenerateCLITokenHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestGenerateCLITokenHandlerSuccess verifies that the handler returns a non-empty
// hex-encoded token with an ID, stores a hash on disk, and that two successive
// calls both remain valid (multi-token support).
func TestGenerateCLITokenHandlerSuccess(t *testing.T) {
	setupTempHome(t)

	req := httptest.NewRequest(http.MethodPost, "/generate-cli-token", nil)
	w := httptest.NewRecorder()
	GenerateCLITokenHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp GenerateCLITokenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if resp.Token == "" {
		t.Error("token should not be empty")
	}
	// A 32-byte random value hex-encoded is always 64 characters.
	if len(resp.Token) != 64 {
		t.Errorf("token length = %d; want 64", len(resp.Token))
	}
	// The token must be valid hex.
	if !isHex(resp.Token) {
		t.Errorf("token is not valid hex: %q", resp.Token)
	}
	// Response must include a non-empty ID and creation timestamp.
	if resp.ID == "" {
		t.Error("response ID should not be empty")
	}
	if resp.CreatedAt == "" {
		t.Error("response CreatedAt should not be empty")
	}

	// The token hash must have been persisted on disk and be verifiable.
	ok, err := VerifyCLIToken(resp.Token)
	if err != nil {
		t.Fatalf("VerifyCLIToken error: %v", err)
	}
	if !ok {
		t.Error("generated token should verify against the stored hash")
	}

	// A second call should produce a different token.
	req2 := httptest.NewRequest(http.MethodPost, "/generate-cli-token", nil)
	w2 := httptest.NewRecorder()
	GenerateCLITokenHandler(w2, req2)

	var resp2 GenerateCLITokenResponse
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("second call: invalid JSON response: %v", err)
	}
	if resp.Token == resp2.Token {
		t.Error("two successive calls returned the same token; expected unique tokens")
	}
	if resp.ID == resp2.ID {
		t.Error("two successive calls returned the same ID; expected unique IDs")
	}

	// With multi-token support, the FIRST token must still be valid after a second is generated.
	ok, err = VerifyCLIToken(resp.Token)
	if err != nil {
		t.Fatalf("VerifyCLIToken (first token after second generated) error: %v", err)
	}
	if !ok {
		t.Error("first token should still be valid after a second token is generated")
	}
}

// isHex returns true if every character in s is a valid lowercase hex digit.
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// TestVerifyCLITokenNoFile verifies that VerifyCLIToken returns false (not an
// error) when no CLI token has ever been generated.
func TestVerifyCLITokenNoFile(t *testing.T) {
	setupTempHome(t)
	ok, err := VerifyCLIToken("anything")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if ok {
		t.Error("expected false when no token file exists")
	}
}

// TestStoreCLITokenHashAndVerify verifies round-trip storage and verification
// for multiple tokens simultaneously.
func TestStoreCLITokenHashAndVerify(t *testing.T) {
	setupTempHome(t)

	token1 := "mysecrettoken1"
	id1, createdAt1, err := StoreCLITokenHash(token1)
	if err != nil {
		t.Fatalf("StoreCLITokenHash (token1): %v", err)
	}
	if id1 == "" {
		t.Error("id1 should not be empty")
	}
	if createdAt1 == "" {
		t.Error("createdAt1 should not be empty")
	}

	token2 := "mysecrettoken2"
	id2, _, err := StoreCLITokenHash(token2)
	if err != nil {
		t.Fatalf("StoreCLITokenHash (token2): %v", err)
	}
	if id1 == id2 {
		t.Error("two tokens must have distinct IDs")
	}

	// Both tokens must verify.
	for _, tok := range []string{token1, token2} {
		ok, err := VerifyCLIToken(tok)
		if err != nil {
			t.Fatalf("VerifyCLIToken(%q): %v", tok, err)
		}
		if !ok {
			t.Errorf("expected token %q to verify", tok)
		}
	}

	// Wrong token must not verify.
	ok, err := VerifyCLIToken("wrongtoken")
	if err != nil {
		t.Fatalf("VerifyCLIToken (wrong): %v", err)
	}
	if ok {
		t.Error("wrong token must not verify")
	}
}

// TestListCLITokensHandler verifies that the list endpoint returns the correct
// metadata after tokens are generated.
func TestListCLITokensHandler(t *testing.T) {
	setupTempHome(t)

	// Generate two tokens.
	req1 := httptest.NewRequest(http.MethodPost, "/generate-cli-token", nil)
	GenerateCLITokenHandler(httptest.NewRecorder(), req1)
	req2 := httptest.NewRequest(http.MethodPost, "/generate-cli-token", nil)
	GenerateCLITokenHandler(httptest.NewRecorder(), req2)

	// List tokens.
	listReq := httptest.NewRequest(http.MethodGet, "/list-cli-tokens", nil)
	listW := httptest.NewRecorder()
	ListCLITokensHandler(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listW.Code, listW.Body.String())
	}

	var meta []cliTokenMeta
	if err := json.NewDecoder(listW.Body).Decode(&meta); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(meta) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(meta))
	}
	for _, m := range meta {
		if m.ID == "" {
			t.Error("token meta ID should not be empty")
		}
		if m.CreatedAt == "" {
			t.Error("token meta CreatedAt should not be empty")
		}
	}
}

// TestListCLITokensHandlerMethodNotAllowed verifies that non-GET requests are rejected.
func TestListCLITokensHandlerMethodNotAllowed(t *testing.T) {
	setupTempHome(t)
	req := httptest.NewRequest(http.MethodPost, "/list-cli-tokens", nil)
	w := httptest.NewRecorder()
	ListCLITokensHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestRevokeCLITokenHandler verifies that revoking a token removes it from the
// store and makes it unverifiable, while leaving other tokens intact.
func TestRevokeCLITokenHandler(t *testing.T) {
	setupTempHome(t)

	// Generate two tokens via the handler.
	genReq1 := httptest.NewRequest(http.MethodPost, "/generate-cli-token", nil)
	genW1 := httptest.NewRecorder()
	GenerateCLITokenHandler(genW1, genReq1)
	var genResp1 GenerateCLITokenResponse
	json.NewDecoder(genW1.Body).Decode(&genResp1)

	genReq2 := httptest.NewRequest(http.MethodPost, "/generate-cli-token", nil)
	genW2 := httptest.NewRecorder()
	GenerateCLITokenHandler(genW2, genReq2)
	var genResp2 GenerateCLITokenResponse
	json.NewDecoder(genW2.Body).Decode(&genResp2)

	// Revoke the first token.
	revokeReq := httptest.NewRequest(http.MethodDelete, "/revoke-cli-token/"+genResp1.ID, nil)
	revokeReq.RequestURI = "/revoke-cli-token/" + genResp1.ID
	revokeW := httptest.NewRecorder()
	RevokeCLITokenHandler(revokeW, revokeReq)
	if revokeW.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", revokeW.Code, revokeW.Body.String())
	}

	// First token must no longer verify.
	ok, err := VerifyCLIToken(genResp1.Token)
	if err != nil {
		t.Fatalf("VerifyCLIToken (revoked): %v", err)
	}
	if ok {
		t.Error("revoked token should not verify")
	}

	// Second token must still verify.
	ok, err = VerifyCLIToken(genResp2.Token)
	if err != nil {
		t.Fatalf("VerifyCLIToken (remaining): %v", err)
	}
	if !ok {
		t.Error("non-revoked token should still verify")
	}
}

// TestRevokeCLITokenHandlerNotFound verifies that revoking a non-existent ID
// returns 404.
func TestRevokeCLITokenHandlerNotFound(t *testing.T) {
	setupTempHome(t)
	req := httptest.NewRequest(http.MethodDelete, "/revoke-cli-token/doesnotexist", nil)
	req.RequestURI = "/revoke-cli-token/doesnotexist"
	w := httptest.NewRecorder()
	RevokeCLITokenHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestRevokeCLITokenHandlerMethodNotAllowed verifies that non-DELETE requests are rejected.
func TestRevokeCLITokenHandlerMethodNotAllowed(t *testing.T) {
	setupTempHome(t)
	req := httptest.NewRequest(http.MethodGet, "/revoke-cli-token/someid", nil)
	w := httptest.NewRecorder()
	RevokeCLITokenHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestWithCLITokenAuthNoHeader verifies that requests without an Authorization
// header pass through untouched.
func TestWithCLITokenAuthNoHeader(t *testing.T) {
	setupTempHome(t)

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := WithCLITokenAuth(inner)

	req := httptest.NewRequest(http.MethodGet, "/some-endpoint", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("inner handler should have been called when no Authorization header is present")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestWithCLITokenAuthValidToken verifies that a request with a valid CLI token
// in the Basic auth username field passes through to the inner handler.
func TestWithCLITokenAuthValidToken(t *testing.T) {
	setupTempHome(t)

	token := "validtoken123"
	if _, _, err := StoreCLITokenHash(token); err != nil {
		t.Fatalf("StoreCLITokenHash: %v", err)
	}

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := WithCLITokenAuth(inner)

	req := httptest.NewRequest(http.MethodGet, "/some-endpoint", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(token+":")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("inner handler should have been called for a valid CLI token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestWithCLITokenAuthInvalidToken verifies that a request with an incorrect token
// is rejected with 401.
func TestWithCLITokenAuthInvalidToken(t *testing.T) {
	setupTempHome(t)

	if _, _, err := StoreCLITokenHash("correcttoken"); err != nil {
		t.Fatalf("StoreCLITokenHash: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WithCLITokenAuth(inner)

	req := httptest.NewRequest(http.MethodGet, "/some-endpoint", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("wrongtoken:")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestWithCLITokenAuthMalformedHeader verifies that a malformed Authorization
// header returns 401.
func TestWithCLITokenAuthMalformedHeader(t *testing.T) {
	setupTempHome(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WithCLITokenAuth(inner)

	// Non-Basic scheme
	req := httptest.NewRequest(http.MethodGet, "/some-endpoint", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for non-Basic scheme, got %d", w.Code)
	}
}

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

// TestBuildCertbotInstructions verifies that the certbot instructions contain
// the domain and key certbot steps.
func TestBuildCertbotInstructions(t *testing.T) {
	domain := "cloud.example.com"
	instructions := buildCertbotInstructions(domain)

	checks := []string{
		domain,
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
	// Email and flags should NOT appear in the generated command — certbot prompts interactively.
	if strings.Contains(instructions, "--email") {
		t.Errorf("certbot instructions should not contain --email; got:\n%s", instructions)
	}
	if strings.Contains(instructions, "--agree-tos") {
		t.Errorf("certbot instructions should not contain --agree-tos; got:\n%s", instructions)
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
		"domain": "",
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
		"domain": "bad domain!",
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
// Note: saveLedgerState is not needed here because ConfigureSSLHandler no longer
// writes to the service ledger — it only returns certbot commands.
func TestConfigureSSLHandlerSuccess(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"domain": "cloud.example.com",
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
	if !strings.Contains(resp.CertbotCmd, "certbot") {
		t.Errorf("CertbotCmd should contain certbot; got: %q", resp.CertbotCmd)
	}
	if !strings.Contains(resp.CertbotCmd, "cloud.example.com") {
		t.Errorf("CertbotCmd should contain domain; got: %q", resp.CertbotCmd)
	}
	// Email and agree-tos flags should not be present — certbot prompts interactively.
	if strings.Contains(resp.CertbotCmd, "--email") {
		t.Errorf("CertbotCmd should not contain --email; got: %q", resp.CertbotCmd)
	}
	if strings.Contains(resp.CertbotCmd, "--agree-tos") {
		t.Errorf("CertbotCmd should not contain --agree-tos; got: %q", resp.CertbotCmd)
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
