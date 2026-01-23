package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

var pipelineNameRegex = regexp.MustCompile(`[^a-zA-Z0-9\-_.]`)

// pipelineProcesses keeps track of running pipeline processes
var (
	pipelineProcesses = make(map[string]*exec.Cmd)
	pipelineMutex     sync.Mutex
)

type Pipeline struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Code        string    `json:"code"`
	Branch      string    `json:"branch"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	LastRun     *time.Time `json:"lastRun,omitempty"`
	Duration    string    `json:"duration,omitempty"`
}

type CreatePipelineRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Code        string `json:"code"`
	Branch      string `json:"branch"`
}

func CreatePipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req CreatePipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Code == "" {
		http.Error(w, "Missing required fields: name and code", http.StatusBadRequest)
		return
	}

	// Sanitize pipeline name to prevent directory traversal and invalid filenames
	sanitizedName := sanitizePipelineName(req.Name)
	if sanitizedName == "" {
		http.Error(w, "Invalid pipeline name", http.StatusBadRequest)
		return
	}

	// Set default branch if not provided
	if req.Branch == "" {
		req.Branch = "main"
	}

	// Get home directory and create pipelines directory
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	pipelineDir := filepath.Join(home, ".opencloud", "pipelines")
	if err := os.MkdirAll(pipelineDir, 0755); err != nil {
		http.Error(w, "Failed to create pipelines directory", http.StatusInternalServerError)
		return
	}

	// Generate unique ID for the pipeline
	pipelineID, err := generatePipelineID()
	if err != nil {
		http.Error(w, "Failed to generate pipeline ID", http.StatusInternalServerError)
		return
	}

	// Create shell script filename from sanitized name
	pipelineFileName := sanitizedName + ".sh"
	pipelinePath := filepath.Join(pipelineDir, pipelineFileName)

	// Check if pipeline already exists
	if _, err := os.Stat(pipelinePath); err == nil {
		http.Error(w, "Pipeline already exists", http.StatusConflict)
		return
	}

	// Write pipeline code to shell script file
	if err := os.WriteFile(pipelinePath, []byte(req.Code), 0755); err != nil {
		http.Error(w, "Failed to create pipeline file", http.StatusInternalServerError)
		return
	}

	// Create response with pipeline details
	pipeline := Pipeline{
		ID:          pipelineID,
		Name:        req.Name,
		Description: req.Description,
		Code:        req.Code,
		Branch:      req.Branch,
		Status:      "idle",
		CreatedAt:   time.Now(),
	}

	// Update service ledger with pipeline entry
	if err := service_ledger.UpdatePipelineEntry(
		pipelineID,
		req.Name,
		req.Description,
		req.Code,
		req.Branch,
		"idle",
		pipeline.CreatedAt.Format(time.RFC3339),
	); err != nil {
		// Log the error but don't fail the request since pipeline file was already created
		fmt.Printf("Warning: Failed to update service ledger: %v\n", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pipeline)
}

// GetPipelines retrieves all pipelines from the ~/.opencloud/pipelines directory
func GetPipelines(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	pipelineDir := filepath.Join(home, ".opencloud", "pipelines")

	// Check if directory exists
	if _, err := os.Stat(pipelineDir); os.IsNotExist(err) {
		// Return empty list if directory doesn't exist
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Pipeline{})
		return
	}

	// Read all files in the pipelines directory
	entries, err := os.ReadDir(pipelineDir)
	if err != nil {
		http.Error(w, "Failed to read pipelines directory", http.StatusInternalServerError)
		return
	}

	// Get all pipeline entries from the service ledger
	ledgerPipelines, err := service_ledger.GetAllPipelineEntries()
	if err != nil {
		fmt.Printf("Warning: Failed to read service ledger: %v\n", err)
		ledgerPipelines = make(map[string]service_ledger.PipelineEntry)
	}

	// Create a map of sanitized names to ledger entries for O(1) lookups
	// Note: Ledger stores original names, but filenames use sanitized names,
	// so we need to sanitize ledger names to match against filenames
	sanitizedNameToEntry := make(map[string]service_ledger.PipelineEntry)
	for _, entry := range ledgerPipelines {
		sanitizedName := sanitizePipelineName(entry.Name)
		sanitizedNameToEntry[sanitizedName] = entry
	}

	pipelines := []Pipeline{}

	// Process each shell script file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sh") {
			continue
		}

		// Derive pipeline name from filename (remove .sh extension)
		// This is already sanitized since the filename was created using sanitizePipelineName
		pipelineName := strings.TrimSuffix(entry.Name(), ".sh")

		// Try to find matching pipeline in ledger using the sanitized name map
		ledgerEntry, existsInLedger := sanitizedNameToEntry[pipelineName]

		// Create pipeline object, preferring ledger data when available
		var pipeline Pipeline
		if existsInLedger {
			createdAt, _ := time.Parse(time.RFC3339, ledgerEntry.CreatedAt)
			if createdAt.IsZero() {
				// Fallback to file mod time if parsing fails
				fileInfo, err := entry.Info()
				if err == nil {
					createdAt = fileInfo.ModTime()
				} else {
					createdAt = time.Now()
				}
			}
			pipeline = Pipeline{
				ID:          ledgerEntry.ID,
				Name:        ledgerEntry.Name,
				Description: ledgerEntry.Description,
				Code:        ledgerEntry.Code,
				Branch:      ledgerEntry.Branch,
				Status:      ledgerEntry.Status,
				CreatedAt:   createdAt,
			}
		} else {
			// Fallback to file-based data if not in ledger
			scriptPath := filepath.Join(pipelineDir, entry.Name())
			scriptData, err := os.ReadFile(scriptPath)
			if err != nil {
				continue // Skip files that can't be read
			}

			fileInfo, err := entry.Info()
			if err != nil {
				continue
			}

			pipelineID, err := generatePipelineID()
			if err != nil {
				continue
			}
			pipeline = Pipeline{
				ID:        pipelineID,
				Name:      pipelineName,
				Code:      string(scriptData),
				Status:    "idle",
				CreatedAt: fileInfo.ModTime(),
			}
		}

		pipelines = append(pipelines, pipeline)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pipelines)
}

// sanitizePipelineName removes potentially dangerous characters from pipeline names
// to prevent directory traversal and invalid filenames
func sanitizePipelineName(name string) string {
	// Remove any path separators and dangerous characters
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "~", "")

	// Only allow alphanumeric, hyphens, underscores, and dots using pre-compiled regex
	sanitized := pipelineNameRegex.ReplaceAllString(name, "-")

	// Trim leading/trailing hyphens or dots
	sanitized = strings.Trim(sanitized, "-.")

	return sanitized
}

// GetPipeline retrieves a single pipeline by its ID
func GetPipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract pipeline ID from URL path
	// URL format: /get-pipeline/{id}
	pipelineID := strings.TrimPrefix(r.URL.Path, "/get-pipeline/")
	if pipelineID == "" {
		http.Error(w, "Pipeline ID is required", http.StatusBadRequest)
		return
	}

	// Get pipeline entry from service ledger
	ledgerEntry, err := service_ledger.GetPipelineEntry(pipelineID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve pipeline: %v", err), http.StatusInternalServerError)
		return
	}

	if ledgerEntry == nil {
		http.Error(w, "Pipeline not found", http.StatusNotFound)
		return
	}

	// Parse created date
	createdAt, err := time.Parse(time.RFC3339, ledgerEntry.CreatedAt)
	if err != nil {
		// If parsing fails, log the error and use zero time to indicate unknown creation time
		fmt.Printf("Warning: Failed to parse creation time for pipeline %s: %v\n", pipelineID, err)
		createdAt = time.Time{}
	}

	// Convert ledger entry to API Pipeline format
	pipeline := Pipeline{
		ID:          ledgerEntry.ID,
		Name:        ledgerEntry.Name,
		Description: ledgerEntry.Description,
		Code:        ledgerEntry.Code,
		Branch:      ledgerEntry.Branch,
		Status:      ledgerEntry.Status,
		CreatedAt:   createdAt,
	}

	// Return pipeline as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pipeline)
}

// generatePipelineID creates a unique identifier for the pipeline
func generatePipelineID() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// UpdatePipelineRequest represents the request body for updating a pipeline
type UpdatePipelineRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Code        string `json:"code"`
	Branch      string `json:"branch"`
}

// UpdatePipeline updates an existing pipeline by its ID
func UpdatePipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract pipeline ID from URL path
	// URL format: /update-pipeline/{id}
	pipelineID := strings.TrimPrefix(r.URL.Path, "/update-pipeline/")
	if pipelineID == "" {
		http.Error(w, "Pipeline ID is required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req UpdatePipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Code == "" {
		http.Error(w, "Missing required fields: name and code", http.StatusBadRequest)
		return
	}

	// Set default branch if not provided
	if req.Branch == "" {
		req.Branch = "main"
	}

	// Get existing pipeline entry from service ledger to verify it exists
	existingEntry, err := service_ledger.GetPipelineEntry(pipelineID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve pipeline: %v", err), http.StatusInternalServerError)
		return
	}

	if existingEntry == nil {
		http.Error(w, "Pipeline not found", http.StatusNotFound)
		return
	}

	// Get home directory and pipelines directory
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	pipelineDir := filepath.Join(home, ".opencloud", "pipelines")

	// Update service ledger with the new pipeline data first
	// This ensures the ledger is updated before filesystem changes to maintain consistency
	// Preserve the original creation time
	if err := service_ledger.UpdatePipelineEntry(
		pipelineID,
		req.Name,
		req.Description,
		req.Code,
		req.Branch,
		existingEntry.Status, // Preserve existing status
		existingEntry.CreatedAt, // Preserve original creation time
	); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update service ledger: %v", err), http.StatusInternalServerError)
		return
	}

	// Delete old pipeline file if the name has changed
	// Note: Both names are sanitized using the same function to ensure consistent comparison
	oldSanitizedName := sanitizePipelineName(existingEntry.Name)
	newSanitizedName := sanitizePipelineName(req.Name)
	
	if oldSanitizedName != newSanitizedName {
		oldPipelineFileName := oldSanitizedName + ".sh"
		oldPipelinePath := filepath.Join(pipelineDir, oldPipelineFileName)
		
		// Remove old file if it exists
		if _, err := os.Stat(oldPipelinePath); err == nil {
			if err := os.Remove(oldPipelinePath); err != nil {
				// Log the specific error for debugging
				fmt.Printf("Warning: Failed to remove old pipeline file %s: %v\n", oldPipelinePath, err)
				http.Error(w, fmt.Sprintf("Failed to remove old pipeline file: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}

	// Write updated pipeline code to file
	pipelineFileName := newSanitizedName + ".sh"
	pipelinePath := filepath.Join(pipelineDir, pipelineFileName)
	
	if err := os.WriteFile(pipelinePath, []byte(req.Code), 0755); err != nil {
		// Log the specific error for debugging
		fmt.Printf("Error: Failed to write pipeline file %s: %v\n", pipelinePath, err)
		http.Error(w, fmt.Sprintf("Failed to update pipeline file: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse created date for response
	createdAt, err := time.Parse(time.RFC3339, existingEntry.CreatedAt)
	if err != nil {
		createdAt = time.Time{}
	}

	// Create response with updated pipeline details
	pipeline := Pipeline{
		ID:          pipelineID,
		Name:        req.Name,
		Description: req.Description,
		Code:        req.Code,
		Branch:      req.Branch,
		Status:      existingEntry.Status,
		CreatedAt:   createdAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pipeline)
}

// DeletePipeline deletes a pipeline by its ID
func DeletePipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract pipeline ID from URL path
	// URL format: /delete-pipeline/{id}
	pipelineID := strings.TrimPrefix(r.URL.Path, "/delete-pipeline/")
	if pipelineID == "" {
		http.Error(w, "Pipeline ID is required", http.StatusBadRequest)
		return
	}

	// Get pipeline entry from service ledger
	ledgerEntry, err := service_ledger.GetPipelineEntry(pipelineID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve pipeline: %v", err), http.StatusInternalServerError)
		return
	}

	if ledgerEntry == nil {
		http.Error(w, "Pipeline not found", http.StatusNotFound)
		return
	}

	// Get home directory and pipelines directory
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	pipelineDir := filepath.Join(home, ".opencloud", "pipelines")

	// Delete pipeline file
	sanitizedName := sanitizePipelineName(ledgerEntry.Name)
	pipelineFileName := sanitizedName + ".sh"
	pipelinePath := filepath.Join(pipelineDir, pipelineFileName)

	if _, err := os.Stat(pipelinePath); err == nil {
		if err := os.Remove(pipelinePath); err != nil {
			fmt.Printf("Warning: Failed to remove pipeline file %s: %v\n", pipelinePath, err)
			http.Error(w, fmt.Sprintf("Failed to remove pipeline file: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Delete from service ledger
	if err := service_ledger.DeletePipelineEntry(pipelineID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete pipeline from ledger: %v", err), http.StatusInternalServerError)
		return
	}

	// Delete log file if it exists
	logDir := filepath.Join(home, ".opencloud", "logs", "pipelines")
	logFileName := sanitizedName + ".log"
	logFilePath := filepath.Join(logDir, logFileName)
	if _, err := os.Stat(logFilePath); err == nil {
		os.Remove(logFilePath) // Best effort, don't fail if log deletion fails
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Pipeline deleted successfully",
	})
}

// PipelineLog represents a single pipeline execution log entry
type PipelineLog struct {
	Timestamp string `json:"timestamp"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	Status    string `json:"status"` // "success" or "error"
}

