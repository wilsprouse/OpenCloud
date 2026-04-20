package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

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

// buildNginxInstructions returns a human-readable string describing how to update
// the nginx configuration to use the given domain, since OpenCloud does not have
// root permissions to modify nginx directly.
func buildNginxInstructions(domain string) string {
	return fmt.Sprintf(
		"1. Edit the nginx configuration file:\n"+
			"   sudo vim /etc/nginx/sites-available/opencloud\n\n"+
			"2. Find the 'server_name' line and replace it with:\n"+
			"   server_name %s;\n\n"+
			"3. Test the configuration:\n"+
			"   sudo nginx -t\n\n"+
			"4. Reload nginx to apply the change:\n"+
			"   sudo systemctl reload nginx",
		domain,
	)
}

// SetInstanceDomainResponse is the JSON body returned by SetInstanceDomainHandler.
type SetInstanceDomainResponse struct {
	// Domain is the value that was saved.
	Domain string `json:"domain"`
	// NginxEditCmd is the command to open the nginx config for editing (requires sudo).
	NginxEditCmd string `json:"nginxEditCmd"`
	// NginxConfigLine is the exact server_name directive to place in the nginx config.
	NginxConfigLine string `json:"nginxConfigLine"`
	// NginxReloadCmd is the command the operator should run to apply and reload nginx.
	NginxReloadCmd string `json:"nginxReloadCmd"`
	// Instructions contains a full step-by-step guide for applying the nginx change.
	Instructions string `json:"instructions"`
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
// It validates the supplied domain and persists it in the service ledger.
// Because OpenCloud does not run with root permissions, it cannot modify nginx
// directly. Instead, the response includes the exact nginx configuration line
// and step-by-step commands the operator should run to apply the change.
//
// Request body: {"domain": "<value>"}
// Response:     SetInstanceDomainResponse
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

	// Persist the domain in the service ledger.
	if err := service_ledger.SetInstanceDomain(req.Domain); err != nil {
		http.Error(w, "Failed to save domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SetInstanceDomainResponse{
		Domain:          req.Domain,
		NginxEditCmd:    "sudo vim /etc/nginx/sites-available/opencloud",
		NginxConfigLine: fmt.Sprintf("server_name %s;", req.Domain),
		NginxReloadCmd:  "sudo nginx -t && sudo systemctl reload nginx",
		Instructions:    buildNginxInstructions(req.Domain),
	})
}

