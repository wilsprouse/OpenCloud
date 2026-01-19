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

	// Save metadata to JSON file
	metadataPath := filepath.Join(pipelineDir, sanitizedName+".json")
	metadataJSON, err := json.Marshal(pipeline)
	if err != nil {
		http.Error(w, "Failed to create metadata", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		http.Error(w, "Failed to save metadata", http.StatusInternalServerError)
		return
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

	pipelines := []Pipeline{}

	// Process each JSON metadata file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Read metadata file
		metadataPath := filepath.Join(pipelineDir, entry.Name())
		metadataData, err := os.ReadFile(metadataPath)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Parse metadata
		var pipeline Pipeline
		if err := json.Unmarshal(metadataData, &pipeline); err != nil {
			continue // Skip files that can't be parsed
		}

		// Read the corresponding shell script to get the latest code
		scriptName := strings.TrimSuffix(entry.Name(), ".json") + ".sh"
		scriptPath := filepath.Join(pipelineDir, scriptName)
		scriptData, err := os.ReadFile(scriptPath)
		if err == nil {
			pipeline.Code = string(scriptData)
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

// generatePipelineID creates a unique identifier for the pipeline
func generatePipelineID() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}
