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
	"github.com/WavexSoftware/OpenCloud/service_ledger"
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
	Trigger      *Trigger  `json:"trigger,omitempty"`
}

type Trigger struct {
	Type     string `json:"type"`     // "cron" for now
	Schedule string `json:"schedule"` // CRON expression like "0 0 * * *"
	Enabled  bool   `json:"enabled"`
}

type UpdateFunctionRequest struct {
	Name       string   `json:"name"`
	Runtime    string   `json:"runtime"`
	Code       string   `json:"code"`
	MemorySize int      `json:"memorySize"`
	Timeout    int      `json:"timeout"`
	Trigger    *Trigger `json:"trigger,omitempty"`
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

	// Get all function entries from the service ledger
	ledgerFunctions, err := service_ledger.GetAllFunctionEntries()
	if err != nil {
		fmt.Printf("Warning: Failed to read service ledger: %v\n", err)
		ledgerFunctions = make(map[string]service_ledger.FunctionEntry)
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
			Trigger:      nil,
		}

		// Check if this function has metadata in the service ledger
		if ledgerEntry, exists := ledgerFunctions[file.Name()]; exists {
			// If the function has a trigger and schedule in the ledger, populate it.
			// The presence of trigger and schedule indicates the trigger is enabled.
			if ledgerEntry.Trigger != "" && ledgerEntry.Schedule != "" {
				fn.Trigger = &Trigger{
					Type:     ledgerEntry.Trigger,
					Schedule: ledgerEntry.Schedule,
					Enabled:  true,
				}
			}
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

	logDir := filepath.Join(home, ".opencloud", "logs", "functions")
	if mkErr := os.MkdirAll(logDir, 0755); mkErr != nil {
		fmt.Printf("Warning: failed to create log directory: %v\n", mkErr)
	}

	// Change function_name.extenesion to function_name.log
	baseName := strings.TrimSuffix(fnName, filepath.Ext(fnName))
	logFileName := baseName + ".log"
	logFilePath := filepath.Join(logDir, logFileName)

	logFile, fileErr := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr != nil {
		fmt.Printf("Warning: failed to open log file: %v\n", fileErr)
	} else {
		defer logFile.Close()
	}

	// Add log entry to host system
	logEntry := fmt.Sprintf("%s", out.String())

	if logFile != nil {
		if _, writeErr := logFile.WriteString(logEntry); writeErr != nil {
			fmt.Printf("Warning: failed to write log file: %v\n", writeErr)
		}
	}
	
	// Create log entry
	log := service_ledger.FunctionLog{
		Timestamp: time.Now().Format(time.RFC3339),
		Output:    out.String(),
		Error:     stderr.String(),
		Status:    "success",
	}
	
	if err != nil {
		log.Status = "error"
		// Store the error log
		if logErr := service_ledger.AddFunctionLog(fnName, log); logErr != nil {
			fmt.Printf("Warning: Failed to store function log: %v\n", logErr)
		}
		http.Error(w, "Execution error: "+stderr.String(), http.StatusInternalServerError)
		return
	}

	// Store the success log
	if logErr := service_ledger.AddFunctionLog(fnName, log); logErr != nil {
		fmt.Printf("Warning: Failed to store function log: %v\n", logErr)
	}

	fmt.Printf(out.String())

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

	// Delete function entry from service ledger
	if err := service_ledger.DeleteFunctionEntry(fnName); err != nil {
		// Log the error but don't fail the request
		fmt.Printf("Warning: Failed to delete function from service ledger: %v\n", err)
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

	// Get trigger information from service ledger
	var trigger *Trigger
	if ledgerEntry, err := service_ledger.GetFunctionEntry(fnName); err == nil && ledgerEntry != nil {
		// The presence of trigger and schedule in the ledger indicates the trigger is enabled
		if ledgerEntry.Trigger != "" && ledgerEntry.Schedule != "" {
			trigger = &Trigger{
				Type:     ledgerEntry.Trigger,
				Schedule: ledgerEntry.Schedule,
				Enabled:  true,
			}
		}
	}

	resp := map[string]interface{}{
		"name":         fnName,
		"path":         fnPath,
		"Invocations":	0,
		"runtime":      detectRuntime(fnName),
		"lastModified": info.ModTime().Format(time.RFC3339),
		"sizeBytes":    info.Size(),
		"code":         string(code),
		"trigger":      trigger,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func addCron(filePath string, schedule string) error {

	fmt.Println(schedule)

	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	out := string(output)

	// Handle case where user has no crontab yet
	if err != nil {
		if strings.Contains(out, "no crontab for") {
			fmt.Println("No crontab found — continuing with empty crontab.")
			out = "" // treat as empty crontab
		} else {
			// Real error → stop
			return fmt.Errorf("Unexpected crontab error: %v\n%s", err, output)
		}
	}

	currentCrontab := out

	// Resolve file path
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Home dir not grabbed")
		return nil
	}

	fnDir := filepath.Join(home, ".opencloud", "logs")

	if err := os.MkdirAll(fnDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Cron job to append
	//newCronJob := "* * * * * echo \"Hello from Go cron!\" >> /tmp/go_cron.log"
	newCronJob := fmt.Sprintf("%s %s %s >> %s/go_cron_output.log 2>&1", schedule, detectRuntime(filePath), filePath, fnDir)

	// Prevent duplicate entries
	if strings.Contains(currentCrontab, newCronJob) {
		fmt.Println("Cron job already exists — skipping add.")
		return nil
	}

	// Add newline only if needed
	if !strings.HasSuffix(currentCrontab, "\n") && currentCrontab != "" {
		currentCrontab += "\n"
	}

	updatedCrontab := currentCrontab + newCronJob + "\n"

	// Write new crontab
	cmd = exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(updatedCrontab)
	output, err = cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("error updating crontab: %v\n%s", err, output)
	}

	fmt.Println("Crontab updated successfully.")
	return nil
}

func CreateFunction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		Name    string `json:"name"`
		Runtime string `json:"runtime"`
		Code    string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Runtime == "" || req.Code == "" {
		http.Error(w, "Missing required fields: name, runtime, and code", http.StatusBadRequest)
		return
	}

	// Determine file extension based on runtime
	var extension string
	runtimeLower := strings.ToLower(req.Runtime)
	if strings.Contains(runtimeLower, "python") {
		extension = ".py"
	} else if strings.Contains(runtimeLower, "node") || strings.Contains(runtimeLower, "javascript") {
		extension = ".js"
	} else if strings.HasPrefix(runtimeLower, "go") || runtimeLower == "golang" {
		extension = ".go"
	} else if strings.Contains(runtimeLower, "ruby") {
		extension = ".rb"
	} else {
		http.Error(w, "Unsupported runtime: "+req.Runtime, http.StatusBadRequest)
		return
	}

	// Create function filename
	functionFileName := req.Name
	if !strings.HasSuffix(functionFileName, extension) {
		functionFileName += extension
	}

	// Resolve file path
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}
	fnDir := filepath.Join(home, ".opencloud", "functions")
	fnPath := filepath.Join(fnDir, functionFileName)

	// Create the functions directory if it doesn't exist
	if err := os.MkdirAll(fnDir, 0755); err != nil {
		http.Error(w, "Failed to create functions directory", http.StatusInternalServerError)
		return
	}

	// Check if function already exists (both file and ledger)
	if _, err := os.Stat(fnPath); err == nil {
		http.Error(w, "Function already exists", http.StatusConflict)
		return
	}

	// Also check if function exists in service ledger
	if existingEntry, err := service_ledger.GetFunctionEntry(functionFileName); err == nil && existingEntry != nil {
		http.Error(w, "Function already exists in service ledger", http.StatusConflict)
		return
	}

	// Write function code to file
	if err := os.WriteFile(fnPath, []byte(req.Code), 0644); err != nil {
		http.Error(w, "Failed to create function file", http.StatusInternalServerError)
		return
	}

	// Update service ledger with function entry
	if err := service_ledger.UpdateFunctionEntry(functionFileName, req.Runtime, "", "", req.Code); err != nil {
		// Log the error but don't fail the request since function file was already created
		fmt.Printf("Warning: Failed to update service ledger: %v\n", err)
	}

	// Respond with created function info
	resp := map[string]interface{}{
		"id":           functionFileName,
		"name":         functionFileName,
		"runtime":      req.Runtime,
		"lastModified": time.Now().Format(time.RFC3339),
		"status":       "active",
		"message":      "Function created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
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

	// Update the service ledger with function metadata
	trigger := ""
	schedule := ""
	if req.Trigger != nil && req.Trigger.Enabled {
		trigger = req.Trigger.Type
		schedule = req.Trigger.Schedule
		// Add cron job to system crontab
		if err := addCron(fnPath, req.Trigger.Schedule); err != nil {
			http.Error(w, "Failed to save cron trigger metadata", http.StatusInternalServerError)
			return
		}
	}

	// Update service ledger with function entry
	// Note: If this fails after addCron succeeds, the cron entry may remain orphaned.
	// A future improvement would be to implement transaction-like cleanup on failure.
	if err := service_ledger.UpdateFunctionEntry(id, req.Runtime, trigger, schedule, req.Code); err != nil {
		// Log the error but don't fail the request since function code was already updated
		fmt.Printf("Warning: Failed to update service ledger: %v\n", err)
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
		"trigger":      req.Trigger,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetFunctionLogs retrieves the logs for a specific function
func GetFunctionLogs(w http.ResponseWriter, r *http.Request) {
	// Extract function name from path after /get-function-logs/
	fnName := strings.TrimPrefix(r.URL.Path, "/get-function-logs/")
	if fnName == "" || fnName == "/get-function-logs" {
		http.Error(w, "Missing function name", http.StatusBadRequest)
		return
	}

	// Get function entry from service ledger
	entry, err := service_ledger.GetFunctionEntry(fnName)
	if err != nil {
		http.Error(w, "Failed to retrieve function logs", http.StatusInternalServerError)
		return
	}

	if entry == nil {
		http.Error(w, "Function not found", http.StatusNotFound)
		return
	}

	// Return logs (empty array if no logs)
	logs := entry.Logs
	if logs == nil {
		logs = []service_ledger.FunctionLog{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