// RunPipeline executes a pipeline by its ID
func RunPipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract pipeline ID from URL path
	// URL format: /run-pipeline/{id}
	pipelineID := strings.TrimPrefix(r.URL.Path, "/run-pipeline/")
	if pipelineID == "" {
		http.Error(w, "Pipeline ID is required", http.StatusBadRequest)
		return
	}

	// Get pipeline entry from service ledger
	ledgerEntry, err := service_ledger.GetPipelineEntry(pipelineID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve pipeline: %v", err), http.StatusInternalServerError)
		return
	}

	if ledgerEntry == nil {
		http.Error(w, "Pipeline not found", http.StatusNotFound)
		return
	}

	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	// Construct path to pipeline script
	pipelineDir := filepath.Join(home, ".opencloud", "pipelines")
	sanitizedName := sanitizePipelineName(ledgerEntry.Name)
	pipelineFileName := sanitizedName + ".sh"
	pipelinePath := filepath.Join(pipelineDir, pipelineFileName)

	// Check if pipeline file exists
	if _, err := os.Stat(pipelinePath); os.IsNotExist(err) {
		http.Error(w, "Pipeline script file not found", http.StatusNotFound)
		return
	}

	// Update status to "running"
	if err := service_ledger.UpdatePipelineEntry(
		pipelineID,
		ledgerEntry.Name,
		ledgerEntry.Description,
		ledgerEntry.Code,
		ledgerEntry.Branch,
		"running",
		ledgerEntry.CreatedAt,
	); err != nil {
		fmt.Printf("Warning: Failed to update pipeline status: %v\n", err)
	}

	// Execute pipeline in a goroutine to avoid blocking
	go func() {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "/bin/bash", pipelinePath)

		// Capture output
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr

		// Store the command in the map so it can be stopped
		pipelineMutex.Lock()
		pipelineProcesses[pipelineID] = cmd
		pipelineMutex.Unlock()

		// Execute the pipeline
		err := cmd.Run()

		// Remove from running processes
		pipelineMutex.Lock()
		delete(pipelineProcesses, pipelineID)
		pipelineMutex.Unlock()

		// Determine status
		status := "success"
		if err != nil {
			status = "failed"
		}

		// Create log directory
		logDir := filepath.Join(home, ".opencloud", "logs", "pipelines")
		if mkErr := os.MkdirAll(logDir, 0755); mkErr != nil {
			fmt.Printf("Warning: failed to create log directory: %v\n", mkErr)
		}

		// Create log file
		logFileName := sanitizedName + ".log"
		logFilePath := filepath.Join(logDir, logFileName)
		logFile, fileErr := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if fileErr != nil {
			fmt.Printf("Warning: failed to open log file: %v\n", fileErr)
		} else {
			defer logFile.Close()
		}

		// Format log entry with timestamp separator
		timestamp := time.Now().Format(time.RFC3339)
		statusMarker := "SUCCESS"
		if status == "failed" {
			statusMarker = "ERROR"
		}
		logEntry := fmt.Sprintf("===EXECUTION_START:%s|%s===\n%s%s===EXECUTION_END===\n", 
			timestamp, statusMarker, out.String(), stderr.String())

		// Write to log file
		if logFile != nil {
			if _, writeErr := logFile.WriteString(logEntry); writeErr != nil {
				fmt.Printf("Warning: failed to write log file: %v\n", writeErr)
			}
		}

		// Update pipeline status to final status
		updatedEntry, err := service_ledger.GetPipelineEntry(pipelineID)
		if err == nil && updatedEntry != nil {
			if err := service_ledger.UpdatePipelineEntry(
				pipelineID,
				updatedEntry.Name,
				updatedEntry.Description,
				updatedEntry.Code,
				updatedEntry.Branch,
				status,
				updatedEntry.CreatedAt,
			); err != nil {
				fmt.Printf("Warning: Failed to update pipeline status: %v\n", err)
			}
		}
	}()

	// Return success immediately (pipeline runs in background)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Pipeline started successfully",
		"status":  "running",
	})
}

