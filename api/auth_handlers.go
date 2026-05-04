package api

import (
	"bufio"
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

// cliTokensPath returns the path to the JSON file that stores all active CLI
// token hashes.
func cliTokensPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".opencloud", "user", "cli_tokens.json"), nil
}

// cliTokenEntry represents one stored CLI token (without the plaintext).
type cliTokenEntry struct {
	// ID is the unique identifier for this token (hex-encoded random bytes).
	ID string `json:"id"`
	// Hash is the bcrypt hash of the plaintext token.
	Hash string `json:"hash"`
	// CreatedAt is the RFC3339-formatted creation timestamp.
	CreatedAt string `json:"createdAt"`
}

// readCLITokens reads all stored CLI token entries from disk.
// Returns an empty slice when no tokens have been stored yet.
func readCLITokens() ([]cliTokenEntry, error) {
	path, err := cliTokensPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []cliTokenEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cli tokens: cannot read file: %w", err)
	}

	var entries []cliTokenEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("cli tokens: cannot parse file: %w", err)
	}
	return entries, nil
}

// writeCLITokens persists the given CLI token entries to disk.
func writeCLITokens(entries []cliTokenEntry) error {
	path, err := cliTokensPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("cli tokens: cannot create directory: %w", err)
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("cli tokens: cannot encode entries: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cli tokens: cannot write file: %w", err)
	}
	return nil
}

// StoreCLITokenHash bcrypt-hashes the given plaintext CLI token, appends a new
// entry to the token list, and persists it to disk.  The generated token ID and
// creation timestamp are returned.
func StoreCLITokenHash(token string) (id string, createdAt string, err error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("cli token: hash failed: %w", err)
	}

	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", fmt.Errorf("cli token: cannot generate id: %w", err)
	}
	id = hex.EncodeToString(idBytes)
	createdAt = time.Now().UTC().Format(time.RFC3339)

	entries, err := readCLITokens()
	if err != nil {
		return "", "", err
	}

	entries = append(entries, cliTokenEntry{
		ID:        id,
		Hash:      string(hash),
		CreatedAt: createdAt,
	})

	if err := writeCLITokens(entries); err != nil {
		return "", "", err
	}

	return id, createdAt, nil
}

// VerifyCLIToken checks whether the provided plaintext token matches any of the
// stored bcrypt hashes.  It returns (false, nil) when no tokens exist.
func VerifyCLIToken(token string) (bool, error) {
	entries, err := readCLITokens()
	if err != nil {
		return false, err
	}

	for _, e := range entries {
		err := bcrypt.CompareHashAndPassword([]byte(e.Hash), []byte(token))
		if err == nil {
			return true, nil
		}
		if err != bcrypt.ErrMismatchedHashAndPassword {
			// Unexpected error (e.g. malformed hash); skip this entry.
			continue
		}
	}
	return false, nil
}

// ListCLITokenMeta returns metadata for all stored CLI tokens (IDs and creation
// timestamps) without exposing any hash material.
func ListCLITokenMeta() ([]cliTokenMeta, error) {
	entries, err := readCLITokens()
	if err != nil {
		return nil, err
	}

	meta := make([]cliTokenMeta, len(entries))
	for i, e := range entries {
		meta[i] = cliTokenMeta{ID: e.ID, CreatedAt: e.CreatedAt}
	}
	return meta, nil
}

// cliTokenMeta is a safe, hash-free view of a cliTokenEntry for API responses.
type cliTokenMeta struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
}

// RevokeCLIToken removes the token with the given ID from persistent storage.
// Returns (false, nil) when no token with that ID exists.
func RevokeCLIToken(id string) (bool, error) {
	entries, err := readCLITokens()
	if err != nil {
		return false, err
	}

	filtered := make([]cliTokenEntry, 0, len(entries))
	found := false
	for _, e := range entries {
		if e.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, e)
	}

	if !found {
		return false, nil
	}

	if err := writeCLITokens(filtered); err != nil {
		return false, err
	}
	return true, nil
}

// WithCLITokenAuth is HTTP middleware that authenticates requests using HTTP
// Basic Auth where the CLI token is supplied as the username (password is
// ignored).  This supports the usage pattern:
//
//	curl -u "<token>:" http://localhost:3000/api/<path>
//
// Behavior:
//   - Request has no Authorization header → passed through unchanged.
//   - Request has a valid Basic-auth CLI token → passed through.
//   - Request has an Authorization header with an invalid/unknown token → 401.
func WithCLITokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// No auth header — let the request pass through as before.
			next.ServeHTTP(w, r)
			return
		}

		const prefix = "Basic "
		if !strings.HasPrefix(authHeader, prefix) {
			// Non-Basic auth schemes are not supported for CLI tokens.
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, prefix))
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Basic auth format: "username:password".  For CLI token auth the
		// token is the username; the password field is intentionally empty.
		parts := strings.SplitN(string(decoded), ":", 2)
		token := parts[0]

		ok, err := VerifyCLIToken(token)
		if err != nil || !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

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
