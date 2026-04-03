package compute

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	opencloudapi "github.com/WavexSoftware/OpenCloud/api"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	nettypes "go.podman.io/common/libnetwork/types"
)

var (
	getContainersConnection   = opencloudapi.RootlessPodmanConnection
	listPodmanContainers      = containers.List
	deleteContainerConnection = opencloudapi.RootlessPodmanConnection
	removePodmanContainer     = containers.Remove
	containerActionConnection = opencloudapi.RootlessPodmanConnection
	startPodmanContainer      = containers.Start
	stopPodmanContainer       = containers.Stop
)

type ContainerInfo = opencloudapi.ContainerInfo

// GetContainers lists all containers from Podman and returns their state along
// with common runtime metrics such as memory usage and the host PID of the main
// container process.
func GetContainers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	conn, err := getContainersConnection(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman: %v", err), http.StatusInternalServerError)
		return
	}

	containerList, err := listPodmanContainers(conn, new(containers.ListOptions).WithAll(true).WithSync(true))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list containers: %v", err), http.StatusInternalServerError)
		return
	}

	// Initialize to an empty slice so JSON encodes as [] rather than null
	// when no containers are present.
	result := make([]opencloudapi.ContainerInfo, 0, len(containerList))
	for _, ctr := range containerList {
		names := append([]string(nil), ctr.Names...)
		if len(names) == 0 {
			names = []string{ctr.ID}
		}

		pid := uint32(0)
		if ctr.Pid > 0 {
			pid = uint32(ctr.Pid)
		}

		ci := opencloudapi.ContainerInfo{
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

// DeleteContainerRequest represents the JSON payload for deleting a container.
type DeleteContainerRequest struct {
	ContainerID string `json:"containerId"`
}

// DeleteContainer force-removes a Podman container by ID. It accepts POST
// requests with a JSON body containing the containerId to delete.
func DeleteContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req DeleteContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	req.ContainerID = strings.TrimSpace(req.ContainerID)
	if req.ContainerID == "" {
		http.Error(w, "containerId is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	conn, err := deleteContainerConnection(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := removePodmanContainer(conn, req.ContainerID, new(containers.RemoveOptions).WithForce(true)); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete container: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":      "deleted",
		"containerId": req.ContainerID,
	})
}

// ContainerAction starts or stops a Podman container addressed as
// /containers/{containerId}/{action}.
func ContainerAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/containers/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "container ID and action are required", http.StatusBadRequest)
		return
	}

	containerID := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	if containerID == "" || action == "" {
		http.Error(w, "container ID and action are required", http.StatusBadRequest)
		return
	}

	status := ""
	var performAction func(context.Context, string) error
	switch action {
	case "start":
		status = "started"
		performAction = func(ctx context.Context, id string) error {
			return startPodmanContainer(ctx, id, nil)
		}
	case "stop":
		status = "stopped"
		performAction = func(ctx context.Context, id string) error {
			return stopPodmanContainer(ctx, id, nil)
		}
	default:
		http.Error(w, "Unsupported container action", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	conn, err := containerActionConnection(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman: %v", err), http.StatusInternalServerError)
		return
	}

	if err := performAction(conn, containerID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to %s container: %v", action, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":      status,
		"containerId": containerID,
	})
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

	normalised := opencloudapi.NormalizeImageRef(ref)
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
	if errMsg := opencloudapi.ValidateImageName(req.Image); errMsg != "" {
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

	socket, err := opencloudapi.RootlessPodmanSocket()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to determine rootless Podman socket: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Printf("PullAndRun using Podman socket: %s\n", socket)

	ctx, cancel := context.WithTimeout(r.Context(), opencloudapi.BuildTimeout)
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

// pullProgressEvent mirrors Docker's JSON progress protocol as emitted by
// Podman's progress writer during an image pull.
type pullProgressEvent struct {
	Status   string `json:"status"`
	Progress string `json:"progress"`
	Stream   string `json:"stream"`
	Error    string `json:"error"`
	ID       string `json:"id"`
}

// PullAndRunStream pulls the specified container image and starts a new
// container through Podman, streaming real-time pull progress to the client
// using Server-Sent Events (SSE).
//
// Request:  POST /pull-and-run-stream  body: PullAndRunRequest JSON
// Response: text/event-stream
//
//	data: <pull progress line>
//	...
//	data: Starting container…
//	event: done
//	data: {"status":"success","containerId":"<id>"}
//
//	On error:
//	event: error
//	data: <error message>
func PullAndRunStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req PullAndRunRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	req.Image = strings.TrimSpace(req.Image)
	req.Name = strings.TrimSpace(req.Name)

	if req.Image == "" {
		http.Error(w, "image is required", http.StatusBadRequest)
		return
	}

	if errMsg := opencloudapi.ValidateImageName(req.Image); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	if req.Name != "" {
		if errMsg := validateContainerName(req.Name); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}

	for _, port := range req.Ports {
		if errMsg := validatePortMapping(port); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}

	for _, vol := range req.Volumes {
		if errMsg := validateVolumeMount(vol); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}

	if req.RestartPolicy != "" && !validRestartPolicies[req.RestartPolicy] {
		http.Error(w, "invalid restart policy: must be one of no, always, on-failure, unless-stopped", http.StatusBadRequest)
		return
	}

	if req.AutoRemove && req.RestartPolicy != "" && req.RestartPolicy != "no" {
		http.Error(w, "autoRemove cannot be used with a restart policy other than 'no'", http.StatusBadRequest)
		return
	}

	socket, err := opencloudapi.RootlessPodmanSocket()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to determine rootless Podman socket: %v", err), http.StatusInternalServerError)
		return
	}

	// Upgrade the connection to SSE before any long-running operations.
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
	sendError := func(msg string) {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", msg)
		flusher.Flush()
	}

	ctx, cancel := context.WithTimeout(r.Context(), opencloudapi.BuildTimeout)
	defer cancel()

	conn, err := bindings.NewConnection(ctx, socket)
	if err != nil {
		sendError(fmt.Sprintf("Failed to connect to Podman: %v", err))
		return
	}

	// Resolve the image reference, preferring a locally cached copy.
	imageRef := opencloudapi.NormalizeImageRef(req.Image)

	// Check whether the image already exists locally; if not, pull it with
	// streaming progress so the client can observe each layer download.
	localExists := false
	if exists, err := images.Exists(conn, req.Image, nil); err == nil && exists {
		localExists = true
		imageRef = req.Image
	} else if exists, err := images.Exists(conn, imageRef, nil); err == nil && exists {
		localExists = true
	}

	if !localExists {
		sendLine(fmt.Sprintf("Pulling image %s…", imageRef))

		pr, pw := io.Pipe()
		pullErr := make(chan error, 1)
		go func() {
			opts := new(images.PullOptions).WithQuiet(false).WithProgressWriter(pw)
			_, err := images.Pull(conn, imageRef, opts)
			pw.Close()
			pullErr <- err
		}()

		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			raw := scanner.Text()
			if raw == "" {
				continue
			}
			var event pullProgressEvent
			if json.Unmarshal([]byte(raw), &event) == nil {
				if event.Error != "" {
					sendError(event.Error)
					return
				}
				msg := strings.TrimRight(event.Stream, "\n")
				if msg == "" {
					msg = event.Status
					if event.ID != "" {
						msg = event.ID + ": " + msg
					}
					if event.Progress != "" {
						msg += " " + event.Progress
					}
				}
				if msg != "" {
					sendLine(msg)
				}
			} else {
				sendLine(raw)
			}
		}

		if err := <-pullErr; err != nil {
			sendError(fmt.Sprintf("Failed to pull image %q: %v", imageRef, err))
			return
		}
	}

	// Build the unique container ID from the requested name (or a timestamp fallback).
	containerID := req.Name
	if containerID == "" {
		containerID = fmt.Sprintf("opencloud-%d", time.Now().UnixNano())
	}

	var mounts []specs.Mount
	for _, vol := range req.Volumes {
		parts := strings.SplitN(vol, ":", 2)
		if len(parts) != 2 {
			continue
		}
		mounts = append(mounts, specs.Mount{
			Type:        "bind",
			Source:      parts[0],
			Destination: parts[1],
			Options:     []string{"rbind", "rw"},
		})
	}

	var portMappings []nettypes.PortMapping
	for _, mapping := range req.Ports {
		pm, err := parsePortMapping(mapping)
		if err != nil {
			sendError(err.Error())
			return
		}
		portMappings = append(portMappings, pm)
	}

	envMap := envListToMap(req.Env)

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

	sendLine("Creating container…")

	createResponse, err := containers.CreateWithSpec(conn, spec, nil)
	if err != nil {
		sendError(fmt.Sprintf("Failed to create container: %v", err))
		return
	}

	sendLine("Starting container…")

	if err := containers.Start(conn, createResponse.ID, nil); err != nil {
		_, _ = containers.Remove(conn, createResponse.ID, new(containers.RemoveOptions).WithForce(true).WithIgnore(true))
		sendError(fmt.Sprintf("Failed to start container: %v", err))
		return
	}

	donePayload, _ := json.Marshal(map[string]string{
		"status":      "success",
		"message":     fmt.Sprintf("Container started from image %s", imageRef),
		"containerId": createResponse.ID,
	})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", donePayload)
	flusher.Flush()
}
