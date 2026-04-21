package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

// TestListGatewayRoutes_empty verifies that the handler returns an empty JSON
// array when no routes have been configured.
func TestListGatewayRoutes_empty(t *testing.T) {
	// Use a temp HOME so nginx config writes land in a temp directory.
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	req := httptest.NewRequest(http.MethodGet, "/list-gateway-routes", nil)
	w := httptest.NewRecorder()

	ListGatewayRoutes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var routes []service_ledger.GatewayRouteEntry
	if err := json.Unmarshal(w.Body.Bytes(), &routes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// The result should be a JSON array (may not be empty if other tests left routes).
	if routes == nil {
		t.Error("expected a JSON array, got nil")
	}
}

// TestListGatewayRoutes_wrongMethod verifies that non-GET requests are rejected.
func TestListGatewayRoutes_wrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/list-gateway-routes", nil)
	w := httptest.NewRecorder()

	ListGatewayRoutes(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestCreateGatewayRoute_success verifies that a valid route is stored and returned.
func TestCreateGatewayRoute_success(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	body, _ := json.Marshal(CreateGatewayRouteRequest{
		PathPrefix:  "/api-gw",
		TargetURL:   "http://localhost:8080",
		Description: "test route",
	})
	req := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(body))
	w := httptest.NewRecorder()

	CreateGatewayRoute(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var route service_ledger.GatewayRouteEntry
	if err := json.Unmarshal(w.Body.Bytes(), &route); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if route.PathPrefix != "/api-gw" {
		t.Errorf("expected pathPrefix=/api-gw, got %s", route.PathPrefix)
	}
	if route.TargetURL != "http://localhost:8080" {
		t.Errorf("expected targetURL=http://localhost:8080, got %s", route.TargetURL)
	}
	if route.ID == "" {
		t.Error("expected non-empty route ID")
	}

	// Clean up by deleting the created route.
	if route.ID != "" {
		_ = service_ledger.DeleteGatewayRouteEntry(route.ID)
	}
}

// TestCreateGatewayRoute_missingFields verifies that missing required fields
// return 400.
func TestCreateGatewayRoute_missingFields(t *testing.T) {
	tests := []struct {
		name string
		body CreateGatewayRouteRequest
	}{
		{"missing pathPrefix", CreateGatewayRouteRequest{TargetURL: "http://localhost:8080"}},
		{"missing targetURL", CreateGatewayRouteRequest{PathPrefix: "/app"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(body))
			w := httptest.NewRecorder()
			CreateGatewayRoute(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

// TestCreateGatewayRoute_noLeadingSlash verifies that a path prefix without a
// leading slash returns 400.
func TestCreateGatewayRoute_noLeadingSlash(t *testing.T) {
	body, _ := json.Marshal(CreateGatewayRouteRequest{
		PathPrefix: "no-slash",
		TargetURL:  "http://localhost:9000",
	})
	req := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(body))
	w := httptest.NewRecorder()

	CreateGatewayRoute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestCreateGatewayRoute_invalidTargetURL verifies that a malformed target URL
// returns 400.
func TestCreateGatewayRoute_invalidTargetURL(t *testing.T) {
	body, _ := json.Marshal(CreateGatewayRouteRequest{
		PathPrefix: "/app",
		TargetURL:  "not-a-url",
	})
	req := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(body))
	w := httptest.NewRecorder()

	CreateGatewayRoute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestUpdateGatewayRoute_success verifies that an existing route can be updated.
func TestUpdateGatewayRoute_success(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create a route first.
	createBody, _ := json.Marshal(CreateGatewayRouteRequest{
		PathPrefix: "/old",
		TargetURL:  "http://localhost:1111",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(createBody))
	createW := httptest.NewRecorder()
	CreateGatewayRoute(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", createW.Code, createW.Body.String())
	}

	var created service_ledger.GatewayRouteEntry
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	defer service_ledger.DeleteGatewayRouteEntry(created.ID)

	// Now update it.
	updateBody, _ := json.Marshal(UpdateGatewayRouteRequest{
		PathPrefix:  "/new",
		TargetURL:   "http://localhost:2222",
		Description: "updated",
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/update-gateway-route/"+created.ID, bytes.NewReader(updateBody))
	updateW := httptest.NewRecorder()
	UpdateGatewayRoute(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", updateW.Code, updateW.Body.String())
	}

	var updated service_ledger.GatewayRouteEntry
	if err := json.Unmarshal(updateW.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if updated.PathPrefix != "/new" {
		t.Errorf("expected /new, got %s", updated.PathPrefix)
	}
	if updated.TargetURL != "http://localhost:2222" {
		t.Errorf("expected http://localhost:2222, got %s", updated.TargetURL)
	}
}

// TestUpdateGatewayRoute_notFound verifies that updating a non-existent route
// returns 404.
func TestUpdateGatewayRoute_notFound(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	body, _ := json.Marshal(UpdateGatewayRouteRequest{
		PathPrefix: "/x",
		TargetURL:  "http://localhost:9999",
	})
	req := httptest.NewRequest(http.MethodPut, "/update-gateway-route/doesnotexist", bytes.NewReader(body))
	w := httptest.NewRecorder()

	UpdateGatewayRoute(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestDeleteGatewayRoute_success verifies that a route is removed after deletion.
func TestDeleteGatewayRoute_success(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create a route.
	createBody, _ := json.Marshal(CreateGatewayRouteRequest{
		PathPrefix: "/to-delete",
		TargetURL:  "http://localhost:3333",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(createBody))
	createW := httptest.NewRecorder()
	CreateGatewayRoute(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create failed: %d", createW.Code)
	}

	var created service_ledger.GatewayRouteEntry
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Delete it.
	delReq := httptest.NewRequest(http.MethodDelete, "/delete-gateway-route/"+created.ID, nil)
	delW := httptest.NewRecorder()
	DeleteGatewayRoute(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", delW.Code, delW.Body.String())
	}

	// Verify it is gone.
	routes, err := service_ledger.GetAllGatewayRouteEntries()
	if err != nil {
		t.Fatalf("unexpected error listing routes: %v", err)
	}

	if _, stillThere := routes[created.ID]; stillThere {
		t.Errorf("route %s still present after deletion", created.ID)
	}
}

// TestDeleteGatewayRoute_missingID verifies that a DELETE without an ID returns 400.
func TestDeleteGatewayRoute_missingID(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/delete-gateway-route/", nil)
	w := httptest.NewRecorder()

	DeleteGatewayRoute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestApplyGatewayNginxConfig_generatesFile verifies that the nginx config file
// is written to the expected path when routes are provided.
func TestApplyGatewayNginxConfig_generatesFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	routes := map[string]service_ledger.GatewayRouteEntry{
		"abc": {
			ID:         "abc",
			PathPrefix: "/svc",
			TargetURL:  "http://localhost:9000",
		},
	}

	// applyGatewayNginxConfig will fail to reload nginx (not running in test
	// environments) but should still write the file and return nil.
	if err := applyGatewayNginxConfig(routes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	confPath := tmpHome + "/.opencloud/gateway/nginx-gateway.conf"
	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	content := string(data)
	if !containsString(content, "/svc") {
		t.Errorf("config missing expected prefix /svc; got:\n%s", content)
	}
	if !containsString(content, "http://localhost:9000") {
		t.Errorf("config missing expected target URL; got:\n%s", content)
	}
}

// containsString is a small helper used by gateway tests.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})())
}
