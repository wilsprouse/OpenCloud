package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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
	pipelineID := generatePipelineID()

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
		Name:        sanitizedName,
		Description: req.Description,
		Code:        req.Code,
		Branch:      req.Branch,
		Status:      "idle",
		CreatedAt:   time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pipeline)
}

// sanitizePipelineName removes potentially dangerous characters from pipeline names
// to prevent directory traversal and invalid filenames
func sanitizePipelineName(name string) string {
	// Remove any path separators and dangerous characters
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "~", "")

	// Only allow alphanumeric, hyphens, underscores, and dots
	reg := regexp.MustCompile(`[^a-zA-Z0-9\-_.]`)
	sanitized := reg.ReplaceAllString(name, "-")

	// Trim leading/trailing hyphens or dots
	sanitized = strings.Trim(sanitized, "-.")

	return sanitized
}

// generatePipelineID creates a unique identifier for the pipeline
func generatePipelineID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
