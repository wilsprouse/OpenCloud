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
)

// FunctionLog represents a single function execution log
type FunctionLog struct {
	Timestamp string `json:"timestamp"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	Status    string `json:"status"` // "success" or "error"
}

// FunctionEntry represents an individual function's metadata in the ledger
type FunctionEntry struct {
	Runtime  string        `json:"runtime"`
	Trigger  string        `json:"trigger,omitempty"`
	Schedule string        `json:"schedule,omitempty"`
	Content  string        `json:"content"`
	Logs     []FunctionLog `json:"logs,omitempty"`
}

// ServiceStatus represents the status of a single service
type ServiceStatus struct {
	Enabled   bool                     `json:"enabled"`
	Functions map[string]FunctionEntry `json:"functions,omitempty"`
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

// UpdateFunctionEntry updates a specific function entry in the Functions service ledger
func UpdateFunctionEntry(functionName, runtime, trigger, schedule, content string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	status, exists := ledger["Functions"]
	if !exists {
		status = ServiceStatus{Enabled: false, Functions: make(map[string]FunctionEntry)}
	}

	if status.Functions == nil {
		status.Functions = make(map[string]FunctionEntry)
	}

	// Preserve existing logs when updating
	existingEntry, exists := status.Functions[functionName]
	var existingLogs []FunctionLog
	if exists {
		existingLogs = existingEntry.Logs
	}

	status.Functions[functionName] = FunctionEntry{
		Runtime:  runtime,
		Trigger:  trigger,
		Schedule: schedule,
		Content:  content,
		Logs:     existingLogs, // Preserve existing logs
	}

	ledger["Functions"] = status

	return WriteServiceLedger(ledger)
}

// DeleteFunctionEntry removes a function entry from the Functions service ledger
func DeleteFunctionEntry(functionName string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	status, exists := ledger["Functions"]
	if !exists || status.Functions == nil {
		return nil // Nothing to delete
	}

	delete(status.Functions, functionName)
	ledger["Functions"] = status

	return WriteServiceLedger(ledger)
}

// GetFunctionEntry retrieves a specific function entry from the Functions service ledger
func GetFunctionEntry(functionName string) (*FunctionEntry, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return nil, err
	}

	status, exists := ledger["Functions"]
	if !exists || status.Functions == nil {
		return nil, nil
	}

	entry, exists := status.Functions[functionName]
	if !exists {
		return nil, nil
	}

	return &entry, nil
}

// GetAllFunctionEntries retrieves all function entries from the Functions service ledger
func GetAllFunctionEntries() (map[string]FunctionEntry, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return nil, err
	}

	status, exists := ledger["Functions"]
	if !exists || status.Functions == nil {
		return make(map[string]FunctionEntry), nil
	}

	return status.Functions, nil
}
