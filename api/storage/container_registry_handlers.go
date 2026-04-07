package storage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	opencloudapi "github.com/WavexSoftware/OpenCloud/api"
	service_ledger "github.com/WavexSoftware/OpenCloud/service_ledger"
	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	podmanEntities "github.com/containers/podman/v5/pkg/domain/entities/types"
)

// newDeleteImageConnection establishes a Podman bindings connection for the
// DeleteImage handler.  It is a package-level variable so tests can substitute
// a no-op implementation that returns the context unchanged.
var newDeleteImageConnection = func(ctx context.Context, socket string) (context.Context, error) {
	return bindings.NewConnection(ctx, socket)
}

// newGetImageConnection establishes a Podman bindings connection for the
// GetImage handler.  It is a package-level variable so tests can substitute
// a no-op implementation that returns the context unchanged.
var newGetImageConnection = func(ctx context.Context, socket string) (context.Context, error) {
	return bindings.NewConnection(ctx, socket)
}

// listContainersForImage is a package-level variable that can be overridden in
// tests to avoid a real Podman connection when checking for containers that use
// an image before deletion.
var listContainersForImage = func(ctx context.Context, opts *containers.ListOptions) ([]podmanEntities.ListContainer, error) {
	return containers.List(ctx, opts)
}

// inspectPodmanImage is a package-level variable that can be overridden in
// tests to avoid a real Podman connection when inspecting an image.
var inspectPodmanImage = func(ctx context.Context, nameOrID string, opts *images.GetOptions) (*podmanEntities.ImageInspectReport, error) {
	return images.GetImage(ctx, nameOrID, opts)
}

// ImageDetail holds the detailed metadata for a single container image as
// returned by GET /get-image.
type ImageDetail struct {
	// ID is the full image content-addressable ID.
	ID string `json:"id"`
	// RepoTags is the list of repository tags associated with this image.
	RepoTags []string `json:"repoTags"`
	// RepoDigests is the list of content-addressable digests for this image.
	RepoDigests []string `json:"repoDigests"`
	// Created is the Unix timestamp when the image was built.
	Created int64 `json:"created"`
	// Size is the on-disk size of the image in bytes.
	Size int64 `json:"size"`
	// VirtualSize is the total virtual size (including shared layers) in bytes.
	VirtualSize int64 `json:"virtualSize"`
	// Labels contains metadata labels attached to the image.
	Labels map[string]string `json:"labels"`
	// Architecture is the CPU architecture the image was built for.
	Architecture string `json:"architecture"`
	// Os is the operating system the image was built for.
	Os string `json:"os"`
	// Author is the image author field, if present.
	Author string `json:"author"`
	// Comment is an optional comment stored in the image manifest.
	Comment string `json:"comment"`
	// NamesHistory contains the historical list of names this image has had.
	NamesHistory []string `json:"namesHistory"`
}

// BuildImageRequest represents the JSON payload for building a container image
type BuildImageRequest struct {
	Dockerfile string            `json:"dockerfile"`
	ImageName  string            `json:"imageName"`
	Context    string            `json:"context"` // legacy optional text context
	Files      map[string]string `json:"files"`   // optional build context files: path -> contents
	NoCache    bool              `json:"nocache"`
	Platform   string            `json:"platform"` // optional
}

// DeleteImageRequest represents the JSON payload for deleting a container image
type DeleteImageRequest struct {
	ImageName string `json:"imageName"`
}

// PullImageRequest represents the JSON payload for pulling a container image from a registry.
type PullImageRequest struct {
	// ImageName is the image reference to pull, e.g. "nginx:latest" or "quay.io/prometheus/prometheus:latest".
	ImageName string `json:"imageName"`
	// Registry is the source registry: "docker.io" (default) or "quay.io".
	Registry string `json:"registry"`
}

// hasFromInstruction checks whether a Dockerfile string contains a FROM instruction,
// ignoring comment lines (lines starting with #).
func hasFromInstruction(dockerfile string) bool {
	for _, line := range strings.Split(dockerfile, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM") {
			return true
		}
		// First non-comment, non-empty line does not start with FROM
		return false
	}
	return false
}

// parsePlatform parses a platform string in os/arch or os/arch/variant format.
func parsePlatform(platform string) (string, string, error) {
	parts := strings.Split(platform, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return "", "", fmt.Errorf("platform must be in os/arch or os/arch/variant format")
	}

	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("platform must include both operating system and architecture")
	}

	return parts[0], parts[1], nil
}

