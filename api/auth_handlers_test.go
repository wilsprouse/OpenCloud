package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
