package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

// gatewayLedgerPath returns the absolute path to serviceLedger.json, mirroring
// the logic in the service_ledger package so that tests can check for the
// file's existence without importing the private helper.
func gatewayLedgerPath(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// gateway_handlers_test.go lives in api/; serviceLedger.json lives in
	// service_ledger/ which is a sibling directory.
	return filepath.Join(filepath.Dir(currentFile), "..", "service_ledger", "serviceLedger.json")
}

// setupGatewayTest saves the current service ledger (or notes that it doesn't
// exist yet), clears all gateway routes so that each test starts from a
// pristine state, and registers a t.Cleanup that restores the original state.
// It also redirects HOME to a temp dir so that nginx config files don't leak
// into the developer's home directory during tests.
func setupGatewayTest(t *testing.T) {
	t.Helper()

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)

	ledgerPath := gatewayLedgerPath(t)

	// Check whether the ledger file exists before the test runs.
	origData, readErr := os.ReadFile(ledgerPath)
	ledgerExistedBefore := readErr == nil

	// Parse the snapshot so we can clear gateway routes for this test.
	var snapshot service_ledger.ServiceLedger
	if ledgerExistedBefore {
		if jsonErr := json.Unmarshal(origData, &snapshot); jsonErr != nil {
			t.Logf("setupGatewayTest: could not parse ledger snapshot: %v", jsonErr)
			snapshot = nil
		}
	}

	// Write a clean version of the ledger that preserves all services but
	// removes any existing gateway routes.  This ensures the test starts
	// from a deterministic state even if a previous run left stale data.
	if snapshot != nil {
		clean := make(service_ledger.ServiceLedger)
		for k, v := range snapshot {
			if k == "gateway" {
				clean[k] = service_ledger.ServiceStatus{Enabled: v.Enabled}
			} else {
				clean[k] = v
			}
		}
		if writeErr := service_ledger.WriteServiceLedger(clean); writeErr != nil {
			t.Logf("setupGatewayTest: failed to clear gateway routes: %v", writeErr)
		}
	}

	t.Cleanup(func() {
		os.Setenv("HOME", origHome)
		if ledgerExistedBefore {
			// Restore the exact bytes that were there before.
			_ = os.WriteFile(ledgerPath, origData, 0600)
		} else {
			// The file did not exist before; remove it to restore that state.
			_ = os.Remove(ledgerPath)
		}
	})
}

// TestListGatewayRoutes_empty confirms that an empty list is returned when no
// routes have been created yet.
func TestListGatewayRoutes_empty(t *testing.T) {
	setupGatewayTest(t)

	req := httptest.NewRequest(http.MethodGet, "/list-gateway-routes", nil)
	w := httptest.NewRecorder()
	ListGatewayRoutes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var routes []GatewayRoute
	if err := json.Unmarshal(w.Body.Bytes(), &routes); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(routes))
	}
}