// sanitizeRelativePath validates and cleans a relative file path, rejecting
// absolute paths, path traversal sequences, and empty paths.
func sanitizeRelativePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("path is empty")
	}

	clean := filepath.Clean(p)

	if clean == "." {
		return "", fmt.Errorf("path resolves to current directory")
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal is not allowed")
	}

	return clean, nil
}

// truncateString returns s truncated to max bytes, appending a truncation marker if needed.
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...truncated..."
}

// marshalFilesForLedger serialises the files map to JSON for ledger storage, or
// falls back to legacyContext when the map is empty.
func marshalFilesForLedger(files map[string]string, legacyContext string) string {
	if len(files) > 0 {
		b, err := json.Marshal(files)
		if err == nil {
			return string(b)
		}
	}
	return legacyContext
}

// imageTagEntry holds the subset of fields from a Podman ImageSummary that are
// needed to expand a single physical image into one ImageInfo per tag.
type imageTagEntry struct {
	ID          string
	RepoTags    []string
	Names       []string
	RepoDigests []string
	Created     int64
	Size        int64
	VirtualSize int64
	Labels      map[string]string
}

// expandImageTags converts a single imageTagEntry into one ImageInfo per unique
// repository:tag combination, mirroring the per-row display of `podman images`.
// The "localhost/" prefix is stripped from all tags and fallback Names entries so
// that locally-built images are presented without the implicit registry prefix.
// An image with no tags is returned as a single entry identified by its ID.
func expandImageTags(entry imageTagEntry) []opencloudapi.ImageInfo {
	tags := append([]string(nil), entry.RepoTags...)
	if len(tags) == 0 {
		tags = append(tags, entry.Names...)
	}
	for i, t := range tags {
		tags[i] = strings.TrimPrefix(t, "localhost/")
	}

	status := fmt.Sprintf("Created %s", time.Unix(entry.Created, 0).Format(time.RFC3339))
	repoDigests := append([]string(nil), entry.RepoDigests...)
	names := append([]string(nil), entry.Names...)

	if len(tags) == 0 {
		return []opencloudapi.ImageInfo{{
			ID:          entry.ID,
			RepoTags:    nil,
			RepoDigests: repoDigests,
			Created:     entry.Created,
			Size:        entry.Size,
			VirtualSize: entry.VirtualSize,
			Labels:      entry.Labels,
			Names:       names,
			Image:       entry.ID,
			State:       "available",
			Status:      status,
		}}
	}

	result := make([]opencloudapi.ImageInfo, 0, len(tags))
	for _, tag := range tags {
		result = append(result, opencloudapi.ImageInfo{
			ID:          entry.ID,
			RepoTags:    []string{tag},
			RepoDigests: repoDigests,
			Created:     entry.Created,
			Size:        entry.Size,
			VirtualSize: entry.VirtualSize,
			Labels:      entry.Labels,
			Names:       names,
			Image:       tag,
			State:       "available",
			Status:      status,
		})
	}
	return result
}