// GetPipelineLogs retrieves execution logs for a pipeline
func GetPipelineLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract pipeline ID from URL path
	// URL format: /get-pipeline-logs/{id}
	pipelineID := strings.TrimPrefix(r.URL.Path, "/get-pipeline-logs/")
	if pipelineID == "" {
		http.Error(w, "Pipeline ID is required", http.StatusBadRequest)
		return
	}

	// Get pipeline entry from service ledger
	ledgerEntry, err := service_ledger.GetPipelineEntry(pipelineID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve pipeline: %v", err), http.StatusInternalServerError)
		return
	}

	if ledgerEntry == nil {
		http.Error(w, "Pipeline not found", http.StatusNotFound)
		return
	}

	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	// Construct path to log file
	logDir := filepath.Join(home, ".opencloud", "logs", "pipelines")
	sanitizedName := sanitizePipelineName(ledgerEntry.Name)
	logFileName := sanitizedName + ".log"
	logFilePath := filepath.Join(logDir, logFileName)

	// Check if log file exists
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		// Return empty logs if file doesn't exist
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]PipelineLog{})
		return
	}

	// Read and parse log file
	file, err := os.Open(logFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open log file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	logs := []PipelineLog{}
	scanner := bufio.NewScanner(file)
	
	var currentLog *PipelineLog
	var outputBuffer strings.Builder
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Check for execution start marker
		if strings.HasPrefix(line, "===EXECUTION_START:") {
			// Parse timestamp and status from marker
			marker := strings.TrimPrefix(line, "===EXECUTION_START:")
			marker = strings.TrimSuffix(marker, "===")
			parts := strings.Split(marker, "|")
			
			if len(parts) == 2 {
				timestamp := parts[0]
				status := "success"
				if strings.ToLower(parts[1]) == "error" {
					status = "error"
				}
				
				currentLog = &PipelineLog{
					Timestamp: timestamp,
					Status:    status,
				}
				outputBuffer.Reset()
			}
		} else if strings.HasPrefix(line, "===EXECUTION_END===") {
			// End of log entry
			if currentLog != nil {
				output := outputBuffer.String()
				// Split into output and error based on status
				if currentLog.Status == "error" {
					currentLog.Error = output
				} else {
					currentLog.Output = output
				}
				logs = append(logs, *currentLog)
				currentLog = nil
			}
		} else if currentLog != nil {
			// Accumulate output lines
			if outputBuffer.Len() > 0 {
				outputBuffer.WriteString("\n")
			}
			outputBuffer.WriteString(line)
		}
	}

	if err := scanner.Err(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to read log file: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// StopPipeline stops a running pipeline
func StopPipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract pipeline ID from URL path
	// URL format: /stop-pipeline/{id}
	pipelineID := strings.TrimPrefix(r.URL.Path, "/stop-pipeline/")
	if pipelineID == "" {
		http.Error(w, "Pipeline ID is required", http.StatusBadRequest)
		return
	}

	// Get the running process
	pipelineMutex.Lock()
	cmd, exists := pipelineProcesses[pipelineID]
	if exists {
		delete(pipelineProcesses, pipelineID)
	}
	pipelineMutex.Unlock()

	if !exists || cmd.Process == nil {
		http.Error(w, "Pipeline is not running", http.StatusBadRequest)
		return
	}

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop pipeline: %v", err), http.StatusInternalServerError)
		return
	}

	// Update status to "idle"
	ledgerEntry, err := service_ledger.GetPipelineEntry(pipelineID)
	if err == nil && ledgerEntry != nil {
		if err := service_ledger.UpdatePipelineEntry(
			pipelineID,
			ledgerEntry.Name,
			ledgerEntry.Description,
			ledgerEntry.Code,
			ledgerEntry.Branch,
			"idle",
			ledgerEntry.CreatedAt,
		); err != nil {
			fmt.Printf("Warning: Failed to update pipeline status: %v\n", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Pipeline stopped successfully",
	})
}
