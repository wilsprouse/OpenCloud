package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	nettypes "go.podman.io/common/libnetwork/types"
	"github.com/containers/podman/v5/pkg/bindings"

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

// GetContainers lists all containers from Podman and returns their state along
// with common runtime metrics such as memory usage and the host PID of the main
// container process.
func GetContainers(w http.ResponseWriter, r *http.Request) {
	conn, err := podmanConnection(context.Background())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman: %v", err), http.StatusInternalServerError)
		return
	}

	containerList, err := containers.List(conn, new(containers.ListOptions).WithAll(true))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list containers: %v", err), http.StatusInternalServerError)
		return
	}

	// Initialize to an empty slice so JSON encodes as [] rather than null
	// when no containers are present.
	result := make([]ContainerInfo, 0, len(containerList))
	for _, ctr := range containerList {
		names := append([]string(nil), ctr.Names...)
		if len(names) == 0 {
			names = []string{ctr.ID}
		}

		pid := uint32(0)
		if ctr.Pid > 0 {
			pid = uint32(ctr.Pid)
		}

		ci := ContainerInfo{
			ID:      ctr.ID,
			Names:   names,
			Image:   ctr.Image,
			State:   ctr.State,
			Status:  ctr.Status,
			Created: ctr.Created.Unix(),
			Labels:  ctr.Labels,
			PID:     pid,
		}

		if ci.PID > 0 {
			ci.MemoryUsageBytes = containerMemoryUsageBytes(ci.PID)
		}

		result = append(result, ci)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// containerMemoryUsageBytes reads the resident set size (RSS) of the process
// with the given PID from the Linux /proc filesystem and returns it in bytes.
// Returns 0 when the PID is zero, the file cannot be read, or the value cannot
// be parsed.
func containerMemoryUsageBytes(pid uint32) int64 {
	if pid == 0 {
		return 0
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			var kb int64
			if n, err := fmt.Sscanf(strings.TrimPrefix(line, "VmRSS:"), "%d", &kb); err != nil || n != 1 {
				return 0
			}
			return kb * 1024
		}
	}
	return 0
}

// PullAndRunRequest represents the JSON payload for pulling an image and starting a container.
type PullAndRunRequest struct {
	// Image is the container image to pull and run (e.g. "nginx:latest").
	Image string `json:"image"`
	// Name is an optional human-readable name to assign to the container.
	Name string `json:"name,omitempty"`
	// Ports is a list of port mappings in "hostPort:containerPort" format.
	Ports []string `json:"ports,omitempty"`
	// Env is a list of environment variables in "KEY=VALUE" or "KEY" format.
	Env []string `json:"env,omitempty"`
	// Volumes is a list of volume mounts in "hostPath:containerPath" format.
	Volumes []string `json:"volumes,omitempty"`
	// RestartPolicy is the container restart policy ("no", "always", "on-failure", "unless-stopped").
	RestartPolicy string `json:"restartPolicy,omitempty"`
	// AutoRemove removes the container automatically when it exits.
	AutoRemove bool `json:"autoRemove,omitempty"`
	// Command overrides the default container entrypoint command.
	Command string `json:"command,omitempty"`
}

// validRestartPolicies lists the restart policies accepted by nerdctl.
var validRestartPolicies = map[string]bool{
	"no":             true,
	"always":         true,
	"on-failure":     true,
	"unless-stopped": true,
}

// validateContainerName checks a container name for unsafe or invalid characters.
// Names must start with a letter or digit and may only contain letters, digits,
// hyphens, underscores, and dots.
func validateContainerName(name string) string {
	if len(name) == 0 {
		return "container name must not be empty"
	}
	first := name[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || (first >= '0' && first <= '9')) {
		return "container name must start with a letter or digit"
	}
	for _, c := range name[1:] {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return fmt.Sprintf("container name contains invalid character %q", c)
		}
	}
	return ""
}

// validatePortMapping checks a port mapping string for invalid or dangerous patterns.
// Accepts formats such as "hostPort:containerPort", "hostIP:hostPort:containerPort",
// and "hostPort:containerPort/proto". Only alphanumeric characters, dots, colons,
// slashes, and hyphens are permitted.
func validatePortMapping(mapping string) string {
	if !strings.Contains(mapping, ":") {
		return fmt.Sprintf("invalid port mapping %q: must contain a colon separator", mapping)
	}
	if strings.Contains(mapping, "..") {
		return fmt.Sprintf("invalid port mapping %q: must not contain path traversal sequences", mapping)
	}
	for _, c := range mapping {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			c == '.' || c == ':' || c == '/' || c == '-') {
			return fmt.Sprintf("invalid port mapping %q: contains invalid character %q", mapping, c)
		}
	}
	return ""
}