// GetContainerRegistry lists all container images available through Podman.
func GetContainerRegistry(w http.ResponseWriter, r *http.Request) {
	socket, err := opencloudapi.RootlessPodmanSocket()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to determine rootless Podman socket: %v", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	conn, err := bindings.NewConnection(ctx, socket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman socket %q: %v", socket, err), http.StatusInternalServerError)
		return
	}

	imageList, err := images.List(conn, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list images: %v", err), http.StatusInternalServerError)
		return
	}

	result := make([]opencloudapi.ImageInfo, 0, len(imageList))
	for _, img := range imageList {
		result = append(result, expandImageTags(imageTagEntry{
			ID:          img.ID,
			RepoTags:    img.RepoTags,
			Names:       img.Names,
			RepoDigests: img.RepoDigests,
			Created:     img.Created,
			Size:        img.Size,
			VirtualSize: img.VirtualSize,
			Labels:      img.Labels,
		})...)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// BuildImage handles building a container image using the Podman API.
func BuildImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req BuildImageRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 10<<20))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	req.Dockerfile = strings.TrimSpace(req.Dockerfile)
	req.ImageName = strings.TrimSpace(req.ImageName)

	if req.Dockerfile == "" || req.ImageName == "" {
		http.Error(w, "dockerfile and imageName are required", http.StatusBadRequest)
		return
	}

	if !hasFromInstruction(req.Dockerfile) {
		http.Error(w, "dockerfile must contain a FROM instruction", http.StatusBadRequest)
		return
	}

	if errMsg := opencloudapi.ValidateImageName(req.ImageName); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	tmpDir, err := os.MkdirTemp("", "opencloud-build-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp dir: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	dfPath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dfPath, []byte(req.Dockerfile), 0644); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write Dockerfile: %v", err), http.StatusInternalServerError)
		return
	}

	// Backward compatibility: if legacy context is provided and no files map exists,
	// write it as context.txt so older callers still behave the same way.
	if req.Context != "" && len(req.Files) == 0 {
		ctxPath := filepath.Join(tmpDir, "context.txt")
		if err := os.WriteFile(ctxPath, []byte(req.Context), 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write context: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Preferred: write real build context files.
	for relPath, content := range req.Files {
		cleanRel, err := sanitizeRelativePath(relPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid file path %q: %v", relPath, err), http.StatusBadRequest)
			return
		}

		fullPath := filepath.Join(tmpDir, cleanRel)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create directory for %q: %v", relPath, err), http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write %q: %v", relPath, err), http.StatusInternalServerError)
			return
		}
	}

	socket, err := opencloudapi.RootlessPodmanSocket()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to determine rootless Podman socket: %v", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), opencloudapi.BuildTimeout)
	defer cancel()

	conn, err := bindings.NewConnection(ctx, socket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman socket %q: %v", socket, err), http.StatusServiceUnavailable)
		return
	}

	var buildLogs bytes.Buffer

	buildOpts := podmanEntities.BuildOptions{
		BuildOptions: buildahDefine.BuildOptions{
			ContextDirectory: tmpDir,
			Output:           req.ImageName,
			NoCache:          req.NoCache,
			CommonBuildOpts:  &buildahDefine.CommonBuildOptions{},
			ReportWriter:     &buildLogs,
		},
	}

	if req.Platform != "" {
		osName, arch, err := parsePlatform(req.Platform)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		buildOpts.OS = osName
		buildOpts.Architecture = arch
	}

	if _, err := images.Build(conn, []string{"Dockerfile"}, buildOpts); err != nil {
		log.Printf("BuildImage2 failed for %s: %v", req.ImageName, err)
		http.Error(
			w,
			fmt.Sprintf("Build failed: %v\n\n%s", err, truncateString(buildLogs.String(), 32000)),
			http.StatusInternalServerError,
		)
		return
	}

	if ledgerErr := service_ledger.UpdateContainerImageEntry(
		req.ImageName,
		req.Dockerfile,
		marshalFilesForLedger(req.Files, req.Context),
		req.Platform,
		req.NoCache,
		time.Now().UTC().Format(time.RFC3339),
	); ledgerErr != nil {
		log.Printf("Warning: failed to record image %s in service ledger: %v", req.ImageName, ledgerErr)
	}

	resp := map[string]string{
		"status":    "success",
		"message":   fmt.Sprintf("Image %s built successfully", req.ImageName),
		"imageName": req.ImageName,
		"socket":    socket,
		"logs":      truncateString(buildLogs.String(), 32000),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// normalizeImageRef strips the "localhost/" prefix from an image reference so
// that locally-built images can be compared uniformly regardless of how they
// were stored or referenced (e.g. "localhost/myapp:latest" vs "myapp:latest").
func normalizeImageRef(ref string) string {
	return strings.TrimPrefix(ref, "localhost/")
}

// rejectIfImageInUse returns an error if any container in the Podman runtime
// references the given image. The comparison is done after normalizing both
// sides with normalizeImageRef to handle the "localhost/" prefix difference.
func rejectIfImageInUse(conn context.Context, imageName string) error {
	ctrs, err := listContainersForImage(conn, new(containers.ListOptions).WithAll(true))
	if err != nil {
		// If we cannot list containers, be conservative and block the deletion.
		return fmt.Errorf("failed to list containers before deleting image: %v", err)
	}

	normalizedTarget := normalizeImageRef(imageName)
	for _, ctr := range ctrs {
		if normalizeImageRef(ctr.Image) == normalizedTarget {
			names := ctr.Names
			if len(names) == 0 {
				names = []string{ctr.ID}
			}
			return fmt.Errorf(
				"image %q is in use by container %q; remove the container before deleting the image",
				imageName, names[0],
			)
		}
	}
	return nil
}

// DeleteImage handles deletion of a container image from the Podman image store.
// It accepts a POST request with a JSON body containing the image name to delete.
func DeleteImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req DeleteImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	req.ImageName = strings.TrimSpace(req.ImageName)
	if req.ImageName == "" {
		http.Error(w, "imageName is required", http.StatusBadRequest)
		return
	}

	socket, err := opencloudapi.RootlessPodmanSocket()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to determine rootless Podman socket: %v", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	conn, err := newDeleteImageConnection(ctx, socket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman socket %q: %v", socket, err), http.StatusInternalServerError)
		return
	}

	// Guard: reject deletion if any container in Container Compute is using this image.
	if err := rejectIfImageInUse(conn, req.ImageName); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	if _, errs := images.Remove(conn, []string{req.ImageName}, new(images.RemoveOptions)); len(errs) > 0 {
		http.Error(w, fmt.Sprintf("Failed to delete image: %v", errs[0]), http.StatusInternalServerError)
		return
	}

	ledgerName := strings.TrimPrefix(req.ImageName, "localhost/")
	if ledgerErr := service_ledger.DeleteContainerImageEntry(ledgerName); ledgerErr != nil {
		log.Printf("Warning: failed to remove image %s from service ledger: %v", ledgerName, ledgerErr)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":    "deleted",
		"imageName": req.ImageName,
		"socket":    socket,
	})
}