// TestCreateGatewayRoute_valid verifies that a well-formed route is persisted
// and returned by the create handler.
func TestCreateGatewayRoute_valid(t *testing.T) {
	setupGatewayTest(t)

	body, _ := json.Marshal(CreateGatewayRouteRequest{
		PathPrefix:  "/my-app/",
		TargetURL:   "http://localhost:8080",
		Description: "My test app",
	})

	req := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	CreateGatewayRoute(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var route GatewayRoute
	if err := json.Unmarshal(w.Body.Bytes(), &route); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if route.ID == "" {
		t.Error("expected a non-empty route ID")
	}
	if route.PathPrefix != "/my-app/" {
		t.Errorf("expected pathPrefix /my-app/, got %s", route.PathPrefix)
	}
	if route.TargetURL != "http://localhost:8080" {
		t.Errorf("expected targetURL http://localhost:8080, got %s", route.TargetURL)
	}
	if route.Description != "My test app" {
		t.Errorf("expected description 'My test app', got %s", route.Description)
	}

	// Verify the route was also persisted to the service ledger.
	entry, err := service_ledger.GetGatewayRoute(route.ID)
	if err != nil {
		t.Fatalf("failed to read gateway route from ledger: %v", err)
	}
	if entry == nil {
		t.Fatal("route not found in ledger after creation")
	}
}

// TestCreateGatewayRoute_missingFields checks that missing required fields are
// rejected with 400.
func TestCreateGatewayRoute_missingFields(t *testing.T) {
	setupGatewayTest(t)

	cases := []struct {
		name string
		body CreateGatewayRouteRequest
	}{
		{"missing pathPrefix", CreateGatewayRouteRequest{TargetURL: "http://localhost:8080"}},
		{"missing targetURL", CreateGatewayRouteRequest{PathPrefix: "/app/"}},
		{"bad pathPrefix (no leading /)", CreateGatewayRouteRequest{PathPrefix: "app/", TargetURL: "http://localhost:8080"}},
		{"bad targetURL (relative)", CreateGatewayRouteRequest{PathPrefix: "/app/", TargetURL: "/not-absolute"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(b))
			w := httptest.NewRecorder()
			CreateGatewayRoute(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

// TestUpdateGatewayRoute verifies that an existing route can be updated.
func TestUpdateGatewayRoute(t *testing.T) {
	setupGatewayTest(t)

	// Create a route first.
	createBody, _ := json.Marshal(CreateGatewayRouteRequest{
		PathPrefix: "/old/",
		TargetURL:  "http://localhost:9000",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	CreateGatewayRoute(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("setup: expected 201, got %d", createW.Code)
	}

	var created GatewayRoute
	json.Unmarshal(createW.Body.Bytes(), &created)

	// Now update it.
	updateBody, _ := json.Marshal(UpdateGatewayRouteRequest{
		PathPrefix:  "/new/",
		TargetURL:   "http://localhost:7000",
		Description: "Updated route",
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/update-gateway-route/"+created.ID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	UpdateGatewayRoute(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", updateW.Code, updateW.Body.String())
	}

	var updated GatewayRoute
	json.Unmarshal(updateW.Body.Bytes(), &updated)

	if updated.PathPrefix != "/new/" {
		t.Errorf("expected pathPrefix /new/, got %s", updated.PathPrefix)
	}
	if updated.TargetURL != "http://localhost:7000" {
		t.Errorf("expected targetURL http://localhost:7000, got %s", updated.TargetURL)
	}
	if updated.Description != "Updated route" {
		t.Errorf("expected description 'Updated route', got %s", updated.Description)
	}
	// CreatedAt must be preserved.
	if updated.CreatedAt != created.CreatedAt {
		t.Errorf("createdAt changed: was %s, now %s", created.CreatedAt, updated.CreatedAt)
	}
}

// TestUpdateGatewayRoute_notFound checks that updating a non-existent route
// returns 404.
func TestUpdateGatewayRoute_notFound(t *testing.T) {
	setupGatewayTest(t)

	body, _ := json.Marshal(UpdateGatewayRouteRequest{
		PathPrefix: "/x/",
		TargetURL:  "http://localhost:1234",
	})
	req := httptest.NewRequest(http.MethodPut, "/update-gateway-route/doesnotexist", bytes.NewReader(body))
	w := httptest.NewRecorder()
	UpdateGatewayRoute(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestDeleteGatewayRoute verifies that a route can be deleted.
func TestDeleteGatewayRoute(t *testing.T) {
	setupGatewayTest(t)

	// Create a route to delete.
	createBody, _ := json.Marshal(CreateGatewayRouteRequest{
		PathPrefix: "/to-delete/",
		TargetURL:  "http://localhost:5555",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	CreateGatewayRoute(createW, createReq)

	var created GatewayRoute
	json.Unmarshal(createW.Body.Bytes(), &created)

	// Delete it.
	delReq := httptest.NewRequest(http.MethodDelete, "/delete-gateway-route/"+created.ID, nil)
	delW := httptest.NewRecorder()
	DeleteGatewayRoute(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", delW.Code, delW.Body.String())
	}

	// Confirm it is gone from the ledger.
	entry, err := service_ledger.GetGatewayRoute(created.ID)
	if err != nil {
		t.Fatalf("error reading ledger: %v", err)
	}
	if entry != nil {
		t.Error("route still present in ledger after deletion")
	}
}

// TestDeleteGatewayRoute_notFound checks that deleting a non-existent route
// returns 404.
func TestDeleteGatewayRoute_notFound(t *testing.T) {
	setupGatewayTest(t)

	req := httptest.NewRequest(http.MethodDelete, "/delete-gateway-route/ghost", nil)
	w := httptest.NewRecorder()
	DeleteGatewayRoute(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestListGatewayRoutes_returnsSaved verifies that routes created via
// CreateGatewayRoute appear in the list returned by ListGatewayRoutes.
func TestListGatewayRoutes_returnsSaved(t *testing.T) {
	setupGatewayTest(t)

	paths := []string{"/alpha/", "/beta/", "/gamma/"}
	for _, p := range paths {
		b, _ := json.Marshal(CreateGatewayRouteRequest{PathPrefix: p, TargetURL: "http://localhost:8000"})
		req := httptest.NewRequest(http.MethodPost, "/create-gateway-route", bytes.NewReader(b))
		w := httptest.NewRecorder()
		CreateGatewayRoute(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: expected 201, got %d", p, w.Code)
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/list-gateway-routes", nil)
	listW := httptest.NewRecorder()
	ListGatewayRoutes(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", listW.Code)
	}

	var routes []GatewayRoute
	json.Unmarshal(listW.Body.Bytes(), &routes)

	if len(routes) != len(paths) {
		t.Errorf("expected %d routes, got %d", len(paths), len(routes))
	}
}
