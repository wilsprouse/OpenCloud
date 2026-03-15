package api

import (
        "context"
        "encoding/json"
        "fmt"
        "io"
        "log"
        "mime"
        "net/http"
        "os"
        "os/exec"
        "path/filepath"
        "regexp"
        "strings"
        "time"

        "github.com/containerd/containerd"
        "github.com/containerd/containerd/namespaces"
        service_ledger "github.com/WavexSoftware/OpenCloud/service_ledger"
)

const buildTimeout = 5 * time.Minute

// containerdSocket is the path to the containerd Unix domain socket.
const containerdSocket = "/run/containerd/containerd.sock"

// Pre-compiled regex patterns for image name validation
var (
        // Pattern for lowercase image names (after normalization)
        imageNamePatternLower = regexp.MustCompile(`^[a-z0-9]+(([._-]|__)[a-z0-9]+)*(:[a-z0-9]+(([._-]|__)[a-z0-9]+)*)*(/[a-z0-9]+(([._-]|__)[a-z0-9]+)*(:[a-z0-9]+(([._-]|__)[a-z0-9]+)*)*)*(@sha256:[a-f0-9]{64})?$`)
        // Pattern for mixed-case image names
        imageNamePatternMixed = regexp.MustCompile(`^[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*(:[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*)*(/[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*(:[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*)*)*$`)

        // Buffer size for BuildKit progress status channel
        buildProgressBufferSize = 100
)

type Blob struct {
        ID           string `json:"id"`
        Name         string `json:"name"`
        Size         int64  `json:"size"`
        ContentType  string `json:"contentType"`
        LastModified string `json:"lastModified"`
        Container    string `json:"container"`
}

type Container struct {
        Name         string `json:"name"`
        ObjectCount  int    `json:"objectCount"`
        TotalSize    int64  `json:"totalSize"`
        LastModified string `json:"lastModified"`
}



// GetContainerRegistry lists all container images built by buildkitd using containerd.
// Images built via BuildImage are stored in the "buildkit" containerd namespace,
// which corresponds to the output of `ctr -n buildkit images ls`.
func GetContainerRegistry(w http.ResponseWriter, r *http.Request) {
        ctx := context.Background()

        // Use the "buildkit" namespace where buildkitd stores built images
        ctx = namespaces.WithNamespace(ctx, "buildkit")

	// Connect to containerd socket using the shared constant
	cli, err := containerd.New(containerdSocket)
	if err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprintf("Failed to connect to containerd: %v", err), http.StatusInternalServerError)
		return
	}
        defer cli.Close()

        // List all images in the containerd image store
        imageList, err := cli.ImageService().List(ctx)
        if err != nil {
                http.Error(w, fmt.Sprintf("Failed to list images: %v", err), http.StatusInternalServerError)
                return
        }

	fmt.Println(imageList)

        // Convert containerd images to the format expected by the frontend
        var result []ImageInfo
        for _, img := range imageList {
                // Get image size
                size := img.Target.Size

                // Parse tags from image name
                tags := []string{img.Name}

                imageInfo := ImageInfo{
                        ID:          img.Target.Digest.String(),
                        RepoTags:    tags,
                        RepoDigests: []string{img.Target.Digest.String()},
                        Created:     img.CreatedAt.Unix(),
                        Size:        size,
                        VirtualSize: size,
                        Labels:      img.Labels,
                        Names:       tags,
                        Image:       img.Name,
                        State:       "available",
                        Status:      fmt.Sprintf("Created %s", img.CreatedAt.Format(time.RFC3339)),
                }

                result = append(result, imageInfo)
        }

        // Encode the images as JSON and write to response
        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(result); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }
}

