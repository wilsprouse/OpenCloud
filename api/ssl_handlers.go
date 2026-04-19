package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// sslHandlerDir holds the directory containing this source file, resolved once
// at package init time so that the SSL installer script path is always correct
// regardless of the working directory at runtime.
var sslHandlerDir string

func init() {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("ssl_handlers: failed to determine source directory via runtime.Caller")
	}
	sslHandlerDir = filepath.Dir(currentFile)
}

// SSLStatus represents the current SSL/TLS configuration status reported to the
// frontend.
type SSLStatus struct {
	// Configured is true when at least one Let's Encrypt certificate directory
	// exists under /etc/letsencrypt/live/.
	Configured bool `json:"configured"`
	// Domains lists every domain for which a certificate has been issued.
	Domains []string `json:"domains,omitempty"`
}

// letsEncryptLiveDir is the directory where certbot stores issued certificates.
// It is a variable (not a constant) so tests can override it.
var letsEncryptLiveDir = "/etc/letsencrypt/live"

// GetSSLStatus handles GET /ssl-status.
//
// It inspects the Let's Encrypt certificate directory to determine whether SSL
// has been configured and which domains are covered.
//
// Response (200 OK):
//
//	{"configured": true,  "domains": ["example.com"]}
//	{"configured": false}
func GetSSLStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	domains := []string{}
	entries, err := os.ReadDir(letsEncryptLiveDir)
	if err == nil {
		for _, entry := range entries {
			// certbot creates one sub-directory per domain; the README file is
			// not a certificate directory and is intentionally skipped.
			if entry.IsDir() && !strings.EqualFold(entry.Name(), "README") {
				domains = append(domains, entry.Name())
			}
		}
	}

	status := SSLStatus{
		Configured: len(domains) > 0,
		Domains:    domains,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// sslInstallerPath returns the absolute path to the ssl.sh installer script.
// It is a variable so tests can substitute a different path.
var sslInstallerPath = func() string {
	// The script lives in service_ledger/service_installers/ssl.sh relative to
	// the repository root, which is one level above this file's package.
	repoRoot := filepath.Dir(sslHandlerDir)
	return filepath.Join(repoRoot, "service_ledger", "service_installers", "ssl.sh")
}

// configureSSLCmd is the function that builds the *exec.Cmd used by
// ConfigureSSLStream.  It is a variable so tests can substitute a lightweight
// fake command without running certbot.
var configureSSLCmd = func(domain, email string) *exec.Cmd {
	scriptPath := sslInstallerPath()
	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Env = append(os.Environ(),
		"OPENCLOUD_SSL_DOMAIN="+domain,
		"OPENCLOUD_SSL_EMAIL="+email,
	)
	return cmd
}

// ConfigureSSLStream handles POST /configure-ssl-stream.
//
// It accepts a JSON body with domain, email, and agree_tos fields, then runs
// the SSL installer script and streams every output line back to the client
// using Server-Sent Events (SSE).
//
// Request body:
//
//	{"domain":"example.com","email":"admin@example.com","agree_tos":true}
//
// SSE response:
//
//	data: <installer output line>
//	...
//	event: done
//	data: {"domain":"example.com","configured":true}
//
//	On error:
//	event: error
//	data: <error message>
func ConfigureSSLStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domain   string `json:"domain"`
		Email    string `json:"email"`
		AgreeTOS bool   `json:"agree_tos"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Domain = strings.TrimSpace(req.Domain)
	req.Email = strings.TrimSpace(req.Email)

	if req.Domain == "" {
		http.Error(w, "Missing domain", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		http.Error(w, "Missing email", http.StatusBadRequest)
		return
	}
	if !req.AgreeTOS {
		http.Error(w, "You must agree to the Let's Encrypt Terms of Service", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sendLine := func(line string) {
		fmt.Fprintf(w, "data: %s\n\n", line)
		flusher.Flush()
	}

	sendLine(fmt.Sprintf("[INFO] Configuring SSL for domain: %s", req.Domain))

	if err := runSSLInstaller(req.Domain, req.Email, sendLine); err != nil {
		log.Printf("ConfigureSSLStream: error for domain %q: %v", req.Domain, err)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "event: done\ndata: {\"domain\":%q,\"configured\":true}\n\n", req.Domain)
	flusher.Flush()
}

// runSSLInstaller executes the ssl.sh installer script, forwarding each output
// line to the provided send callback as it is produced.
func runSSLInstaller(domain, email string, send func(string)) error {
	cmd := configureSSLCmd(domain, email)

	// Merge stdout and stderr into a single pipe so output arrives in order.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	errCh := make(chan error, 1)
	go func() {
		runErr := cmd.Run()
		pw.Close()
		errCh <- runErr
	}()

	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		send(scanner.Text())
	}

	if err := <-errCh; err != nil {
		return fmt.Errorf("SSL installer failed: %w", err)
	}
	return nil
}
