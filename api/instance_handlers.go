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

// emailRegex and isValidEmail were removed; email is no longer collected by the
// SSL configuration flow — certbot prompts interactively during certificate issuance.

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

// buildCertbotInstructions returns a human-readable step-by-step guide for obtaining
// a Let's Encrypt SSL certificate using certbot. Certbot will interactively prompt
// the operator for an email address and Terms of Service agreement during issuance.
func buildCertbotInstructions(domain string) string {
	return fmt.Sprintf(
		"1. Install certbot and the nginx plugin (if not already installed):\n"+
			"   sudo apt-get install certbot python3-certbot-nginx -y\n\n"+
			"2. Obtain and install the SSL certificate for %s:\n"+
			"   sudo certbot --nginx -d %s\n\n"+
			"3. Verify that automatic certificate renewal is configured:\n"+
			"   sudo certbot renew --dry-run\n\n"+
			"4. Reload nginx to apply the changes (if not reloaded automatically):\n"+
			"   sudo systemctl reload nginx",
		domain, domain,
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

// ConfigureSSLRequest is the JSON body for POST /configure-ssl.
type ConfigureSSLRequest struct {
	// Domain is the domain name for which to obtain the SSL certificate.
	Domain string `json:"domain"`
}

// ConfigureSSLResponse is the JSON body returned by ConfigureSSLHandler.
type ConfigureSSLResponse struct {
	// Domain is the domain the certificate is being requested for.
	Domain string `json:"domain"`
	// CertbotInstallCmd is the command to install certbot if it is not already present.
	CertbotInstallCmd string `json:"certbotInstallCmd"`
	// CertbotCmd is the command to run certbot and obtain/install the certificate.
	// Certbot will interactively prompt for email and Terms of Service agreement.
	CertbotCmd string `json:"certbotCmd"`
	// AutoRenewCmd is the command to test the automatic renewal configuration.
	AutoRenewCmd string `json:"autoRenewCmd"`
	// Instructions contains the full step-by-step guide for certificate installation.
	Instructions string `json:"instructions"`
}

// GetSSLStatusHandler handles GET /get-ssl-status.
// It returns the currently configured Let's Encrypt email from the service ledger.
//
// Response: {"email": "<value>"}
func GetSSLStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	email, err := service_ledger.GetInstanceSSLEmail()
	if err != nil {
		http.Error(w, "Failed to read SSL status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"email": email})
}

// ConfigureSSLHandler handles POST /configure-ssl.
// It validates the domain and returns certbot commands for the operator to run
// to obtain a Let's Encrypt SSL certificate. Certbot will interactively prompt
// for an email address and Terms of Service agreement during certificate issuance.
// Because OpenCloud does not run with root permissions, it cannot invoke certbot directly.
//
// Request body: {"domain": "<value>"}
// Response:     ConfigureSSLResponse
func ConfigureSSLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConfigureSSLRequest
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

	certbotCmd := fmt.Sprintf("sudo certbot --nginx -d %s", req.Domain)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigureSSLResponse{
		Domain:            req.Domain,
		CertbotInstallCmd: "sudo apt-get install certbot python3-certbot-nginx -y",
		CertbotCmd:        certbotCmd,
		AutoRenewCmd:      "sudo certbot renew --dry-run",
		Instructions:      buildCertbotInstructions(req.Domain),
	})
}
