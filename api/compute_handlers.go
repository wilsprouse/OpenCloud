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

	// Add log entry to host system with timestamp separator
	timestamp := time.Now().Format(time.RFC3339)
	hasError := stderr.Len() > 0
	statusMarker := "SUCCESS"
	if hasError {
		statusMarker = "ERROR"
	}
	logEntry := fmt.Sprintf("===EXECUTION_START:%s|%s===\n%s%s===EXECUTION_END===\n", timestamp, statusMarker, out.String(), stderr.String())

	if logFile != nil {
		if _, writeErr := logFile.WriteString(logEntry); writeErr != nil {
			fmt.Printf("Warning: failed to write log file: %v\n", writeErr)
		}
	}
	
	fmt.Print(out.String() + stderr.String())

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

	// Get function entry from service ledger to check if it has a cron trigger
	functionEntry, err := service_ledger.GetFunctionEntry(fnName)
	if err != nil {
		fmt.Printf("Warning: Failed to retrieve function entry from service ledger: %v\n", err)
	}

	// Remove the function file first
	if err := os.Remove(fnPath); err != nil {
		http.Error(w, "Failed to delete function: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// After successful file deletion, remove cron job if the function has a cron trigger
	if functionEntry != nil && functionEntry.Trigger == "cron" {
		if err := removeCron(fnPath); err != nil {
			fmt.Printf("Warning: Failed to remove cron job: %v\n", err)
		}
	}

	// Remove log files
	logsDir := filepath.Join(home, ".opencloud", "logs")
	
	// Remove execution log file (~/.opencloud/logs/functions/{baseName}.log)
	// Strip extension from function name to match how logs are created
	baseName := strings.TrimSuffix(fnName, filepath.Ext(fnName))
	executionLogPath := filepath.Join(logsDir, "functions", baseName+".log")
	if err := os.Remove(executionLogPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: Failed to remove execution log file: %v\n", err)
	}

	// Remove cron log file (~/.opencloud/logs/functions/{functionName}.log)
	cronLogPath := filepath.Join(logsDir, "functions", fmt.Sprintf("%s.log", fnName))
	if err := os.Remove(cronLogPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: Failed to remove cron log file: %v\n", err)
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

	fnDir := filepath.Join(home, ".opencloud", "logs", "functions")

	if err := os.MkdirAll(fnDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Cron job to append
	// Use function-specific log file based on the base filename
	//fileName := filepath.Base(filePath)
	//logFile := filepath.Join(fnDir, fmt.Sprintf("%s.log", fileName))
	//baseName := strings.TrimSuffix(fileName, logFile.Ext(fileName))
	//fmt.Sprint("%s", baseName)

	fileName := filepath.Base(filePath)                 // hello.py
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName)) // hello
	logFile := filepath.Join(fnDir, baseName + ".log")  // hello.log

	newCronJob := fmt.Sprintf("%s %s %s >> %s 2>&1", schedule, detectRuntime(filePath), filePath, logFile)

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

// removeCron removes a cron job entry for the given file path from the user's crontab
func removeCron(filePath string) error {
	// Get current crontab
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	out := string(output)

	// Handle case where user has no crontab
	if err != nil {
		if strings.Contains(out, "no crontab for") {
			fmt.Println("No crontab found — nothing to remove.")
			return nil // No crontab means nothing to remove
		} else {
			// Real error → stop
			return fmt.Errorf("Unexpected crontab error: %v\n%s", err, output)
		}
	}

	currentCrontab := out

	// Build the expected cron job pattern to remove
	// We need to match any line that contains the filePath
	lines := strings.Split(currentCrontab, "\n")
	var updatedLines []string
	removed := false

	for _, line := range lines {
		// Skip lines that contain the filePath as a command to execute
		// Check both " {filePath} " (with spaces) and " {filePath} >>" (followed by output redirection)
		// to ensure we're matching the actual command, not just a substring
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" && (strings.Contains(line, " "+filePath+" ") || strings.Contains(line, " "+filePath+" >>")) {
			removed = true
			fmt.Printf("Removing cron job: %s\n", line)
			continue
		}
		// Keep all other non-empty lines
		if line != "" {
			updatedLines = append(updatedLines, line)
		}
	}

	if !removed {
		fmt.Println("No matching cron job found — nothing to remove.")
		return nil
	}

	// Build updated crontab
	updatedCrontab := strings.Join(updatedLines, "\n")
	// Ensure there's a trailing newline if content exists
	if len(updatedLines) > 0 && !strings.HasSuffix(updatedCrontab, "\n") {
		updatedCrontab += "\n"
	}

	// Write updated crontab
	cmd = exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(updatedCrontab)
	output, err = cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("error updating crontab: %v\n%s", err, output)
	}

	fmt.Println("Cron job removed successfully.")
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

	// Determine if we need to rename the file
	// req.Name should be the new name with extension already included
	newFileName := req.Name
	needsRename := (id != newFileName)
	newFnPath := filepath.Join(fnDir, newFileName)

	// If renaming, check that the new file doesn't already exist
	if needsRename {
		if _, err := os.Stat(newFnPath); err == nil {
			http.Error(w, "A function with the new name already exists", http.StatusConflict)
			return
		}
	}

	// Get old cron trigger status before making changes
	oldFunctionEntry, _ := service_ledger.GetFunctionEntry(id)
	hadCronTrigger := oldFunctionEntry != nil && oldFunctionEntry.Trigger == "cron"

	// Remove old cron job if it existed
	if hadCronTrigger {
		if err := removeCron(fnPath); err != nil {
			fmt.Printf("Warning: Failed to remove old cron job: %v\n", err)
		}
	}

	// Update function code (write to the current path first)
	if err := os.WriteFile(fnPath, []byte(req.Code), 0644); err != nil {
		http.Error(w, "Failed to update function code", http.StatusInternalServerError)
		return
	}

	// If renaming the function, rename the file
	if needsRename {
		if err := os.Rename(fnPath, newFnPath); err != nil {
			http.Error(w, "Failed to rename function file", http.StatusInternalServerError)
			return
		}

		// Delete old entry from service ledger
		if err := service_ledger.DeleteFunctionEntry(id); err != nil {
			fmt.Printf("Warning: Failed to delete old service ledger entry: %v\n", err)
		}

		// Rename log file if it exists
		logsDir := filepath.Join(home, ".opencloud", "logs", "functions")
		oldBaseName := strings.TrimSuffix(id, filepath.Ext(id))
		newBaseName := strings.TrimSuffix(newFileName, filepath.Ext(newFileName))
		oldLogPath := filepath.Join(logsDir, oldBaseName+".log")
		newLogPath := filepath.Join(logsDir, newBaseName+".log")
		
		if _, err := os.Stat(oldLogPath); err == nil {
			if err := os.Rename(oldLogPath, newLogPath); err != nil {
				fmt.Printf("Warning: Failed to rename log file: %v\n", err)
			}
		}

		// Update path references to use new file name
		fnPath = newFnPath
		id = newFileName
	}

	// Update the service ledger with function metadata
	trigger := ""
	schedule := ""
	if req.Trigger != nil && req.Trigger.Enabled {
		trigger = req.Trigger.Type
		schedule = req.Trigger.Schedule
		// Add cron job to system crontab with the new file path
		if err := addCron(fnPath, req.Trigger.Schedule); err != nil {
			http.Error(w, "Failed to save cron trigger metadata", http.StatusInternalServerError)
			return
		}
	}

	// Update service ledger with function entry using the new filename
	if err := service_ledger.UpdateFunctionEntry(id, req.Runtime, trigger, schedule, req.Code); err != nil {
		// Log the error but don't fail the request since function code was already updated
		fmt.Printf("Warning: Failed to update service ledger: %v\n", err)
	}

	// Respond with updated function info
	resp := map[string]interface{}{
		"id":           id,
		"name":         id,
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

	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to resolve home directory", http.StatusInternalServerError)
		return
	}

	// Construct log file path: remove extension from function name and add .log
	baseName := strings.TrimSuffix(fnName, filepath.Ext(fnName))
	logFileName := baseName + ".log"
	logFilePath := filepath.Join(home, ".opencloud", "logs", "functions", logFileName)

	// Read log file
	logContent, err := os.ReadFile(logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty array if file doesn't exist (compatible with frontend)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]service_ledger.FunctionLog{})
			return
		}
		http.Error(w, "Failed to read log file", http.StatusInternalServerError)
		return
	}

	// Parse log file to extract individual executions
	// Each execution is wrapped with ===EXECUTION_START:<timestamp>|<status>=== and ===EXECUTION_END===
	logText := string(logContent)
	executions := []service_ledger.FunctionLog{}
	
	// Split by execution markers
	parts := strings.Split(logText, "===EXECUTION_START:")
	for _, part := range parts {
		if part == "" {
			continue
		}
		
		// Find the end marker
		endIdx := strings.Index(part, "===EXECUTION_END===")
		if endIdx == -1 {
			continue
		}
		
		// Extract timestamp and status from header: <timestamp>|<status>===\n
		headerEndMarker := "===\n"
		timestampEndIdx := strings.Index(part, headerEndMarker)
		if timestampEndIdx == -1 {
			continue
		}
		
		// Parse header: "timestamp|status"
		header := strings.TrimSpace(part[:timestampEndIdx])
		headerParts := strings.Split(header, "|")
		if len(headerParts) < 2 {
			continue
		}
		
		timestamp := headerParts[0]
		status := strings.ToLower(headerParts[1])
		
		// Extract output (everything between header and end marker)
		output := part[timestampEndIdx+len(headerEndMarker):endIdx]
		
		executions = append(executions, service_ledger.FunctionLog{
			Timestamp: timestamp,
			Output:    output,
			Status:    status,
		})
	}
	
	// Return only the last execution (most recent one)
	var logs []service_ledger.FunctionLog
	if len(executions) > 0 {
		logs = []service_ledger.FunctionLog{executions[len(executions)-1]}
	} else {
		// Fallback: if no structured logs found, return empty array
		logs = []service_ledger.FunctionLog{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
