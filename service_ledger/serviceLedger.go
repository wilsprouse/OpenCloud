/*
The Service Ledger
*/

package service_ledger

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// ServiceStatus represents the status of a single service
type ServiceStatus struct {
	Enabled     bool   `json:"enabled"`
	LastUpdated string `json:"lastUpdated,omitempty"`
}

// ServiceLedger represents the complete service ledger
type ServiceLedger map[string]ServiceStatus

var ledgerMutex sync.Mutex

// getLedgerPath returns the absolute path to the serviceLedger.json file
func getLedgerPath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	dir := filepath.Dir(currentFile)
	return filepath.Join(dir, "serviceLedger.json"), nil
}

// ReadServiceLedger reads and parses the service ledger JSON file
func ReadServiceLedger() (ServiceLedger, error) {
	ledgerPath, err := getLedgerPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty ledger if file doesn't exist
			return make(ServiceLedger), nil
		}
		return nil, err
	}

	var ledger ServiceLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return nil, err
	}

	return ledger, nil
}

// WriteServiceLedger writes the service ledger to the JSON file
func WriteServiceLedger(ledger ServiceLedger) error {
	ledgerPath, err := getLedgerPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(ledger, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(ledgerPath, data, 0600)
}

// IsServiceEnabled checks if a specific service is enabled
func IsServiceEnabled(serviceName string) (bool, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return false, err
	}

	status, exists := ledger[serviceName]
	if !exists {
		return false, nil
	}

	return status.Enabled, nil
}

// EnableService enables a specific service in the ledger
func EnableService(serviceName string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	ledger[serviceName] = ServiceStatus{Enabled: true}

	return WriteServiceLedger(ledger)
}

// GetServiceStatusHandler is an HTTP handler that returns the status of a service
func GetServiceStatusHandler(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "Missing service parameter", http.StatusBadRequest)
		return
	}

	enabled, err := IsServiceEnabled(serviceName)
	if err != nil {
		http.Error(w, "Failed to read service ledger: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"service": serviceName,
		"enabled": enabled,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// EnableServiceHandler is an HTTP handler that enables a service
func EnableServiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Service string `json:"service"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Service == "" {
		http.Error(w, "Missing service field", http.StatusBadRequest)
		return
	}

	if err := EnableService(body.Service); err != nil {
		http.Error(w, "Failed to enable service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"service": body.Service,
		"enabled": true,
		"message": "Service enabled successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateServiceActivity updates the lastUpdated timestamp for a service in the ledger
func UpdateServiceActivity(serviceName string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	status, exists := ledger[serviceName]
	if !exists {
		status = ServiceStatus{Enabled: false}
	}

	status.LastUpdated = time.Now().Format(time.RFC3339)
	ledger[serviceName] = status

	return WriteServiceLedger(ledger)
}
