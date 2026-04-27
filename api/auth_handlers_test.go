package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// setupCredentialsFile creates a temporary credentials file for testing and
// overrides the home directory to point at the temp dir.  It returns a cleanup
// function that restores the original state.
func setupCredentialsFile(t *testing.T, username, password string) func() {
	t.Helper()

	tmpHome := t.TempDir()

	// Mirror the directory layout expected by credentialsPath.
	userDir := filepath.Join(tmpHome, ".opencloud", "user")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatalf("failed to create test user dir: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash test password: %v", err)
	}

	content := username + ":" + string(hash) + "\n"
	credFile := filepath.Join(userDir, "credentials")
	if err := os.WriteFile(credFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write credentials file: %v", err)
	}

	// Point HOME at the temp directory so credentialsPath() and
	// loadTokenSecret() resolve paths inside our test tree.
	orig := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)

	return func() {
		os.Setenv("HOME", orig)
		// Clear any sessions created during the test.
		sessionStore.Lock()
		sessionStore.data = make(map[string]*userSession)
		sessionStore.Unlock()
	}
}

// TestLoginMethodNotAllowed verifies that GET requests to /user/login are rejected.
func TestLoginMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/user/login", nil)
	w := httptest.NewRecorder()

	Login(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestLoginInvalidJSON verifies that malformed JSON is rejected.
func TestLoginInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/user/login", bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestLoginMissingFields verifies that empty username/password is rejected.
func TestLoginMissingFields(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	body, _ := json.Marshal(loginRequest{Username: "", Password: ""})
	req := httptest.NewRequest(http.MethodPost, "/user/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestLoginInvalidCredentials verifies that wrong credentials return 401.
func TestLoginInvalidCredentials(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "wrongpassword"})
	req := httptest.NewRequest(http.MethodPost, "/user/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestLoginSuccess verifies that correct credentials return tokens.
func TestLoginSuccess(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "admin"})
	req := httptest.NewRequest(http.MethodPost, "/user/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d — body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp loginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh_token")
	}
}

// TestRefreshAuthMethodNotAllowed verifies that POST requests to /user/get-auth/ are rejected.
func TestRefreshAuthMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/user/get-auth/", nil)
	w := httptest.NewRecorder()

	RefreshAuth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestRefreshAuthMissingToken verifies that a missing AccessToken header returns 401.
func TestRefreshAuthMissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/user/get-auth/", nil)
	w := httptest.NewRecorder()

	RefreshAuth(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestRefreshAuthInvalidToken verifies that a tampered token returns 401.
func TestRefreshAuthInvalidToken(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/user/get-auth/", nil)
	req.Header.Set("AccessToken", "invalid.token")
	w := httptest.NewRecorder()

	RefreshAuth(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestRefreshAuthSuccess verifies the full login → refresh flow.
func TestRefreshAuthSuccess(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	// Step 1: login to obtain an access token.
	loginBody, _ := json.Marshal(loginRequest{Username: "admin", Password: "admin"})
	loginReq := httptest.NewRequest(http.MethodPost, "/user/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	Login(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("login failed: %d — %s", loginW.Code, loginW.Body.String())
	}

	var loginResp loginResponse
	json.NewDecoder(loginW.Body).Decode(&loginResp)

	// Step 2: use the access token to refresh.
	refreshReq := httptest.NewRequest(http.MethodGet, "/user/get-auth/", nil)
	refreshReq.Header.Set("AccessToken", loginResp.AccessToken)
	refreshW := httptest.NewRecorder()
	RefreshAuth(refreshW, refreshReq)

	if refreshW.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d — body: %s", http.StatusOK, refreshW.Code, refreshW.Body.String())
	}

	var refreshResp map[string]string
	if err := json.NewDecoder(refreshW.Body).Decode(&refreshResp); err != nil {
		t.Fatalf("failed to decode refresh response: %v", err)
	}
	if refreshResp["new_access_token"] == "" {
		t.Error("expected non-empty new_access_token")
	}
}

// TestRefreshAuthNoSession verifies that refreshing without a prior login returns 401.
func TestRefreshAuthNoSession(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	// Create a valid token but without a corresponding session in the store.
	jti, _ := generateTokenID()
	token, _ := makeToken(tokenClaims{
		Subject:   "admin",
		IssuedAt:  0,
		ExpiresAt: 9999999999,
		TokenID:   jti,
		TokenType: "access",
	})

	// Ensure no session exists for this user.
	sessionStore.Lock()
	delete(sessionStore.data, "admin")
	sessionStore.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/user/get-auth/", nil)
	req.Header.Set("AccessToken", token)
	w := httptest.NewRecorder()

	RefreshAuth(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestGenerateCLITokenMethodNotAllowed verifies that non-POST requests are rejected.
func TestGenerateCLITokenMethodNotAllowed(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/user/generate-cli-token", nil)
	w := httptest.NewRecorder()

	GenerateCLIToken(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestGenerateCLITokenSuccess verifies that a POST request generates and returns a token.
func TestGenerateCLITokenSuccess(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/user/generate-cli-token", nil)
	w := httptest.NewRecorder()

	GenerateCLIToken(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d — body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp cliTokenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token in response")
	}
	if !resp.Exists {
		t.Error("expected exists=true in response")
	}
}

// TestGenerateCLITokenPersisted verifies that the token is written to disk.
func TestGenerateCLITokenPersisted(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/user/generate-cli-token", nil)
	w := httptest.NewRecorder()

	GenerateCLIToken(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, w.Code)
	}

	var resp cliTokenResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Verify the token was written to the expected path.
	path, err := cliTokenPath()
	if err != nil {
		t.Fatalf("cliTokenPath() error: %v", err)
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read stored CLI token: %v", err)
	}
	if strings.TrimSpace(string(stored)) != resp.Token {
		t.Errorf("stored token %q does not match response token %q", string(stored), resp.Token)
	}
}

// TestGenerateCLITokenRotation verifies that regenerating replaces the old token.
func TestGenerateCLITokenRotation(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	generate := func() string {
		req := httptest.NewRequest(http.MethodPost, "/user/generate-cli-token", nil)
		w := httptest.NewRecorder()
		GenerateCLIToken(w, req)
		var resp cliTokenResponse
		json.NewDecoder(w.Body).Decode(&resp)
		return resp.Token
	}

	first := generate()
	second := generate()

	if first == "" || second == "" {
		t.Fatal("one of the generated tokens was empty")
	}
	if first == second {
		t.Error("expected different tokens on regeneration")
	}
}

// TestGetCLITokenStatusMethodNotAllowed verifies that non-GET requests are rejected.
func TestGetCLITokenStatusMethodNotAllowed(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/user/get-cli-token", nil)
	w := httptest.NewRecorder()

	GetCLITokenStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestGetCLITokenStatusNotExists verifies that a missing token file returns exists=false.
func TestGetCLITokenStatusNotExists(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/user/get-cli-token", nil)
	w := httptest.NewRecorder()

	GetCLITokenStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d — body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp cliTokenResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Exists {
		t.Error("expected exists=false when no CLI token has been generated")
	}
}

// TestGetCLITokenStatusExists verifies that an existing token file returns exists=true.
func TestGetCLITokenStatusExists(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	// Generate a token first.
	genReq := httptest.NewRequest(http.MethodPost, "/user/generate-cli-token", nil)
	genW := httptest.NewRecorder()
	GenerateCLIToken(genW, genReq)
	if genW.Code != http.StatusOK {
		t.Fatalf("setup: GenerateCLIToken failed: %d", genW.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/user/get-cli-token", nil)
	w := httptest.NewRecorder()

	GetCLITokenStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, w.Code)
	}

	var resp cliTokenResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Exists {
		t.Error("expected exists=true after generating a CLI token")
	}
	if resp.Token != "" {
		t.Error("GetCLITokenStatus must not return the token value")
	}
}

// TestRevokeCLITokenMethodNotAllowed verifies that non-DELETE requests are rejected.
func TestRevokeCLITokenMethodNotAllowed(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/user/revoke-cli-token", nil)
	w := httptest.NewRecorder()

	RevokeCLIToken(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestRevokeCLITokenSuccess verifies that an existing token can be revoked.
func TestRevokeCLITokenSuccess(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	// Generate first.
	genReq := httptest.NewRequest(http.MethodPost, "/user/generate-cli-token", nil)
	genW := httptest.NewRecorder()
	GenerateCLIToken(genW, genReq)

	// Revoke.
	req := httptest.NewRequest(http.MethodDelete, "/user/revoke-cli-token", nil)
	w := httptest.NewRecorder()

	RevokeCLIToken(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d — body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp cliTokenResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Exists {
		t.Error("expected exists=false after revoking")
	}
}

// TestRevokeCLITokenNoToken verifies that revoking when no token exists still returns 200.
func TestRevokeCLITokenNoToken(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/user/revoke-cli-token", nil)
	w := httptest.NewRecorder()

	RevokeCLIToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}
}

// TestVerifyCLIToken verifies that VerifyCLIToken correctly matches stored tokens.
func TestVerifyCLIToken(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	// No token file yet.
	ok, err := VerifyCLIToken("sometoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false when no token file exists")
	}

	// Generate a token.
	genReq := httptest.NewRequest(http.MethodPost, "/user/generate-cli-token", nil)
	genW := httptest.NewRecorder()
	GenerateCLIToken(genW, genReq)
	var resp cliTokenResponse
	json.NewDecoder(genW.Body).Decode(&resp)

	// Correct token should match.
	ok, err = VerifyCLIToken(resp.Token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for the correct CLI token")
	}

	// Wrong token should not match.
	ok, err = VerifyCLIToken("wrongtoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for an incorrect CLI token")
	}
}

// --- WithCLITokenAuth middleware tests ---

// makeTestHandler returns a simple handler that records calls so we can verify
// the middleware forwarded the request.
func makeTestHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

// TestWithCLITokenAuthNoHeader verifies that requests without Basic Auth pass through.
func TestWithCLITokenAuthNoHeader(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	called := false
	handler := WithCLITokenAuth(makeTestHandler(&called))

	req := httptest.NewRequest(http.MethodGet, "/get-containers", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called when no Authorization header is present")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}
}

// TestWithCLITokenAuthValidToken verifies that a correct token passes through.
func TestWithCLITokenAuthValidToken(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	// Generate a CLI token.
	genReq := httptest.NewRequest(http.MethodPost, "/user/generate-cli-token", nil)
	genW := httptest.NewRecorder()
	GenerateCLIToken(genW, genReq)
	var resp cliTokenResponse
	json.NewDecoder(genW.Body).Decode(&resp)

	called := false
	handler := WithCLITokenAuth(makeTestHandler(&called))

	req := httptest.NewRequest(http.MethodGet, "/get-containers", nil)
	req.SetBasicAuth(resp.Token, "")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called for a valid CLI token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}
}

// TestWithCLITokenAuthInvalidToken verifies that a wrong token is rejected with 401.
func TestWithCLITokenAuthInvalidToken(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	// Generate a token so a token file exists, then try a different token.
	genReq := httptest.NewRequest(http.MethodPost, "/user/generate-cli-token", nil)
	genW := httptest.NewRecorder()
	GenerateCLIToken(genW, genReq)

	called := false
	handler := WithCLITokenAuth(makeTestHandler(&called))

	req := httptest.NewRequest(http.MethodGet, "/get-containers", nil)
	req.SetBasicAuth("wrongtoken", "")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("next handler must not be called for an invalid CLI token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestWithCLITokenAuthUserRouteBypass verifies that /user/* routes always pass through.
func TestWithCLITokenAuthUserRouteBypass(t *testing.T) {
	cleanup := setupCredentialsFile(t, "admin", "admin")
	defer cleanup()

	called := false
	handler := WithCLITokenAuth(makeTestHandler(&called))

	// Supply an invalid token — the /user/ bypass should ignore it.
	req := httptest.NewRequest(http.MethodPost, "/user/login", nil)
	req.SetBasicAuth("badtoken", "")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called for /user/ routes regardless of token validity")
	}
}
