package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

// nginxConfigPath is the path to the nginx configuration file managed by OpenCloud.
// It can be overridden in tests.
var nginxConfigPath = "/etc/nginx/sites-available/opencloud"

// reloadNginx is the function used to reload nginx after configuration changes.
// It can be overridden in tests.
var reloadNginx = func() error {
	cmd := exec.Command("sudo", "nginx", "-s", "reload")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx reload failed: %w\noutput: %s", err, string(out))
	}
	return nil
}

// domainRegex validates standard (non-wildcard) nginx server_name values.
// Allowed characters are alphanumeric, hyphens, dots, and underscores.
// The value must start and end with an alphanumeric character or underscore.
var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9_]([a-zA-Z0-9\-_\.]*[a-zA-Z0-9_])?$`)

// wildcardDomainRegex validates wildcard nginx server_name values such as "*.example.com".
// The asterisk-dot prefix must be followed by a valid domain name.
var wildcardDomainRegex = regexp.MustCompile(`^\*\.[a-zA-Z0-9_]([a-zA-Z0-9\-_\.]*[a-zA-Z0-9_])?$`)

// isValidDomain returns true when domain is an acceptable nginx server_name value.
func isValidDomain(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	// "_" alone is the nginx catch-all directive – always valid.
	if domain == "_" {
		return true
	}
	// Wildcard subdomains must have the "*." prefix only at the very start.
	if strings.HasPrefix(domain, "*") {
		return wildcardDomainRegex.MatchString(domain)
	}
	return domainRegex.MatchString(domain)
}

// updateNginxServerName reads the nginx configuration file at nginxConfigPath, replaces
// the server_name directive, and writes the file back.
func updateNginxServerName(domain string) error {
	data, err := os.ReadFile(nginxConfigPath)
	if err != nil {
		return fmt.Errorf("reading nginx config: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	updated := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "server_name ") {
			// Preserve the original indentation.
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = fmt.Sprintf("%sserver_name %s;", indent, domain)
			updated = true
		}
	}

	if !updated {
		return fmt.Errorf("server_name directive not found in nginx config")
	}

	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(nginxConfigPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("writing nginx config: %w", err)
	}

	return nil
}

// GetInstanceDomainHandler handles GET /get-instance-domain.
// It returns the currently configured domain from the service ledger.
//
// Response: {"domain": "<value>"}
func GetInstanceDomainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	domain, err := service_ledger.GetInstanceDomain()
	if err != nil {
		http.Error(w, "Failed to read instance domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"domain": domain})
}

// SetInstanceDomainHandler handles POST /set-instance-domain.
// It validates the supplied domain, persists it in the service ledger, updates the nginx
// configuration, and reloads nginx.
//
// Request body: {"domain": "<value>"}
// Response:     {"domain": "<value>", "message": "Domain configured successfully"}
func SetInstanceDomainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Domain == "" {
		http.Error(w, "Missing required field: domain", http.StatusBadRequest)
		return
	}

	if !isValidDomain(req.Domain) {
		http.Error(w, "Invalid domain name", http.StatusBadRequest)
		return
	}

	// Persist the domain in the service ledger first.
	if err := service_ledger.SetInstanceDomain(req.Domain); err != nil {
		http.Error(w, "Failed to save domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update the nginx configuration file. If the config file is absent (e.g., development
	// environment without nginx), log a warning but still return success so the ledger value
	// is saved.
	if err := updateNginxServerName(req.Domain); err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "reading nginx config") {
			// nginx config not present – skip reload (development mode)
		} else {
			http.Error(w, "Failed to update nginx configuration: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Config updated successfully – reload nginx.
		if err := reloadNginx(); err != nil {
			// nginx reload failure is non-fatal: the domain is saved and the config file is
			// updated. The operator can reload nginx manually.
			http.Error(w, "Domain saved but nginx reload failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"domain":  req.Domain,
		"message": "Domain configured successfully",
	})
}