// PullImage pulls a container image from a public registry (docker.io or quay.io)
// using Podman and records the pulled image in the service ledger.
func PullImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req PullImageRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	req.ImageName = strings.TrimSpace(req.ImageName)
	if req.ImageName == "" {
		http.Error(w, "imageName is required", http.StatusBadRequest)
		return
	}

	req.Registry = strings.TrimSpace(req.Registry)
	if req.Registry == "" {
		req.Registry = "docker.io"
	}
	if req.Registry != "docker.io" && req.Registry != "quay.io" {
		http.Error(w, "registry must be \"docker.io\" or \"quay.io\"", http.StatusBadRequest)
		return
	}

	if errMsg := opencloudapi.ValidateImageName(req.ImageName); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	// Build the fully-qualified image reference.
	imageRef := req.ImageName
	// Only prepend the registry when the name does not already contain one.
	if !strings.ContainsRune(imageRef, '/') || !strings.Contains(strings.SplitN(imageRef, "/", 2)[0], ".") {
		imageRef = req.Registry + "/" + imageRef
	}

	socket, err := opencloudapi.RootlessPodmanSocket()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to determine rootless Podman socket: %v", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	conn, err := bindings.NewConnection(ctx, socket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman socket %q: %v", socket, err), http.StatusInternalServerError)
		return
	}

	if _, err := images.Pull(conn, imageRef, new(images.PullOptions).WithQuiet(false)); err != nil {
		http.Error(w, fmt.Sprintf("Failed to pull image %q: %v", imageRef, err), http.StatusInternalServerError)
		return
	}

	if ledgerErr := service_ledger.RecordPulledImageEntry(
		req.ImageName,
		req.Registry,
		time.Now().UTC().Format(time.RFC3339),
	); ledgerErr != nil {
		log.Printf("Warning: failed to record pulled image %s in service ledger: %v", req.ImageName, ledgerErr)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":    "success",
		"message":   fmt.Sprintf("Image %q pulled successfully", imageRef),
		"imageName": imageRef,
	})
}

// pullProgressEvent is the JSON structure emitted by Podman's progress writer
// during an image pull.  Fields mirror Docker's JSON progress protocol.
type pullProgressEvent struct {
	Status   string `json:"status"`
	Progress string `json:"progress"`
	Stream   string `json:"stream"`
	Error    string `json:"error"`
	ID       string `json:"id"`
}

