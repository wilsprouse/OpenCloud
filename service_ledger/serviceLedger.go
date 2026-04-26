/*
The Service Ledger
*/

package service_ledger

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	Runtime     string        `json:"runtime"`
	Trigger     string        `json:"trigger,omitempty"`
	Schedule    string        `json:"schedule,omitempty"`
	Content     string        `json:"content"`
	Logs        []FunctionLog `json:"logs,omitempty"`
	Invocations int           `json:"invocations"`
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

// ContainerImageEntry stores all information needed to rebuild a container image
type ContainerImageEntry struct {
	ImageName  string `json:"imageName"`
	Dockerfile string `json:"dockerfile"`
	Context    string `json:"context,omitempty"`
	Platform   string `json:"platform,omitempty"`
	NoCache    bool   `json:"nocache"`
	BuiltAt    string `json:"builtAt"`
	// PulledAt is set when the image was pulled from a remote registry rather than built locally.
	PulledAt string `json:"pulledAt,omitempty"`
	// Registry is the source registry used when pulling the image (e.g. "docker.io", "quay.io").
	Registry string `json:"registry,omitempty"`
	// Logs contains the build or pull log output captured during the image creation.
	Logs string `json:"logs,omitempty"`
}

// BucketEntry stores metadata for a blob storage bucket in the service ledger
type BucketEntry struct {
	Name           string `json:"name"`
	CreatedAt      string `json:"createdAt"`
	ContainerMount bool   `json:"containerMount"`
	// VolumeName is the Podman named volume created for this bucket when
	// ContainerMount is true (e.g. "opencloud-my-bucket").
	VolumeName string `json:"volumeName,omitempty"`
}

// GatewayRouteEntry stores a single routing rule for the Gateway service.
// When a request arrives at PathPrefix, it is proxied to TargetURL.
type GatewayRouteEntry struct {
	// ID is a unique identifier for the route (hex random string).
	ID string `json:"id"`
	// PathPrefix is the URL path prefix that triggers this route (e.g. "/app").
	PathPrefix string `json:"pathPrefix"`
	// TargetURL is the upstream address to proxy matching requests to (e.g. "http://localhost:8080").
	TargetURL string `json:"targetURL"`
	// Description is an optional human-readable note for the route.
	Description string `json:"description,omitempty"`
	// CreatedAt is the RFC3339 timestamp when the route was created.
	CreatedAt string `json:"createdAt"`
}