// validateVolumeMount checks a volume mount string for path traversal or missing separators.
// Volume mounts must be in "hostPath:containerPath" format.
func validateVolumeMount(mount string) string {
	if !strings.Contains(mount, ":") {
		return fmt.Sprintf("invalid volume mount %q: must be in hostPath:containerPath format", mount)
	}
	if strings.Contains(mount, "..") {
		return fmt.Sprintf("invalid volume mount %q: path must not contain path traversal sequences", mount)
	}
	return ""
}

func ensurePodmanImage(ctx context.Context, ref string) (string, error) {
	if exists, err := images.Exists(ctx, ref, nil); err == nil && exists {
		return ref, nil
	}

	normalised := normalizeImageRef(ref)
	if normalised != ref {
		if exists, err := images.Exists(ctx, normalised, nil); err == nil && exists {
			return normalised, nil
		}
	}

	pullRef := ref
	if normalised != ref {
		pullRef = normalised
	}

	if _, err := images.Pull(ctx, pullRef, new(images.PullOptions).WithPolicy("missing").WithQuiet(true)); err != nil {
		return "", fmt.Errorf("image %q not found locally and could not be pulled: %w", ref, err)
	}

	return pullRef, nil
}

func envListToMap(env []string) map[string]string {
	if len(env) == 0 {
		return nil
	}

	result := make(map[string]string, len(env))
	for _, item := range env {
		key, value, found := strings.Cut(item, "=")
		if !found {
			result[item] = ""
			continue
		}
		result[key] = value
	}

	return result
}

func parsePortMapping(mapping string) (nettypes.PortMapping, error) {
	protocol := "tcp"
	portSpec := mapping
	if before, after, found := strings.Cut(mapping, "/"); found {
		portSpec = before
		if after != "" {
			protocol = after
		}
	}

	parts := strings.Split(portSpec, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return nettypes.PortMapping{}, fmt.Errorf("invalid port mapping %q: expected hostPort:containerPort or hostIP:hostPort:containerPort", mapping)
	}

	hostIP := ""
	hostPortPart := parts[0]
	containerPortPart := parts[1]
	if len(parts) == 3 {
		hostIP = parts[0]
		hostPortPart = parts[1]
		containerPortPart = parts[2]
	}

	hostPort, err := strconv.ParseUint(hostPortPart, 10, 16)
	if err != nil || hostPort == 0 {
		return nettypes.PortMapping{}, fmt.Errorf("invalid port mapping %q: host port must be a valid port number", mapping)
	}

	containerPort, err := strconv.ParseUint(containerPortPart, 10, 16)
	if err != nil || containerPort == 0 {
		return nettypes.PortMapping{}, fmt.Errorf("invalid port mapping %q: container port must be a valid port number", mapping)
	}

	return nettypes.PortMapping{
		HostIP:        hostIP,
		HostPort:      uint16(hostPort),
		ContainerPort: uint16(containerPort),
		Range:         1,
		Protocol:      protocol,
	}, nil
}

