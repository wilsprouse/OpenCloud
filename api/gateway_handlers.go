package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

// gatewayRoutePathRegex is intentionally left as a simple URL-path validator.
// We use url.Parse instead of a regex so that any valid URL path is accepted.

// GatewayRoute is the API representation of a gateway routing rule.
type GatewayRoute struct {
	ID          string `json:"id"`
	PathPrefix  string `json:"pathPrefix"`
	TargetURL   string `json:"targetURL"`
	Description string `json:"description,omitempty"`
	ServiceType string `json:"serviceType,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

// CreateGatewayRouteRequest holds the fields accepted when creating a new route.
type CreateGatewayRouteRequest struct {
	PathPrefix  string `json:"pathPrefix"`
	TargetURL   string `json:"targetURL"`
	Description string `json:"description,omitempty"`
	ServiceType string `json:"serviceType,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
}

// UpdateGatewayRouteRequest holds the fields that can be updated on an existing route.
type UpdateGatewayRouteRequest struct {
	PathPrefix  string `json:"pathPrefix"`
	TargetURL   string `json:"targetURL"`
	Description string `json:"description,omitempty"`
	ServiceType string `json:"serviceType,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
}

// validateGatewayRoute checks that the path prefix and target URL supplied by
// the caller are well-formed.
func validateGatewayRoute(pathPrefix, targetURL string) error {
	if pathPrefix == "" {
		return fmt.Errorf("pathPrefix is required")
	}
	if !strings.HasPrefix(pathPrefix, "/") {
		return fmt.Errorf("pathPrefix must start with /")
	}
	if targetURL == "" {
		return fmt.Errorf("targetURL is required")
	}
	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("targetURL must be an absolute URL (e.g. http://localhost:8080)")
	}
	return nil
}

// generateRouteID returns a random 8-byte hex string suitable for use as a
// unique gateway route identifier.
func generateRouteID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ListGatewayRoutes handles GET /list-gateway-routes.
// It returns all configured gateway routing rules as a JSON array.
func ListGatewayRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries, err := service_ledger.GetAllGatewayRoutes()
	if err != nil {
		http.Error(w, "Failed to read gateway routes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	routes := make([]GatewayRoute, 0, len(entries))
	for _, e := range entries {
		routes = append(routes, GatewayRoute{
			ID:          e.ID,
			PathPrefix:  e.PathPrefix,
			TargetURL:   e.TargetURL,
			Description: e.Description,
			ServiceType: e.ServiceType,
			ServiceName: e.ServiceName,
			CreatedAt:   e.CreatedAt,
		})
	}

	// Return a stable, creation-time-sorted list so the UI is deterministic.
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].CreatedAt < routes[j].CreatedAt
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(routes)
}

// CreateGatewayRoute handles POST /create-gateway-route.
// It creates a new routing rule and (re)generates the nginx gateway config.
func CreateGatewayRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateGatewayRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := validateGatewayRoute(req.PathPrefix, req.TargetURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := generateRouteID()
	if err != nil {
		http.Error(w, "Failed to generate route ID", http.StatusInternalServerError)
		return
	}

	entry := service_ledger.GatewayRouteEntry{
		ID:          id,
		PathPrefix:  req.PathPrefix,
		TargetURL:   req.TargetURL,
		Description: req.Description,
		ServiceType: req.ServiceType,
		ServiceName: req.ServiceName,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	if err := service_ledger.AddGatewayRoute(entry); err != nil {
		http.Error(w, "Failed to save gateway route: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := applyGatewayNginxConfig(); err != nil {
		// Log but do not fail — the route is saved; nginx reload is best-effort.
		fmt.Printf("Warning: failed to apply gateway nginx config: %v\n", err)
	}

	route := GatewayRoute{
		ID:          entry.ID,
		PathPrefix:  entry.PathPrefix,
		TargetURL:   entry.TargetURL,
		Description: entry.Description,
		ServiceType: entry.ServiceType,
		ServiceName: entry.ServiceName,
		CreatedAt:   entry.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(route)
}

// UpdateGatewayRoute handles PUT /update-gateway-route/{id}.
// It replaces the mutable fields of an existing route and regenerates the nginx config.
func UpdateGatewayRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract route ID from the URL path: /update-gateway-route/{id}
	id := strings.TrimPrefix(r.URL.Path, "/update-gateway-route/")
	if id == "" {
		http.Error(w, "Missing route ID", http.StatusBadRequest)
		return
	}

	existing, err := service_ledger.GetGatewayRoute(id)
	if err != nil {
		http.Error(w, "Failed to read gateway route: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "Gateway route not found", http.StatusNotFound)
		return
	}

	var req UpdateGatewayRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := validateGatewayRoute(req.PathPrefix, req.TargetURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated := service_ledger.GatewayRouteEntry{
		ID:          existing.ID,
		PathPrefix:  req.PathPrefix,
		TargetURL:   req.TargetURL,
		Description: req.Description,
		ServiceType: req.ServiceType,
		ServiceName: req.ServiceName,
		CreatedAt:   existing.CreatedAt,
	}

	if err := service_ledger.AddGatewayRoute(updated); err != nil {
		http.Error(w, "Failed to update gateway route: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := applyGatewayNginxConfig(); err != nil {
		fmt.Printf("Warning: failed to apply gateway nginx config: %v\n", err)
	}

	route := GatewayRoute{
		ID:          updated.ID,
		PathPrefix:  updated.PathPrefix,
		TargetURL:   updated.TargetURL,
		Description: updated.Description,
		ServiceType: updated.ServiceType,
		ServiceName: updated.ServiceName,
		CreatedAt:   updated.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(route)
}

// DeleteGatewayRoute handles DELETE /delete-gateway-route/{id}.
// It removes the route from the service ledger and regenerates the nginx config.
func DeleteGatewayRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/delete-gateway-route/")
	if id == "" {
		http.Error(w, "Missing route ID", http.StatusBadRequest)
		return
	}

	existing, err := service_ledger.GetGatewayRoute(id)
	if err != nil {
		http.Error(w, "Failed to read gateway route: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "Gateway route not found", http.StatusNotFound)
		return
	}

	if err := service_ledger.DeleteGatewayRoute(id); err != nil {
		http.Error(w, "Failed to delete gateway route: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := applyGatewayNginxConfig(); err != nil {
		fmt.Printf("Warning: failed to apply gateway nginx config: %v\n", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Gateway route deleted successfully"})
}

// reloadNginx attempts to reload the nginx process using several common
// methods.  It tries each command in order and returns nil as soon as one
// succeeds.  If no method works it returns an error so the caller can log a
// helpful manual-intervention message without aborting the request.
func reloadNginx() error {
	type attempt struct {
		name string
		args []string
	}
	for _, a := range []attempt{
		// Prefer the fastest method that doesn't require sudo.
		{"nginx", []string{"-s", "reload"}},
		// Try sudo variants for setups where the app user is not root.
		{"sudo", []string{"nginx", "-s", "reload"}},
		{"sudo", []string{"systemctl", "reload", "nginx"}},
		{"systemctl", []string{"reload", "nginx"}},
	} {
		if err := exec.Command(a.name, a.args...).Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf(
		"nginx reload: all methods failed (tried: nginx -s reload, " +
			"sudo nginx -s reload, sudo systemctl reload nginx). " +
			"Run one of those commands manually to apply the new routes.",
	)
}

// applyGatewayNginxConfig reads all current gateway routes from the service
// ledger and writes a nginx include file to
// ~/.opencloud/gateway/nginx-gateway.conf.
//
// The generated file contains one `location` block per route (plus a
// companion `location =` block that redirects the bare path without a
// trailing slash so that e.g. /app redirects to /app/).
//
// Include this file inside your nginx server block BEFORE the catch-all
// location:
//
//	# Replace /home/ubuntu with your actual $HOME
//	include /home/ubuntu/.opencloud/gateway/nginx-gateway.conf;
//	location / { proxy_pass http://localhost:3000; }
//
// After including the file, nginx must be reloaded once so it picks up the
// directive.  Subsequent route changes are applied automatically because this
// function calls `nginx -s reload` (or the first sudo variant that succeeds)
// after every write.
func applyGatewayNginxConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	gatewayDir := filepath.Join(home, ".opencloud", "gateway")
	if err := os.MkdirAll(gatewayDir, 0755); err != nil {
		return fmt.Errorf("failed to create gateway directory: %w", err)
	}

	entries, err := service_ledger.GetAllGatewayRoutes()
	if err != nil {
		return fmt.Errorf("failed to read gateway routes: %w", err)
	}

	// Build a stable, sorted list of routes.
	routes := make([]service_ledger.GatewayRouteEntry, 0, len(entries))
	for _, e := range entries {
		routes = append(routes, e)
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].CreatedAt < routes[j].CreatedAt
	})

	confPath := filepath.Join(gatewayDir, "nginx-gateway.conf")

	var sb strings.Builder
	sb.WriteString("# OpenCloud Gateway - auto-generated nginx location blocks\n")
	sb.WriteString("# Do not edit manually; changes will be overwritten by OpenCloud.\n")
	sb.WriteString("# Include this file inside your nginx server {} block BEFORE location /:\n")
	sb.WriteString("#   include " + confPath + ";\n\n")

	for _, route := range routes {
		// Ensure the path prefix ends with / for consistent nginx prefix matching.
		prefix := route.PathPrefix
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}

		// Ensure the target URL ends with / so nginx strips the prefix correctly
		// (e.g. location /app/ { proxy_pass http://localhost:8080/; } causes
		// nginx to strip /app/ from the request path before forwarding).
		target := route.TargetURL
		if !strings.HasSuffix(target, "/") {
			target += "/"
		}

		// Emit a redirect from the bare path (without trailing slash) to the
		// trailing-slash version.  This ensures that a browser visiting /rabby
		// gets redirected to /rabby/ and subsequently matched by the location
		// block below, instead of falling through to the catch-all location /.
		bare := strings.TrimSuffix(prefix, "/")
		if bare != "" && bare != "/" {
			if route.Description != "" {
				sb.WriteString(fmt.Sprintf("# %s\n", route.Description))
			}
			sb.WriteString(fmt.Sprintf("location = %s {\n", bare))
			sb.WriteString(fmt.Sprintf("    return 301 $scheme://$host%s;\n", prefix))
			sb.WriteString("}\n")
		}

		if route.Description != "" {
			sb.WriteString(fmt.Sprintf("# %s\n", route.Description))
		}
		sb.WriteString(fmt.Sprintf("location %s {\n", prefix))
		sb.WriteString(fmt.Sprintf("    proxy_pass %s;\n", target))
		sb.WriteString("    proxy_http_version 1.1;\n")
		sb.WriteString("    proxy_set_header Host $host;\n")
		sb.WriteString("    proxy_set_header X-Real-IP $remote_addr;\n")
		sb.WriteString("    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		sb.WriteString("    proxy_set_header X-Forwarded-Proto $scheme;\n")
		sb.WriteString("    proxy_set_header Upgrade $http_upgrade;\n")
		sb.WriteString("    proxy_set_header Connection 'upgrade';\n")
		sb.WriteString("}\n\n")
	}

	if err := os.WriteFile(confPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write nginx gateway config: %w", err)
	}

	// Best-effort nginx reload so the new location blocks take effect
	// immediately.  If the reload fails (e.g. the app doesn't have sudo
	// privileges), a warning is printed and the caller should run the
	// reload command manually.
	if err := reloadNginx(); err != nil {
		fmt.Printf("Warning: gateway config written to %s but nginx reload failed: %v\n", confPath, err)
	}

	return nil
}
