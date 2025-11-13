package api

import (
	"net/http"
	"context"
	"encoding/json"
	"time"
	"os"
	"io"
	"os/exec"
	"bytes"
	"fmt"
	"strings"
	"path/filepath"
	"github.com/docker/docker/api/types/image"
    "github.com/docker/docker/client"
)

type FunctionItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Runtime      string    `json:"runtime"`
	Status       string    `json:"status"`
	LastModified time.Time `json:"lastModified"`
	Invocations  int       `json:"invocations"`
	MemorySize   int       `json:"memorySize"`
	Timeout      int       `json:"timeout"`
}

type UpdateFunctionRequest struct {
	Name       string `json:"name"`
	Runtime    string `json:"runtime"`
	Code       string `json:"code"`
	MemorySize int    `json:"memorySize"`
	Timeout    int    `json:"timeout"`
}

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

func GetContainers(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        panic(err)
    }

    images, err := cli.ImageList(ctx, image.ListOptions{
        All: true, // include intermediate images
    })
    if err != nil {
        panic(err)
    }

    /*for _, img := range images {
		fmt.Printf("ID: %s\n", img.ID[7:19])
		fmt.Printf("RepoTags: %v\n", img.RepoTags)
		fmt.Printf("RepoDigests: %v\n", img.RepoDigests)
		fmt.Printf("Created: %d\n", img.Created)
		fmt.Printf("Size: %.2f MB\n", float64(img.Size)/1_000_000)
		fmt.Printf("Virtual Size: %.2f MB\n", float64(img.VirtualSize)/1_000_000)
		fmt.Printf("Labels: %v\n", img.Labels)
		fmt.Printf("Containers: %d\n\n", img.Containers)
    }*/

	// Encode the images as JSON and write to response
	if err := json.NewEncoder(w).Encode(images); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func ListFunctions(w http.ResponseWriter, r *http.Request) {
	home, err := os.UserHomeDir()
	functionDir := filepath.Join(home, ".opencloud", "functions")

	files, err := os.ReadDir(functionDir)
	if err != nil {
		http.Error(w, "Failed to read functions directory", http.StatusInternalServerError)
		return
	}

	var functions []FunctionItem

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		fn := FunctionItem{
			ID:           file.Name(),
			Name:         file.Name(),
			Runtime:      detectRuntime(file.Name()),
			Status:       "active",
			LastModified: info.ModTime(),
			Invocations:  0,
			MemorySize:   128,
			Timeout:      30,
		}

		functions = append(functions, fn)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(functions)
}

func InvokeFunction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse function name from query string, e.g. ?name=hello.py
	fnName := r.URL.Query().Get("name")
	if fnName == "" {
		http.Error(w, "Missing function name", http.StatusBadRequest)
		return
	}

	// Locate the function file
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to resolve home directory", http.StatusInternalServerError)
		return
	}
	fnPath := filepath.Join(home, ".opencloud", "functions", fnName)

	// Check that it exists
	if _, err := os.Stat(fnPath); os.IsNotExist(err) {
		http.Error(w, "Function not found", http.StatusNotFound)
		return
	}

	// Detect runtime from file extension
	runtime := detectRuntime(fnName)

	// Choose interpreter or build command
	var cmd *exec.Cmd
	switch runtime {
	case "python":
		cmd = exec.CommandContext(ctx, "python3", fnPath)
	case "nodejs":
		cmd = exec.CommandContext(ctx, "node", fnPath)
	case "go":
		// Build and run Go file
		cmd = exec.CommandContext(ctx, "go", "run", fnPath)
	case "ruby":
		cmd = exec.CommandContext(ctx, "ruby", fnPath)
	default:
		http.Error(w, "Unsupported runtime", http.StatusBadRequest)
		return
	}

	// Optional: pass JSON input (if provided in POST body)
	if r.Method == http.MethodPost {
		var input map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&input); err == nil {
			inputJSON, _ := json.Marshal(input)
			cmd.Stdin = bytes.NewReader(inputJSON)
		}
	}

	// Capture output
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr


	err = cmd.Run()
	if err != nil {
		http.Error(w, "Execution error: "+stderr.String(), http.StatusInternalServerError)
		return
	}

	fmt.Println(out.String())

	// Send JSON response
	resp := map[string]string{
		"output": out.String(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// DeleteFunction removes a user function file by name (e.g. /delete-function?name=hello.py)
func DeleteFunction(w http.ResponseWriter, r *http.Request) {
	fnName := r.URL.Query().Get("name")
	if fnName == "" {
		http.Error(w, "Missing function name", http.StatusBadRequest)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to resolve home directory", http.StatusInternalServerError)
		return
	}

	fnPath := filepath.Join(home, ".opencloud", "functions", fnName)

	if _, err := os.Stat(fnPath); os.IsNotExist(err) {
		http.Error(w, "Function not found", http.StatusNotFound)
		return
	}

	if err := os.Remove(fnPath); err != nil {
		http.Error(w, "Failed to delete function: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]string{
		"status":  "success",
		"message": "Function deleted successfully",
		"name":    fnName,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetFunction handles routes like /get-function/<name>
func GetFunction(w http.ResponseWriter, r *http.Request) {
	// Extract function name from path after /get-function/
	fnName := strings.TrimPrefix(r.URL.Path, "/get-function/")
	if fnName == "" || fnName == "/get-function" {
		http.Error(w, "Missing function name", http.StatusBadRequest)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to resolve home directory", http.StatusInternalServerError)
		return
	}

	fnPath := filepath.Join(home, ".opencloud", "functions", fnName)
	info, err := os.Stat(fnPath)
	if os.IsNotExist(err) {
		http.Error(w, "Function not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Error checking function file", http.StatusInternalServerError)
		return
	}

	code, err := os.ReadFile(fnPath)
	if err != nil {
		http.Error(w, "Failed to read function file", http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"name":         fnName,
		"path":         fnPath,
		"Invocations":	0,
		"runtime":      detectRuntime(fnName),
		"lastModified": info.ModTime().Format(time.RFC3339),
		"sizeBytes":    info.Size(),
		"code":         string(code),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func UpdateFunction(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract function ID from URL path
	id := strings.TrimPrefix(r.URL.Path, "/update-function/")
	if id == "" {
		http.Error(w, "Function ID not provided", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req UpdateFunctionRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Resolve file path
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}
	fnDir := filepath.Join(home, ".opencloud", "functions")
	fnPath := filepath.Join(fnDir, id)

	// Check if function exists
	if _, err := os.Stat(fnPath); os.IsNotExist(err) {
		http.Error(w, "Function not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Failed to read function", http.StatusInternalServerError)
		return
	}

	// Update function code
	if err := os.WriteFile(fnPath, []byte(req.Code), 0644); err != nil {
		http.Error(w, "Failed to update function code", http.StatusInternalServerError)
		return
	}

	// Respond with updated function info
	resp := map[string]interface{}{
		"id":           id,
		"name":         req.Name,
		"runtime":      req.Runtime,
		"memorySize":   req.MemorySize,
		"timeout":      req.Timeout,
		"lastModified": time.Now().Format(time.RFC3339),
		"invocations":  0, //getInvocationCount(id), // implement this if you track invocations
		"code":         req.Code,
		"status":       "active",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