// PullAndRun pulls the specified container image and starts a new container
// through Podman. It accepts POST requests with a JSON body matching
// PullAndRunRequest and returns the new container ID on success.
func PullAndRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req PullAndRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	fmt.Printf("PullAndRun raw request from frontend: %+v\n", req)
	fmt.Printf("PullAndRun frontend image=%q name=%q command=%q restartPolicy=%q autoRemove=%v\n",
		req.Image, req.Name, req.Command, req.RestartPolicy, req.AutoRemove)
	fmt.Printf("PullAndRun frontend ports=%v\n", req.Ports)
	fmt.Printf("PullAndRun frontend env=%v\n", req.Env)
	fmt.Printf("PullAndRun frontend volumes=%v\n", req.Volumes)

	req.Image = strings.TrimSpace(req.Image)
	req.Name = strings.TrimSpace(req.Name)

	if req.Image == "" {
		http.Error(w, "image is required", http.StatusBadRequest)
		return
	}

	// Validate the image name to prevent command injection or path traversal.
	if errMsg := validateImageName(req.Image); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	// Validate the optional container name.
	if req.Name != "" {
		if errMsg := validateContainerName(req.Name); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}

	// Validate port mappings.
	for _, port := range req.Ports {
		fmt.Printf("Validating port mapping from frontend: %q\n", port)
		if errMsg := validatePortMapping(port); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}

	// Validate volume mounts for path traversal.
	for _, vol := range req.Volumes {
		fmt.Printf("Validating volume mount from frontend: %q\n", vol)
		if errMsg := validateVolumeMount(vol); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}

	// Validate restart policy when explicitly provided.
	if req.RestartPolicy != "" && !validRestartPolicies[req.RestartPolicy] {
		http.Error(w, "invalid restart policy: must be one of no, always, on-failure, unless-stopped", http.StatusBadRequest)
		return
	}

	// autoRemove conflicts with any restart policy other than "no" because the
	// container runtime cannot both remove the container on exit and restart it.
	if req.AutoRemove && req.RestartPolicy != "" && req.RestartPolicy != "no" {
		http.Error(w, "autoRemove cannot be used with a restart policy other than 'no'", http.StatusBadRequest)
		return
	}

	socket, err := rootlessPodmanSocket()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to determine rootless Podman socket: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Printf("PullAndRun using Podman socket: %s\n", socket)

	ctx, cancel := context.WithTimeout(r.Context(), buildTimeout)
	defer cancel()

	conn, err := bindings.NewConnection(ctx, socket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman socket %q: %v", socket, err), http.StatusInternalServerError)
		return
	}
	fmt.Println("PullAndRun connected to Podman successfully")

	imageRef, err := ensurePodmanImage(conn, req.Image)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to resolve image %q: %v", req.Image, err), http.StatusInternalServerError)
		return
	}
	fmt.Printf("PullAndRun resolved imageRef: %q\n", imageRef)

	// Build the unique container ID from the requested name (or a timestamp fallback).
	containerID := req.Name
	if containerID == "" {
		containerID = fmt.Sprintf("opencloud-%d", time.Now().UnixNano())
	}
	fmt.Printf("PullAndRun containerID: %q\n", containerID)

	var mounts []specs.Mount
	for _, vol := range req.Volumes {
		parts := strings.SplitN(vol, ":", 2)
		if len(parts) != 2 {
			fmt.Printf("Skipping malformed volume mount: %q\n", vol)
			continue
		}
		mount := specs.Mount{
			Type:        "bind",
			Source:      parts[0],
			Destination: parts[1],
			Options:     []string{"rbind", "rw"},
		}
		fmt.Printf("Parsed volume mount: %+v\n", mount)
		mounts = append(mounts, mount)
	}

	var portMappings []nettypes.PortMapping
	for _, mapping := range req.Ports {
		portMapping, err := parsePortMapping(mapping)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Printf("Parsed port mapping from %q -> %+v\n", mapping, portMapping)
		portMappings = append(portMappings, portMapping)
	}

	envMap := envListToMap(req.Env)
	fmt.Printf("Parsed env map: %+v\n", envMap)

	labels := map[string]string{
		"opencloud/name": containerID,
	}
	if req.RestartPolicy != "" {
		labels["opencloud/restart-policy"] = req.RestartPolicy
	}
	if req.AutoRemove {
		labels["opencloud/auto-remove"] = "true"
	}
	if len(req.Ports) > 0 {
		labels["opencloud/ports"] = strings.Join(req.Ports, " ")
	}
	fmt.Printf("Container labels: %+v\n", labels)

	spec := specgen.NewSpecGenerator(imageRef, false)
	spec.Name = containerID
	spec.Labels = labels
	spec.NetNS = specgen.Namespace{NSMode: specgen.Bridge}
	spec.Env = envMap
	spec.Mounts = mounts
	spec.PortMappings = portMappings
	spec.RestartPolicy = req.RestartPolicy
	spec.Remove = &req.AutoRemove

	if req.Command != "" {
		spec.Entrypoint = []string{}
		spec.Command = strings.Fields(req.Command)
	}

	fmt.Printf("Final spec.Name: %q\n", spec.Name)
	fmt.Printf("Final spec.Image: %q\n", imageRef)
	fmt.Printf("Final spec.NetNS: %+v\n", spec.NetNS)
	fmt.Printf("Final spec.Env: %+v\n", spec.Env)
	fmt.Printf("Final spec.Mounts: %+v\n", spec.Mounts)
	fmt.Printf("Final spec.PortMappings: %+v\n", spec.PortMappings)
	fmt.Printf("Final spec.RestartPolicy: %q\n", spec.RestartPolicy)
	fmt.Printf("Final spec.Remove: %+v\n", spec.Remove)
	fmt.Printf("Final spec.Command: %+v\n", spec.Command)
	fmt.Printf("Final spec.Entrypoint: %+v\n", spec.Entrypoint)

	createResponse, err := containers.CreateWithSpec(conn, spec, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create container: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Printf("Container created successfully: ID=%s\n", createResponse.ID)

	if err := containers.Start(conn, createResponse.ID, nil); err != nil {
		_, _ = containers.Remove(conn, createResponse.ID, new(containers.RemoveOptions).WithForce(true).WithIgnore(true))
		http.Error(w, fmt.Sprintf("Failed to start container: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Printf("Container started successfully: ID=%s\n", createResponse.ID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":      "success",
		"message":     fmt.Sprintf("Container started from image %s", imageRef),
		"containerId": createResponse.ID,
		"socket":      socket,
	})
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
		"Invocations":  0,
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

	fileName := filepath.Base(filePath)                              // hello.py
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName)) // hello
	logFile := filepath.Join(fnDir, baseName+".log")                 // hello.log

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

	// Validate required fields
	if req.Name == "" {
		http.Error(w, "Function name is required", http.StatusBadRequest)
		return
	}
	if req.Runtime == "" {
		http.Error(w, "Runtime is required", http.StatusBadRequest)
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
		output := part[timestampEndIdx+len(headerEndMarker) : endIdx]

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