// ListBlobContainers returns a list of blob storage containers with metadata.
func ListBlobContainers(w http.ResponseWriter, r *http.Request) {
        home, err := os.UserHomeDir()
        if err != nil {
                http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
                return
        }

        root := filepath.Join(home, ".opencloud", "blob_storage")
        entries, err := os.ReadDir(root)
        if err != nil {
                http.Error(w, "Failed to read blob storage directory", http.StatusInternalServerError)
                return
        }

        var containers []Container
        for _, entry := range entries {
                if !entry.IsDir() {
                        continue
                }

                containerPath := filepath.Join(root, entry.Name())
                containerInfo, err := os.Stat(containerPath)
                if err != nil {
                        continue
                }

                // Count objects and calculate total size
                files, _ := os.ReadDir(containerPath)
                objectCount := 0
                var totalSize int64
                var lastModified time.Time = containerInfo.ModTime()

                for _, file := range files {
                        if file.IsDir() {
                                continue
                        }
                        objectCount++
                        info, _ := os.Stat(filepath.Join(containerPath, file.Name()))
                        totalSize += info.Size()
                        if info.ModTime().After(lastModified) {
                                lastModified = info.ModTime()
                        }
                }

                containers = append(containers, Container{
                        Name:         entry.Name(),
                        ObjectCount:  objectCount,
                        TotalSize:    totalSize,
                        LastModified: lastModified.UTC().Format(time.RFC3339),
                })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(containers)
}

// GetBlobBuckets returns blobs from all containers or a specific container if specified.
func GetBlobBuckets(w http.ResponseWriter, r *http.Request) {
        home, err := os.UserHomeDir()
        if err != nil {
                http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
                return
        }

        // Check if a specific container is requested via query parameter
        containerFilter := r.URL.Query().Get("container")

        root := filepath.Join(home, ".opencloud", "blob_storage")
        entries, err := os.ReadDir(root)
        if err != nil {
                http.Error(w, "Failed to read blob storage directory", http.StatusInternalServerError)
                return
        }

        var blobs []Blob
        for _, container := range entries {
                if !container.IsDir() {
                        continue
                }

                // Skip if a specific container is requested and this isn't it
                if containerFilter != "" && container.Name() != containerFilter {
                        continue
                }

                containerPath := filepath.Join(root, container.Name())

                files, _ := os.ReadDir(containerPath)
                for _, file := range files {
                        if file.IsDir() {
                                continue
                        }
                        info, _ := os.Stat(filepath.Join(containerPath, file.Name()))

                        blobs = append(blobs, Blob{
                                ID:           fmt.Sprintf("%s-%s", container.Name(), file.Name()), // simple unique ID
                                Name:         file.Name(),
                                Size:         info.Size(),
                                ContentType:  mime.TypeByExtension(filepath.Ext(file.Name())),
                                LastModified: info.ModTime().UTC().Format(time.RFC3339),
                                Container:    container.Name(),
                        })
                }
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(blobs)
}

// CreateBucket creates a new blob storage container
func CreateBucket(w http.ResponseWriter, r *http.Request) {
        var body struct {
        Name string `json:"name"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

        home, err := os.UserHomeDir()
        if err != nil {
                http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
                return
        }

        bucketPath := filepath.Join(home, ".opencloud", "blob_storage", body.Name)
        if err := os.Mkdir(bucketPath, 0755); err != nil {
                http.Error(w, "Failed to create container", http.StatusInternalServerError)
                return
        }

        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]string{"status": "ok", "container": body.Name})
}

// UploadObject uploads a file to a blob storage container
func UploadObject(w http.ResponseWriter, r *http.Request) {
    err := r.ParseMultipartForm(10 << 20) // 10MB limit
    if err != nil {
        http.Error(w, "Error parsing form data", http.StatusBadRequest)
        return
    }

    container := r.FormValue("container")
    file, handler, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "Error retrieving file", http.StatusBadRequest)
        return
    }
    defer file.Close()

    home, _ := os.UserHomeDir()
    containerPath := filepath.Join(home, ".opencloud", "blob_storage", container)
    os.MkdirAll(containerPath, 0755)

    dst, err := os.Create(filepath.Join(containerPath, handler.Filename))
    if err != nil {
        http.Error(w, "Error creating file", http.StatusInternalServerError)
        return
    }
    defer dst.Close()

    io.Copy(dst, file)

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
        "filename": handler.Filename,
        "container": container,
        })
}

// DeleteObject deletes a file from blob storage
func DeleteObject(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Container string `json:"container"`
        Name      string `json:"name"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    home, _ := os.UserHomeDir()
    filePath := filepath.Join(home, ".opencloud", "blob_storage", req.Container, req.Name)

    if err := os.Remove(filePath); err != nil {
        if os.IsNotExist(err) {
            http.Error(w, "File not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Error deleting file", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status":    "deleted",
        "container": req.Container,
        "name":      req.Name,
    })
}

// DeleteImageRequest represents the JSON payload for deleting a container image
type DeleteImageRequest struct {
	ImageName string `json:"imageName"`
}

// DeleteImage handles deletion of a container image from the containerd registry.
// It accepts a POST request with a JSON body containing the image name to delete.
func DeleteImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeleteImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.ImageName == "" {
		http.Error(w, "imageName is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	// Use the "buildkit" namespace where buildkitd stores built images
	ctx = namespaces.WithNamespace(ctx, "buildkit")

	cli, err := containerd.New(containerdSocket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to containerd: %v", err), http.StatusInternalServerError)
		return
	}
	defer cli.Close()

	if err := cli.ImageService().Delete(ctx, req.ImageName); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete image: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "deleted",
		"imageName": req.ImageName,
	})
}

// BuildImageRequest represents the JSON payload for building a container image
type BuildImageRequest struct {
	Dockerfile string `json:"dockerfile"`
	ImageName  string `json:"imageName"`
	Context    string `json:"context"`   // optional
	NoCache    bool   `json:"nocache"`
	Platform   string `json:"platform"`  // optional
}

// normalizeImageRef adds docker.io registry prefix if no registry is specified
func normalizeImageRef(imageRef string) string {
        if strings.Contains(imageRef, "/") {
                return imageRef
        }
        parts := strings.Split(imageRef, ":")
        if strings.Contains(parts[0], ".") {
                return imageRef
        }
        return "docker.io/library/" + imageRef
}


// validateImageName checks an image name for dangerous or invalid patterns.
// Returns an error string if invalid, or empty string if valid.
func validateImageName(name string) string {
	// Reject backslashes
	if strings.Contains(name, `\`) {
		return "image name must not contain backslashes"
	}
	// Reject names starting with a slash (absolute paths)
	if strings.HasPrefix(name, "/") {
		return "image name must not start with a slash"
	}
	// Reject path traversal attempts
	if strings.Contains(name, "..") {
		return "image name must not contain path traversal sequences"
	}
	// Reject double slashes
	if strings.Contains(name, "//") {
		return "image name must not contain consecutive slashes"
	}
	// Validate against the allowed character pattern
	if !imageNamePatternMixed.MatchString(name) {
		return "image name contains invalid characters"
	}
	return ""
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

// BuildImage handles building a container image using buildkitd and registers it with Containerd.
func BuildImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BuildImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	fmt.Printf("imageName %s", req.ImageName)
	fmt.Println(req.ImageName)

	if req.Dockerfile == "" || req.ImageName == "" {
		http.Error(w, "dockerfile and imageName are required", http.StatusBadRequest)
		return
	}

	// Validate that the Dockerfile contains a FROM instruction
	if !hasFromInstruction(req.Dockerfile) {
		http.Error(w, "dockerfile must contain a FROM instruction", http.StatusBadRequest)
		return
	}

	// Validate that the image name is safe and properly formatted
	if errMsg := validateImageName(req.ImageName); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	// Create temp directory for build
	tmpDir, err := os.MkdirTemp("", "opencloud-build-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp dir: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Write Dockerfile exactly as provided
	dfPath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dfPath, []byte(req.Dockerfile), 0644); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write Dockerfile: %v", err), http.StatusInternalServerError)
		return
	}

	// Optional: write context if provided
	// (Assumes raw text or tar extraction logic if needed)
	if req.Context != "" {
		ctxPath := filepath.Join(tmpDir, "context.txt")
		if err := os.WriteFile(ctxPath, []byte(req.Context), 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write context: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Build buildctl arguments
	args := []string{
		"--addr", "unix:///run/buildkit/buildkitd.sock",
		"build",
		"--frontend", "dockerfile.v0",
		"--local", "context=" + tmpDir,
		"--local", "dockerfile=" + tmpDir,
		"--opt", "worker=containerd",
		"--output", "type=image,name="+req.ImageName+",push=false,unpack=true",
	}

	// Optional platform
	if req.Platform != "" {
		args = append(args, "--opt", "platform="+req.Platform)
		//args = append(args, "--opt", "platform=linux/amd64")
	}

	// Optional no-cache
	if req.NoCache {
		args = append(args, "--no-cache")
	}

	buildCmd := exec.Command("buildctl", args...)

	out, err := buildCmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		http.Error(
			w,
			fmt.Sprintf("Build failed: %v\nOutput:\n%s", err, string(out)),
			http.StatusInternalServerError,
		)
		return
	}

	// Record the built image in the service ledger so it can be rebuilt if needed
	if ledgerErr := service_ledger.UpdateContainerImageEntry(
		req.ImageName,
		req.Dockerfile,
		req.Context,
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
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)


}

// DownloadObject downloads a file from blob storage
func DownloadObject(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Decode JSON body into a map
    var body map[string]string
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    container, ok1 := body["container"]
    name, ok2 := body["name"]
    if !ok1 || !ok2 || container == "" || name == "" {
        http.Error(w, "Missing container or name", http.StatusBadRequest)
        return
    }

    // Adjust this path to match your storage layout
        home, _ := os.UserHomeDir()
    filePath := filepath.Join(home, ".opencloud", "blob_storage", container, name)

    file, err := os.Open(filePath)
    if err != nil {
        http.Error(w, "File not found", http.StatusNotFound)
        return
    }
    defer file.Close()

    // Set headers so the browser downloads the file
    w.Header().Set("Content-Disposition", "attachment; filename="+name)
    w.Header().Set("Content-Type", "application/octet-stream")

    // Serve the file
    http.ServeFile(w, r, filePath)
}