// ServiceStatus represents the status of a single service
type ServiceStatus struct {
	Enabled         bool                          `json:"enabled"`
	Functions       map[string]FunctionEntry      `json:"functions,omitempty"`
	Pipelines       map[string]PipelineEntry      `json:"pipelines,omitempty"`
	ContainerImages map[string]ContainerImageEntry `json:"containerImages,omitempty"`
	Buckets         map[string]BucketEntry        `json:"buckets,omitempty"`
	// GatewayRoutes stores the routing rules for the Gateway service.
	GatewayRoutes map[string]GatewayRouteEntry `json:"gatewayRoutes,omitempty"`
	// Domain stores the configured domain for the "instance" service ledger entry.
	Domain string `json:"domain,omitempty"`
	// SSLEmail stores the email address used for Let's Encrypt/certbot SSL configuration.
	SSLEmail string `json:"sslEmail,omitempty"`
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
		"blob_storage": ServiceStatus{
			Enabled: false,
		},
		"container_registry": ServiceStatus{
			Enabled: false,
		},
		"containers": ServiceStatus{
			Enabled: false,
		},
		"Functions": ServiceStatus{
			Enabled: false,
		},
		"gateway": ServiceStatus{
			Enabled: false,
		},
		"instance": ServiceStatus{
			Enabled: true,
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
	// Container Compute requires Container Registry. Auto-enable it first if needed.
	if serviceName == "containers" {
		registryEnabled, err := IsServiceEnabled("container_registry")
		if err != nil {
			return fmt.Errorf("failed to check container_registry status: %w", err)
		}
		if !registryEnabled {
			if err := EnableService("container_registry"); err != nil {
				return fmt.Errorf("failed to enable required container_registry service: %w", err)
			}
		}
	}

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

// enableServiceWithStream runs the service installer for the given service, sending each
// line of combined stdout/stderr output to the provided send callback as it is produced.
// Once the installer completes successfully the service is marked as enabled in the ledger.
// The mutex is NOT held during the installer execution so that streaming is not blocked.
func enableServiceWithStream(serviceName string, send func(string)) error {
	if serviceLedgerDir == "" {
		return fmt.Errorf("service ledger directory not initialized")
	}

	installerPath := filepath.Join(serviceLedgerDir, "service_installers", serviceName+".sh")
	if _, err := os.Stat(installerPath); os.IsNotExist(err) {
		send(fmt.Sprintf("[INFO] No installer found for service '%s', skipping installation step", serviceName))
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check installer script: %w", err)
	}

	if err := os.Chmod(installerPath, 0755); err != nil {
		return fmt.Errorf("failed to make installer script executable: %w", err)
	}

	send(fmt.Sprintf("[INFO] Executing installer for service '%s'...", serviceName))

	cmd := exec.Command("/bin/bash", installerPath)

	// Merge stdout and stderr into a single pipe for ordered, real-time output.
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
		return fmt.Errorf("installer script failed for service '%s': %w", serviceName, err)
	}

	return nil
}

// EnableServiceStreamHandler is an HTTP handler that enables a service and streams the
// installer output to the client using Server-Sent Events (SSE).
//
// Request: POST /enable-service-stream  body: {"service": "<name>"}
// Response: text/event-stream
//
//	data: <installer output line>
//	...
//	event: done
//	data: {"service":"<name>","enabled":true}
//
//	On error:
//	event: error
//	data: <error message>
func EnableServiceStreamHandler(w http.ResponseWriter, r *http.Request) {
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

	// Container Compute requires Container Registry. Auto-enable it with streaming output if needed.
	if body.Service == "containers" {
		registryEnabled, err := IsServiceEnabled("container_registry")
		if err != nil {
			errMsg := fmt.Sprintf("failed to check container_registry status: %s", err.Error())
			log.Printf("EnableServiceStreamHandler: %s", errMsg)
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", errMsg)
			flusher.Flush()
			return
		}
		if !registryEnabled {
			sendLine("[INFO] Container Registry is required by Containers. Enabling Container Registry first...")
			if err := enableServiceWithStream("container_registry", sendLine); err != nil {
				log.Printf("EnableServiceStreamHandler: installer error for 'container_registry': %v", err)
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
				flusher.Flush()
				return
			}
			// Mark container_registry as enabled in the ledger.
			ledgerMutex.Lock()
			regLedger, err := ReadServiceLedger()
			if err != nil {
				ledgerMutex.Unlock()
				errMsg := fmt.Sprintf("failed to read service ledger: %s", err.Error())
				log.Printf("EnableServiceStreamHandler: %s", errMsg)
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", errMsg)
				flusher.Flush()
				return
			}
			regLedger["container_registry"] = ServiceStatus{Enabled: true}
			if err := WriteServiceLedger(regLedger); err != nil {
				ledgerMutex.Unlock()
				errMsg := fmt.Sprintf("failed to write service ledger: %s", err.Error())
				log.Printf("EnableServiceStreamHandler: %s", errMsg)
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", errMsg)
				flusher.Flush()
				return
			}
			ledgerMutex.Unlock()
			sendLine("[SUCCESS] Container Registry service enabled successfully!")
		}
	}

	// Run the installer with real-time streaming output.
	if err := enableServiceWithStream(body.Service, sendLine); err != nil {
		log.Printf("EnableServiceStreamHandler: installer error for '%s': %v", body.Service, err)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	// Mark the service as enabled in the ledger.
	ledgerMutex.Lock()
	ledger, err := ReadServiceLedger()
	if err != nil {
		ledgerMutex.Unlock()
		errMsg := fmt.Sprintf("failed to read service ledger: %s", err.Error())
		log.Printf("EnableServiceStreamHandler: %s", errMsg)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", errMsg)
		flusher.Flush()
		return
	}
	ledger[body.Service] = ServiceStatus{Enabled: true}
	if err := WriteServiceLedger(ledger); err != nil {
		ledgerMutex.Unlock()
		errMsg := fmt.Sprintf("failed to write service ledger: %s", err.Error())
		log.Printf("EnableServiceStreamHandler: %s", errMsg)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", errMsg)
		flusher.Flush()
		return
	}
	ledgerMutex.Unlock()

	sendLine(fmt.Sprintf("[SUCCESS] %s service enabled successfully!", body.Service))
	fmt.Fprintf(w, "event: done\ndata: {\"service\":%q,\"enabled\":true}\n\n", body.Service)
	flusher.Flush()
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

	// Preserve existing logs and invocations when updating
	existingEntry, exists := status.Functions[functionName]
	var existingLogs []FunctionLog
	var existingInvocations int
	if exists {
		existingLogs = existingEntry.Logs
		existingInvocations = existingEntry.Invocations
	}

	status.Functions[functionName] = FunctionEntry{
		Runtime:     runtime,
		Trigger:     trigger,
		Schedule:    schedule,
		Content:     content,
		Logs:        existingLogs,        // Preserve existing logs
		Invocations: existingInvocations, // Preserve existing invocation count
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

// IncrementFunctionInvocations increments the invocation count for a function in the service ledger
func IncrementFunctionInvocations(functionName string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	status, exists := ledger["Functions"]
	if !exists || status.Functions == nil {
		return nil // Nothing to update
	}

	entry, exists := status.Functions[functionName]
	if !exists {
		return nil // Function not in ledger, skip
	}

	entry.Invocations++
	status.Functions[functionName] = entry
	ledger["Functions"] = status

	return WriteServiceLedger(ledger)
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

// UpdateContainerImageEntry stores or updates a container image entry in the container_registry service ledger.
// All fields needed to rebuild the image are persisted, including the captured build log output.
func UpdateContainerImageEntry(imageName, dockerfile, context, platform string, noCache bool, builtAt, logs string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["container_registry"]
	if !exists {
		serviceStatus = ServiceStatus{Enabled: false, ContainerImages: make(map[string]ContainerImageEntry)}
	} else if serviceStatus.ContainerImages == nil {
		serviceStatus.ContainerImages = make(map[string]ContainerImageEntry)
	}

	serviceStatus.ContainerImages[imageName] = ContainerImageEntry{
		ImageName:  imageName,
		Dockerfile: dockerfile,
		Context:    context,
		Platform:   platform,
		NoCache:    noCache,
		BuiltAt:    builtAt,
		Logs:       logs,
	}

	ledger["container_registry"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// RecordPulledImageEntry stores a pulled container image entry in the container_registry service ledger.
// Unlike UpdateContainerImageEntry, a pulled image has no Dockerfile — only the image reference,
// the registry it was fetched from, and the captured pull log output are recorded.
func RecordPulledImageEntry(imageName, registry, pulledAt, logs string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["container_registry"]
	if !exists {
		serviceStatus = ServiceStatus{Enabled: false, ContainerImages: make(map[string]ContainerImageEntry)}
	} else if serviceStatus.ContainerImages == nil {
		serviceStatus.ContainerImages = make(map[string]ContainerImageEntry)
	}

	serviceStatus.ContainerImages[imageName] = ContainerImageEntry{
		ImageName: imageName,
		Registry:  registry,
		PulledAt:  pulledAt,
		Logs:      logs,
	}

	ledger["container_registry"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// DeleteContainerImageEntry removes a container image entry from the container_registry service ledger
func DeleteContainerImageEntry(imageName string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["container_registry"]
	if !exists || serviceStatus.ContainerImages == nil {
		return nil // Nothing to delete
	}

	delete(serviceStatus.ContainerImages, imageName)
	ledger["container_registry"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// GetContainerImageEntry retrieves a specific container image entry from the container_registry service ledger
func GetContainerImageEntry(imageName string) (*ContainerImageEntry, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return nil, err
	}

	serviceStatus, exists := ledger["container_registry"]
	if !exists || serviceStatus.ContainerImages == nil {
		return nil, nil
	}

	entry, exists := serviceStatus.ContainerImages[imageName]
	if !exists {
		return nil, nil
	}

	return &entry, nil
}

// GetAllContainerImageEntries retrieves all container image entries from the container_registry service ledger
func GetAllContainerImageEntries() (map[string]ContainerImageEntry, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return nil, err
	}

	serviceStatus, exists := ledger["container_registry"]
	if !exists || serviceStatus.ContainerImages == nil {
		return make(map[string]ContainerImageEntry), nil
	}

	return serviceStatus.ContainerImages, nil
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

// UpdateBucketEntry stores or updates a blob storage bucket entry in the blob_storage service ledger.
func UpdateBucketEntry(bucketName, createdAt string, containerMount bool, volumeName string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["blob_storage"]
	if !exists {
		serviceStatus = ServiceStatus{Enabled: false, Buckets: make(map[string]BucketEntry)}
	} else if serviceStatus.Buckets == nil {
		serviceStatus.Buckets = make(map[string]BucketEntry)
	}

	serviceStatus.Buckets[bucketName] = BucketEntry{
		Name:           bucketName,
		CreatedAt:      createdAt,
		ContainerMount: containerMount,
		VolumeName:     volumeName,
	}

	ledger["blob_storage"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// DeleteBucketEntry removes a bucket entry from the blob_storage service ledger
func DeleteBucketEntry(bucketName string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["blob_storage"]
	if !exists || serviceStatus.Buckets == nil {
		return nil // Nothing to delete
	}

	delete(serviceStatus.Buckets, bucketName)
	ledger["blob_storage"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// GetBucketEntry retrieves a specific bucket entry from the blob_storage service ledger
func GetBucketEntry(bucketName string) (*BucketEntry, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return nil, err
	}

	serviceStatus, exists := ledger["blob_storage"]
	if !exists || serviceStatus.Buckets == nil {
		return nil, nil
	}

	entry, exists := serviceStatus.Buckets[bucketName]
	if !exists {
		return nil, nil
	}

	return &entry, nil
}

// GetAllBucketEntries retrieves all bucket entries from the blob_storage service ledger
func GetAllBucketEntries() (map[string]BucketEntry, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return nil, err
	}

	serviceStatus, exists := ledger["blob_storage"]
	if !exists || serviceStatus.Buckets == nil {
		return make(map[string]BucketEntry), nil
	}

	return serviceStatus.Buckets, nil
}

// RenameBucketEntry renames a bucket entry in the blob_storage service ledger,
// preserving the original CreatedAt timestamp.
func RenameBucketEntry(currentName, newName string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["blob_storage"]
	if !exists {
		serviceStatus = ServiceStatus{Enabled: false, Buckets: make(map[string]BucketEntry)}
	} else if serviceStatus.Buckets == nil {
		serviceStatus.Buckets = make(map[string]BucketEntry)
	}

	existing, exists := serviceStatus.Buckets[currentName]
	if !exists {
		// No entry to rename; nothing to do
		return nil
	}

	// Copy the entry under the new name and remove the old entry
	serviceStatus.Buckets[newName] = BucketEntry{
		Name:           newName,
		CreatedAt:      existing.CreatedAt,
		ContainerMount: existing.ContainerMount,
		VolumeName:     existing.VolumeName,
	}
	delete(serviceStatus.Buckets, currentName)

	ledger["blob_storage"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// GetInstanceDomain retrieves the configured domain from the "instance" service ledger entry.
// Returns an empty string if no domain has been configured yet.
func GetInstanceDomain() (string, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return "", err
	}

	status, exists := ledger["instance"]
	if !exists {
		return "", nil
	}

	return status.Domain, nil
}

// SetInstanceDomain stores the given domain in the "instance" service ledger entry.
func SetInstanceDomain(domain string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	status, exists := ledger["instance"]
	if !exists {
		status = ServiceStatus{Enabled: true}
	}

	status.Domain = domain
	ledger["instance"] = status

	return WriteServiceLedger(ledger)
}

// GetInstanceSSLEmail retrieves the Let's Encrypt email from the "instance" service ledger entry.
// Returns an empty string if no email has been configured yet.
func GetInstanceSSLEmail() (string, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return "", err
	}

	status, exists := ledger["instance"]
	if !exists {
		return "", nil
	}

	return status.SSLEmail, nil
}

// SetInstanceSSLEmail stores the given Let's Encrypt email in the "instance" service ledger entry.
func SetInstanceSSLEmail(email string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	status, exists := ledger["instance"]
	if !exists {
		status = ServiceStatus{Enabled: true}
	}

	status.SSLEmail = email
	ledger["instance"] = status

	return WriteServiceLedger(ledger)
}

// UpsertGatewayRouteEntry creates or updates a gateway route entry in the service ledger.
func UpsertGatewayRouteEntry(route GatewayRouteEntry) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["gateway"]
	if !exists {
		serviceStatus = ServiceStatus{Enabled: false, GatewayRoutes: make(map[string]GatewayRouteEntry)}
	} else if serviceStatus.GatewayRoutes == nil {
		serviceStatus.GatewayRoutes = make(map[string]GatewayRouteEntry)
	}

	serviceStatus.GatewayRoutes[route.ID] = route
	ledger["gateway"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// DeleteGatewayRouteEntry removes a gateway route entry from the service ledger.
func DeleteGatewayRouteEntry(routeID string) error {
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()

	ledger, err := ReadServiceLedger()
	if err != nil {
		return err
	}

	serviceStatus, exists := ledger["gateway"]
	if !exists || serviceStatus.GatewayRoutes == nil {
		return nil // Nothing to delete
	}

	delete(serviceStatus.GatewayRoutes, routeID)
	ledger["gateway"] = serviceStatus

	return WriteServiceLedger(ledger)
}

// GetAllGatewayRouteEntries retrieves all gateway route entries from the service ledger.
func GetAllGatewayRouteEntries() (map[string]GatewayRouteEntry, error) {
	ledger, err := ReadServiceLedger()
	if err != nil {
		return nil, err
	}

	serviceStatus, exists := ledger["gateway"]
	if !exists || serviceStatus.GatewayRoutes == nil {
		return make(map[string]GatewayRouteEntry), nil
	}

	return serviceStatus.GatewayRoutes, nil
}
