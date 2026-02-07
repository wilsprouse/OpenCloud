/*
The Service Ledger
*/

package service_ledger

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
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

// PipelineEntry represents an individual pipeline's metadata in the ledger
type PipelineEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Code        string `json:"code"`
	Branch      string `json:"branch"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
}

// ServiceStatus represents the status of a single service
type ServiceStatus struct {
	Enabled   bool                      `json:"enabled"`
	Functions map[string]FunctionEntry  `json:"functions,omitempty"`
	Pipelines map[string]PipelineEntry  `json:"pipelines,omitempty"`
}

// ServiceLedger represents the complete service ledger
type ServiceLedger map[string]ServiceStatus

var ledgerMutex sync.Mutex

// serviceLedgerDir is the directory containing the service ledger files.
// It is initialized once during package initialization to avoid runtime.Caller issues.
var serviceLedgerDir string

func init() {
	// Initialize the service ledger directory path
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Critical: Failed to determine service ledger directory path using runtime.Caller(0). " +
			"This may occur in unusual contexts such as certain testing frameworks, stripped binaries, " +
			"or other non-standard execution environments.")
	}
	serviceLedgerDir = filepath.Dir(currentFile)
}

// getLedgerPath returns the absolute path to the serviceLedger.json file
func getLedgerPath() (string, error) {
	if serviceLedgerDir == "" {
		return "", fmt.Errorf("service ledger directory not initialized")
	}
	return filepath.Join(serviceLedgerDir, "serviceLedger.json"), nil
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

// InitializeServiceLedger ensures the service ledger file exists with default services
func InitializeServiceLedger() error {
	ledgerPath, err := getLedgerPath()
	if err != nil {
		return err
	}

	// Check if file already exists
	if _, err := os.Stat(ledgerPath); err == nil {
		// File exists, no need to initialize
		return nil
	}

	// Create initial ledger with default services
	initialLedger := ServiceLedger{
		"container_registry": ServiceStatus{
			Enabled: false,
		},
		"containers": ServiceStatus{
			Enabled: false,
		},
		"Functions": ServiceStatus{
			Enabled: false,
		},
		"blob_storage": ServiceStatus{
			Enabled: false,
		},
		"pipelines": ServiceStatus{
			Enabled: false,
		},
	}

	return WriteServiceLedger(initialLedger)
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

// executeServiceInstaller executes the installer script for a given service if it exists.
// The installer script is expected to be located at service_installers/{serviceName}.sh
// relative to the service_ledger directory.
//
// Parameters:
//   - serviceName: The name of the service to install
//
// Returns:
//   - error: Returns an error if the installer fails to execute. Returns nil if the
//            installer doesn't exist (as not all services require installers) or if
//            the installer executes successfully.
func executeServiceInstaller(serviceName string) error {
	// Use the package-level directory path initialized in init()
	// Note: This check is a defensive measure. In normal execution, serviceLedgerDir
	// is guaranteed to be initialized by init() (which calls log.Fatal if initialization
	// fails). However, this check provides a more graceful error if the function is
	// called in an unexpected context (e.g., during testing with reflection).
	if serviceLedgerDir == "" {
		return fmt.Errorf("service ledger directory not initialized")
	}

	// Construct the path to the installer script
	installerPath := filepath.Join(serviceLedgerDir, "service_installers", serviceName+".sh")

	// Check if the installer script exists
	if _, err := os.Stat(installerPath); os.IsNotExist(err) {
		// Installer doesn't exist, which is fine - not all services need installers
		log.Printf("No installer found for service '%s', skipping installation step", serviceName)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check installer script: %w", err)
	}

	// Make the script executable
	if err := os.Chmod(installerPath, 0755); err != nil {
		return fmt.Errorf("failed to make installer script executable: %w", err)
	}

	// Execute the installer script
	log.Printf("Executing installer script for service '%s'...", serviceName)
	cmd := exec.Command("/bin/bash", installerPath)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	
	// Log the output regardless of success or failure
	if len(output) > 0 {
		log.Printf("Installer output for '%s':\n%s", serviceName, string(output))
	}

	if err != nil {
		return fmt.Errorf("installer script failed for service '%s': %w", serviceName, err)
	}

	log.Printf("Successfully executed installer for service '%s'", serviceName)
	return nil
}

// EnableService enables a specific service in the ledger
func EnableService(serviceName string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	// Execute the service installer before enabling the service
	// If the installer fails, the service will not be enabled
	if err := executeServiceInstaller(serviceName); err != nil {
		return fmt.Errorf("failed to execute installer for service '%s': %w", serviceName, err)
	}

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

// UpdatePipelineEntry updates a specific pipeline entry in the pipelines service ledger
func UpdatePipelineEntry(pipelineID, name, description, code, branch, status, createdAt string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["pipelines"]
	if !exists {
		serviceStatus = ServiceStatus{Enabled: false, Pipelines: make(map[string]PipelineEntry)}
	} else if serviceStatus.Pipelines == nil {
		serviceStatus.Pipelines = make(map[string]PipelineEntry)
	}

	serviceStatus.Pipelines[pipelineID] = PipelineEntry{
		ID:          pipelineID,
		Name:        name,
		Description: description,
		Code:        code,
		Branch:      branch,
		Status:      status,
		CreatedAt:   createdAt,
	}

	ledger["pipelines"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// DeletePipelineEntry removes a pipeline entry from the pipelines service ledger
func DeletePipelineEntry(pipelineID string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["pipelines"]
	if !exists || serviceStatus.Pipelines == nil {
		return nil // Nothing to delete
	}

	delete(serviceStatus.Pipelines, pipelineID)
	ledger["pipelines"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// GetPipelineEntry retrieves a specific pipeline entry from the pipelines service ledger
func GetPipelineEntry(pipelineID string) (*PipelineEntry, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return nil, err
	}

	serviceStatus, exists := ledger["pipelines"]
	if !exists || serviceStatus.Pipelines == nil {
		return nil, nil
	}

	entry, exists := serviceStatus.Pipelines[pipelineID]
	if !exists {
		return nil, nil
	}

	return &entry, nil
}

// GetAllPipelineEntries retrieves all pipeline entries from the pipelines service ledger
func GetAllPipelineEntries() (map[string]PipelineEntry, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return nil, err
	}

	serviceStatus, exists := ledger["pipelines"]
	if !exists || serviceStatus.Pipelines == nil {
		return make(map[string]PipelineEntry), nil
	}

	return serviceStatus.Pipelines, nil
}

// SyncPipelines scans the ~/.opencloud/pipelines/ directory and updates the service ledger
// with any pipelines that exist on disk but are not yet tracked in the ledger
func SyncPipelines() error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	pipelineDir := filepath.Join(home, ".opencloud", "pipelines")

	// Check if directory exists
	if _, err := os.Stat(pipelineDir); os.IsNotExist(err) {
		// Directory doesn't exist, nothing to sync
		return nil
	}

	// Read all files in the pipelines directory
	entries, err := os.ReadDir(pipelineDir)
	if err != nil {
		return err
	}

	// Read current ledger
	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	// Get current pipeline entries
	serviceStatus, exists := ledger["pipelines"]
	if !exists {
		serviceStatus = ServiceStatus{Enabled: false, Pipelines: make(map[string]PipelineEntry)}
	} else if serviceStatus.Pipelines == nil {
		serviceStatus.Pipelines = make(map[string]PipelineEntry)
	}

	// Process each shell script file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .sh files
		if filepath.Ext(entry.Name()) != ".sh" {
			continue
		}

		// Read the file content
		scriptPath := filepath.Join(pipelineDir, entry.Name())
		scriptData, err := os.ReadFile(scriptPath)
		if err != nil {
			fmt.Printf("Warning: Failed to read pipeline file %s: %v\n", entry.Name(), err)
			continue // Skip files that can't be read
		}

		// Get file info for creation time
		fileInfo, err := entry.Info()
		if err != nil {
			fmt.Printf("Warning: Failed to get file info for %s: %v\n", entry.Name(), err)
			continue
		}

		// Extract pipeline name (remove .sh extension)
		pipelineName := entry.Name()[:len(entry.Name())-3]

		// Check if this pipeline already exists in the ledger
		// We check by comparing the code content to avoid duplicates
		found := false
		for _, existing := range serviceStatus.Pipelines {
			if existing.Code == string(scriptData) {
				found = true
				break
			}
		}

		// If not found, add it to the ledger
		if !found {
			// Generate a unique ID
			b := make([]byte, 8)
			if _, err := rand.Read(b); err != nil {
				fmt.Printf("Warning: Failed to generate pipeline ID for %s: %v\n", entry.Name(), err)
				continue
			}
			pipelineID := hex.EncodeToString(b)

			// Add the pipeline entry
			serviceStatus.Pipelines[pipelineID] = PipelineEntry{
				ID:          pipelineID,
				Name:        pipelineName,
				Description: "",
				Code:        string(scriptData),
				Branch:      "main",
				Status:      "idle",
				CreatedAt:   fileInfo.ModTime().Format("2006-01-02T15:04:05Z07:00"),
			}
		}
	}

	// Update the ledger
	ledger["pipelines"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// SyncPipelinesHandler is an HTTP handler that syncs pipelines from disk to the service ledger
func SyncPipelinesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := SyncPipelines(); err != nil {
		http.Error(w, "Failed to sync pipelines: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message": "Pipelines synced successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// detectRuntime determines the runtime based on the file extension
func detectRuntime(filename string) string {
	switch filepath.Ext(filename) {
	case ".py":
		return "python"
	case ".js":
		return "nodejs"
	case ".go":
		return "go"
	case ".rb":
		return "ruby"
	default:
		return "unknown"
	}
}

// SyncFunctions scans the ~/.opencloud/functions/ directory and updates the service ledger
// with any functions that exist on disk but are not yet tracked in the ledger
func SyncFunctions() error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	functionDir := filepath.Join(home, ".opencloud", "functions")

	// Check if directory exists
	if _, err := os.Stat(functionDir); os.IsNotExist(err) {
		// Directory doesn't exist, nothing to sync
		return nil
	}

	// Read all files in the functions directory
	entries, err := os.ReadDir(functionDir)
	if err != nil {
		return err
	}

	// Read current ledger
	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	// Get current function entries
	status, exists := ledger["Functions"]
	if !exists {
		status = ServiceStatus{Enabled: false, Functions: make(map[string]FunctionEntry)}
	} else if status.Functions == nil {
		status.Functions = make(map[string]FunctionEntry)
	}

	// Process each function file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		functionName := entry.Name()

		// Read the file content
		functionPath := filepath.Join(functionDir, functionName)
		functionData, err := os.ReadFile(functionPath)
		if err != nil {
			fmt.Printf("Warning: Failed to read function file %s: %v\n", functionName, err)
			continue // Skip files that can't be read
		}

		// Check if this function already exists in the ledger
		existingEntry, exists := status.Functions[functionName]

		// If it exists, preserve its existing metadata and only update content if changed
		if exists {
			// Only update if content has changed
			if existingEntry.Content != string(functionData) {
				existingEntry.Content = string(functionData)
				status.Functions[functionName] = existingEntry
			}
			// If content is the same, don't update anything to preserve logs and metadata
		} else {
			// New function - add it to the ledger
			status.Functions[functionName] = FunctionEntry{
				Runtime:  detectRuntime(functionName),
				Trigger:  "",
				Schedule: "",
				Content:  string(functionData),
				Logs:     []FunctionLog{},
			}
		}
	}

	// Update the ledger
	ledger["Functions"] = status

	return WriteServiceLedger(ledger)
}

// SyncFunctionsHandler is an HTTP handler that syncs functions from disk to the service ledger
func SyncFunctionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := SyncFunctions(); err != nil {
		http.Error(w, "Failed to sync functions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message": "Functions synced successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