// PullImageStream pulls a container image from a public registry and streams
// real-time progress updates to the client using Server-Sent Events (SSE).
//
// Request:  POST /pull-image-stream  body: {"imageName":"nginx:latest","registry":"docker.io"}
// Response: text/event-stream
//
//	data: <progress line>
//	...
//	event: done
//	data: {"status":"success","imageName":"docker.io/nginx:latest"}
//
//	On error:
//	event: error
//	data: <error message>
func PullImageStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req PullImageRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	req.ImageName = strings.TrimSpace(req.ImageName)
	if req.ImageName == "" {
		http.Error(w, "imageName is required", http.StatusBadRequest)
		return
	}

	req.Registry = strings.TrimSpace(req.Registry)
	if req.Registry == "" {
		req.Registry = "docker.io"
	}
	if req.Registry != "docker.io" && req.Registry != "quay.io" {
		http.Error(w, "registry must be \"docker.io\" or \"quay.io\"", http.StatusBadRequest)
		return
	}

	if errMsg := opencloudapi.ValidateImageName(req.ImageName); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	// Build the fully-qualified image reference.
	imageRef := req.ImageName
	if !strings.ContainsRune(imageRef, '/') || !strings.Contains(strings.SplitN(imageRef, "/", 2)[0], ".") {
		imageRef = req.Registry + "/" + imageRef
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

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	conn, err := bindings.NewConnection(ctx, socket)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", fmt.Sprintf("Failed to connect to Podman: %v", err))
		flusher.Flush()
		return
	}

	// Use a pipe so progress lines can be streamed to the client while Pull runs.
	pr, pw := io.Pipe()

	pullErr := make(chan error, 1)
	go func() {
		opts := new(images.PullOptions).WithQuiet(false).WithProgressWriter(pw)
		_, err := images.Pull(conn, imageRef, opts)
		pw.Close()
		pullErr <- err
	}()

	// Stream each progress line to the SSE client, parsing Podman's JSON
	// progress-event format when possible to produce a readable message.
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		raw := scanner.Text()
		if raw == "" {
			continue
		}

		var event pullProgressEvent
		if json.Unmarshal([]byte(raw), &event) == nil {
			// Propagate explicit pull errors embedded in the progress stream.
			if event.Error != "" {
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", event.Error)
				flusher.Flush()
				return
			}
			// Prefer the human-readable "stream" field (e.g. "Pulling from …"),
			// then the "status" field, optionally decorated with progress bar text.
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
			// Not JSON – send the raw line as-is.
			sendLine(raw)
		}
	}

	if err := <-pullErr; err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", fmt.Sprintf("Failed to pull image %q: %v", imageRef, err))
		flusher.Flush()
		return
	}

	// Record the pulled image in the service ledger.
	if ledgerErr := service_ledger.RecordPulledImageEntry(
		req.ImageName,
		req.Registry,
		time.Now().UTC().Format(time.RFC3339),
	); ledgerErr != nil {
		log.Printf("Warning: failed to record pulled image %s in service ledger: %v", req.ImageName, ledgerErr)
	}

	donePayload, _ := json.Marshal(map[string]string{"status": "success", "imageName": imageRef})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", donePayload)
	flusher.Flush()
}

// GetImage inspects a single container image by name or ID and returns its
// detailed metadata. It accepts GET requests with a required "name" query
// parameter containing the image name or ID.
//
// Route: GET /get-image?name=<imageNameOrID>
func GetImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nameOrID := strings.TrimSpace(r.URL.Query().Get("name"))
	if nameOrID == "" {
		http.Error(w, "name query parameter is required", http.StatusBadRequest)
		return
	}

	socket, err := opencloudapi.RootlessPodmanSocket()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to determine rootless Podman socket: %v", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	conn, err := newGetImageConnection(ctx, socket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Podman socket %q: %v", socket, err), http.StatusInternalServerError)
		return
	}

	data, err := inspectPodmanImage(conn, nameOrID, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to inspect image: %v", err), http.StatusInternalServerError)
		return
	}

	detail := ImageDetail{
		ID:           data.ID,
		RepoTags:     data.RepoTags,
		RepoDigests:  data.RepoDigests,
		Size:         data.Size,
		VirtualSize:  data.VirtualSize,
		Labels:       data.Labels,
		Architecture: data.Architecture,
		Os:           data.Os,
		Author:       data.Author,
		Comment:      data.Comment,
		NamesHistory: data.NamesHistory,
	}

	// Strip the "localhost/" prefix from RepoTags so locally-built images are
	// presented consistently with the list view.
	for i, t := range detail.RepoTags {
		detail.RepoTags[i] = strings.TrimPrefix(t, "localhost/")
	}

	if data.Created != nil {
		detail.Created = data.Created.Unix()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(detail); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
