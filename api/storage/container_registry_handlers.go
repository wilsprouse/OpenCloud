package storage

import (
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
"github.com/containers/podman/v5/pkg/bindings/images"
podmanEntities "github.com/containers/podman/v5/pkg/domain/entities/types"
)

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
tags := append([]string(nil), img.RepoTags...)
if len(tags) == 0 {
tags = append(tags, img.Names...)
}
for i, t := range tags {
tags[i] = strings.TrimPrefix(t, "localhost/")
}

displayName := img.ID
if len(tags) > 0 {
displayName = tags[0]
}

imageInfo := opencloudapi.ImageInfo{
ID:          img.ID,
RepoTags:    tags,
RepoDigests: append([]string(nil), img.RepoDigests...),
Created:     img.Created,
Size:        img.Size,
VirtualSize: img.VirtualSize,
Labels:      img.Labels,
Names:       append([]string(nil), img.Names...),
Image:       displayName,
State:       "available",
Status:      fmt.Sprintf("Created %s", time.Unix(img.Created, 0).Format(time.RFC3339)),
}

result = append(result, imageInfo)
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

conn, err := bindings.NewConnection(ctx, socket)
if err != nil {
http.Error(w, fmt.Sprintf("Failed to connect to Podman socket %q: %v", socket, err), http.StatusInternalServerError)
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
