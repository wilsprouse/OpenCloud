package api

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// tokenClaims holds the data encoded inside an auth token.
type tokenClaims struct {
	Subject   string `json:"sub"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	TokenID   string `json:"jti"`
	TokenType string `json:"type"` // "access" or "refresh"
}

// userSession tracks the refresh token issued to a user.
type userSession struct {
	refreshToken  string
	refreshExpiry time.Time
}

// sessionStore holds active user sessions keyed by username.
var sessionStore = struct {
	sync.RWMutex
	data map[string]*userSession
}{data: make(map[string]*userSession)}

// tokenSecret is the HMAC signing secret loaded once at startup.
var (
	tokenSecret     []byte
	tokenSecretOnce sync.Once
)

// loadTokenSecret reads (or creates) the HMAC secret stored at
// ~/.opencloud/user/secret.  The function panics only on unrecoverable
// errors because a missing secret would silently break all auth.
func loadTokenSecret() {
	tokenSecretOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(fmt.Sprintf("auth: cannot determine home directory: %v", err))
		}
		secretPath := filepath.Join(home, ".opencloud", "user", "secret")

		data, err := os.ReadFile(secretPath)
		if err == nil && len(data) >= 64 {
			tokenSecret = data
			return
		}

		// Generate a new 64-byte random secret.
		secret := make([]byte, 64)
		if _, err := rand.Read(secret); err != nil {
			panic(fmt.Sprintf("auth: cannot generate token secret: %v", err))
		}
		encoded := make([]byte, hex.EncodedLen(len(secret)))
		hex.Encode(encoded, secret)

		if err := os.WriteFile(secretPath, encoded, 0600); err != nil {
			panic(fmt.Sprintf("auth: cannot write token secret: %v", err))
		}
		tokenSecret = encoded
	})
}

// makeToken creates a signed token containing the given claims.
// Format: base64url(json(claims)) + "." + hex(HMAC-SHA256(payload, secret))
func makeToken(claims tokenClaims) (string, error) {
	loadTokenSecret()

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)

	mac := hmac.New(sha256.New, tokenSecret)
	mac.Write([]byte(encodedPayload))
	sig := hex.EncodeToString(mac.Sum(nil))

	return encodedPayload + "." + sig, nil
}

// parseToken decodes and verifies a token, returning its claims.
// When allowExpired is true, the expiry check is skipped (used during token
// refresh to identify the caller even when the access token has expired).
func parseToken(token string, allowExpired bool) (*tokenClaims, error) {
	loadTokenSecret()

	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed token")
	}

	// Verify HMAC signature.
	mac := hmac.New(sha256.New, tokenSecret)
	mac.Write([]byte(parts[0]))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedSig), []byte(parts[1])) {
		return nil, fmt.Errorf("invalid token signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("malformed token payload")
	}

	var claims tokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("malformed token claims")
	}

	if !allowExpired && time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

// generateTokenID returns a random hex string suitable for use as a token ID.
func generateTokenID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// issueTokenPair creates a new access/refresh token pair for the given user.
func issueTokenPair(username string) (accessToken, refreshToken string, err error) {
	now := time.Now()

	accessJTI, err := generateTokenID()
	if err != nil {
		return "", "", err
	}
	accessToken, err = makeToken(tokenClaims{
		Subject:   username,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(1 * time.Hour).Unix(),
		TokenID:   accessJTI,
		TokenType: "access",
	})
	if err != nil {
		return "", "", err
	}

	refreshJTI, err := generateTokenID()
	if err != nil {
		return "", "", err
	}
	refreshExpiry := now.Add(7 * 24 * time.Hour)
	refreshToken, err = makeToken(tokenClaims{
		Subject:   username,
		IssuedAt:  now.Unix(),
		ExpiresAt: refreshExpiry.Unix(),
		TokenID:   refreshJTI,
		TokenType: "refresh",
	})
	if err != nil {
		return "", "", err
	}

	// Store the refresh token so RefreshAuth can validate it.
	sessionStore.Lock()
	sessionStore.data[username] = &userSession{
		refreshToken:  refreshToken,
		refreshExpiry: refreshExpiry,
	}
	sessionStore.Unlock()

	return accessToken, refreshToken, nil
}

// credentialsPath returns the path to the credentials file.
func credentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".opencloud", "user", "credentials"), nil
}

// verifyCredentials checks username/password against the credentials file.
//
// File format (~/.opencloud/user/credentials):
//
//	# Lines starting with '#' are comments and are ignored.
//	# Blank lines are also ignored.
//	# Each non-blank, non-comment line must have the form:
//	#   username:bcrypt_hash
//	admin:$2a$10$...
func verifyCredentials(username, password string) (bool, error) {
	path, err := credentialsPath()
	if err != nil {
		return false, err
	}

	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("cannot open credentials file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		storedUser := parts[0]
		storedHash := parts[1]

		if storedUser == username {
			err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
			return err == nil, nil
		}
	}
	return false, scanner.Err()
}

// loginRequest is the JSON body accepted by the Login handler.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse is the JSON body returned by the Login handler on success.
type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// errorResponse is a generic JSON error body.
type errorResponse struct {
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Login handles POST /user/login.
// It validates the provided credentials and, on success, returns a new
// access/refresh token pair.
func Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Message: "method not allowed"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: "invalid request body"})
		return
	}

	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: "username and password are required"})
		return
	}

	ok, err := verifyCredentials(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "authentication service unavailable"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "invalid username or password"})
		return
	}

	accessToken, refreshToken, err := issueTokenPair(req.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to issue tokens"})
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// cliTokenPath returns the path to the CLI token file.
func cliTokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".opencloud", "user", "cli_token"), nil
}

// generateCLITokenValue creates a cryptographically random hex token string.
func generateCLITokenValue() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// cliTokenResponse is returned by the generate and get CLI token handlers.
type cliTokenResponse struct {
	Token   string `json:"token,omitempty"`
	Exists  bool   `json:"exists"`
	Message string `json:"message,omitempty"`
}

// GenerateCLIToken handles POST /user/generate-cli-token.
// It generates a new random CLI token, persists it to ~/.opencloud/user/cli_token,
// and returns the token value to the caller. Any previously issued CLI token is
// replaced. The token is returned only on generation — use this response to copy
// the token before navigating away.
func GenerateCLIToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Message: "method not allowed"})
		return
	}

	token, err := generateCLITokenValue()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to generate CLI token"})
		return
	}

	path, err := cliTokenPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to resolve token path"})
		return
	}

	// Ensure the parent directory exists (created at startup, but guard anyway).
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to create token directory"})
		return
	}

	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to save CLI token"})
		return
	}

	writeJSON(w, http.StatusOK, cliTokenResponse{Token: token, Exists: true})
}

// GetCLITokenStatus handles GET /user/get-cli-token.
// It reports whether a CLI token has been generated, but does NOT return the
// token value (to avoid exposing it over the wire after initial generation).
func GetCLITokenStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Message: "method not allowed"})
		return
	}

	path, err := cliTokenPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to resolve token path"})
		return
	}

	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		writeJSON(w, http.StatusOK, cliTokenResponse{Exists: false})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to check CLI token"})
		return
	}

	writeJSON(w, http.StatusOK, cliTokenResponse{Exists: true})
}

// RevokeCLIToken handles DELETE /user/revoke-cli-token.
// It removes the stored CLI token file, immediately invalidating the token.
func RevokeCLIToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Message: "method not allowed"})
		return
	}

	path, err := cliTokenPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to resolve token path"})
		return
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		writeJSON(w, http.StatusOK, cliTokenResponse{Exists: false, Message: "no CLI token to revoke"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to revoke CLI token"})
		return
	}

	writeJSON(w, http.StatusOK, cliTokenResponse{Exists: false, Message: "CLI token revoked"})
}

// cliAuthKey is an unexported context key used to mark requests that have
// been authenticated via CLI token.
type cliAuthKey struct{}

// WithCLITokenAuth is an HTTP middleware that supports CLI token authentication
// via HTTP Basic Auth.  When a caller supplies a valid CLI token as the username
// (e.g. curl -u "<token>:"), the request is passed to the next handler.
//
//   - Requests with NO Authorization header are forwarded unchanged, preserving
//     compatibility with the browser UI which uses the AccessToken header.
//   - Requests with a Basic Auth header are validated against the stored CLI
//     token.  An invalid token results in a 401 Unauthorized response.
//   - Routes under /user/ are always forwarded unchanged (they manage their own
//     authentication — login, refresh, and token management endpoints).
func WithCLITokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pass user-management endpoints through without CLI token checks.
		if strings.HasPrefix(r.URL.Path, "/user/") {
			next.ServeHTTP(w, r)
			return
		}

		username, _, hasBasicAuth := r.BasicAuth()
		if !hasBasicAuth {
			// No Basic Auth header — let the request through unchanged.
			next.ServeHTTP(w, r)
			return
		}

		// A Basic Auth header was provided: validate the token.
		valid, err := VerifyCLIToken(username)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "authentication service unavailable"})
			return
		}
		if !valid {
			w.Header().Set("WWW-Authenticate", `Basic realm="OpenCloud CLI"`)
			writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "invalid CLI token"})
			return
		}

		// Valid CLI token — mark context and forward the request.
		ctx := context.WithValue(r.Context(), cliAuthKey{}, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// VerifyCLIToken checks whether the provided raw token matches the stored CLI token.
// Returns true if the token matches, false otherwise.
func VerifyCLIToken(token string) (bool, error) {
	path, err := cliTokenPath()
	if err != nil {
		return false, err
	}

	stored, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(string(stored)) == strings.TrimSpace(token), nil
}

// RefreshAuth handles GET /user/get-auth/.
// It reads the current (possibly expired) access token from the AccessToken
// header, verifies the HMAC signature to identify the user, checks that a
// valid refresh session exists, and returns a fresh access token.
func RefreshAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Message: "method not allowed"})
		return
	}

	rawToken := r.Header.Get("AccessToken")
	if rawToken == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "missing access token"})
		return
	}

	// Parse the token while allowing it to be expired so we can still
	// identify the caller.
	claims, err := parseToken(rawToken, true)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "invalid access token"})
		return
	}

	// Look up the stored refresh session for this user and validate expiry
	// while still holding the read lock to avoid a TOCTOU race.
	sessionStore.RLock()
	session, ok := sessionStore.data[claims.Subject]
	expired := ok && time.Now().After(session.refreshExpiry)
	sessionStore.RUnlock()

	if !ok || expired {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "refresh session expired, please log in again"})
		return
	}

	// Issue a new access token (refresh token is kept the same).
	jti, err := generateTokenID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to generate token"})
		return
	}
	now := time.Now()
	newAccessToken, err := makeToken(tokenClaims{
		Subject:   claims.Subject,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(1 * time.Hour).Unix(),
		TokenID:   jti,
		TokenType: "access",
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "failed to issue token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"new_access_token": newAccessToken,
	})
}
