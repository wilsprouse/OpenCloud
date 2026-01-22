package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

var pipelineNameRegex = regexp.MustCompile(`[^a-zA-Z0-9\-_.]`)

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
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/get-pipeline/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Pipeline ID is required", http.StatusBadRequest)
		return
	}
	
	pipelineID := parts[0]

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
		// If parsing fails, use current time as fallback
		createdAt = time.Now()
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
