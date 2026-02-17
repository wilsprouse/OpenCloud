package api

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"net/http"
	"context"
	"encoding/json"
	"mime"
	"time"
	"regexp"
	"strings"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/moby/buildkit/client"
	"github.com/distribution/reference"
)

const buildTimeout = 5 * time.Minute

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


// GetContainerRegistry lists all container images using containerd
func GetContainerRegistry(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	
	// Use the "default" namespace for containerd operations
	ctx = namespaces.WithNamespace(ctx, "default")

	// Connect to containerd socket (usually /run/containerd/containerd.sock)
	cli, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
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

/*

GetBlobBuckets()
- Reads from ~/.opencloud/blob_storage

*/
/*
ListBlobContainers returns a list of blob storage containers with metadata.
*/
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

/*
GetBlobBuckets returns blobs from all containers or a specific container if specified.
*/
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

// BuildImageRequest represents the request body for building an image
/*type BuildImageRequest struct {
	Dockerfile string `json:"dockerfile"`
	ImageName  string `json:"imageName"`
	Context    string `json:"context"`   // optional, default "."
	NoCache    bool   `json:"nocache"`   // optional
	Platform   string `json:"platform"`  // optional, default "linux/amd64"
}*/
type BuildImageRequest struct {
	Dockerfile string `json:"dockerfile"`
	ImageName  string `json:"imageName"`
	Context    string `json:"context,omitempty"`  // unused in this version
	Platform   string `json:"platform,omitempty"` // ex: linux/amd64
	NoCache    bool   `json:"noCache,omitempty"`
}

// BuildImage builds a container image using BuildKit + containerd
func BuildImage2(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	fmt.Println("juice")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req BuildImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	fmt.Println("juice2")

	// Validate required fields
	if req.Dockerfile == "" {
		http.Error(w, "Dockerfile content is required", http.StatusBadRequest)
		return
	}
	if req.ImageName == "" {
		http.Error(w, "Image name is required", http.StatusBadRequest)
		return
	}

	// Validate that Dockerfile contains FROM instruction (case-insensitive)
	// Allow comments and syntax directives before FROM
	hasFrom := false
	lines := strings.Split(req.Dockerfile, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Check if line starts with FROM (case-insensitive)
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM ") {
			hasFrom = true
			break
		}
		// If we hit a non-comment, non-FROM instruction, the Dockerfile is invalid
		break
	}
	if !hasFrom {
		http.Error(w, "Dockerfile must contain a FROM instruction", http.StatusBadRequest)
		return
	}

	// Validate image name for security (prevent path traversal and malicious names)
	if strings.Contains(req.ImageName, "..") {
		http.Error(w, "Invalid image name: path traversal not allowed", http.StatusBadRequest)
		return
	}
	if strings.Contains(req.ImageName, "//") {
		http.Error(w, "Invalid image name: double slashes not allowed", http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(req.ImageName, "/") {
		http.Error(w, "Invalid image name: absolute paths not allowed", http.StatusBadRequest)
		return
	}
	if strings.Contains(req.ImageName, "\\") {
		http.Error(w, "Invalid image name: backslashes not allowed", http.StatusBadRequest)
		return
	}
	fmt.Println("juice3")

	// Set default values for optional fields
	if req.Context == "" {
		req.Context = "."
	}
	if req.Platform == "" {
		req.Platform = "linux/amd64"
	}

	// Create a temporary directory for the build context
	tmpDir, err := os.MkdirTemp("", "buildkit-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp directory: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("juice4")
	// Write the Dockerfile to the temp directory
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(req.Dockerfile), 0600); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write Dockerfile: %v", err), http.StatusInternalServerError)
		return
	}

	// Connect to BuildKit using request context for proper cancellation
	ctx := r.Context()
	bkClient, err := client.New(ctx, "unix:///run/buildkit/buildkitd.sock")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to BuildKit: %v", err), http.StatusInternalServerError)
		return
	}
	defer bkClient.Close()

	// Configure build options
	solveOpt := &client.SolveOpt{
		LocalDirs: map[string]string{
			"context":    tmpDir,
			"dockerfile": tmpDir,
		},
		Frontend: "dockerfile.v0",
		FrontendAttrs: map[string]string{
			"filename": "Dockerfile",
			"platform": req.Platform,
		},
		Exports: []client.ExportEntry{
			{
				Type: "containerd",
				Attrs: map[string]string{
					"name": req.ImageName,
					"unpack": "true",
				},
			},
		},
	}
	fmt.Println("juice6")

	// Add no-cache option if requested
	if req.NoCache {
		solveOpt.FrontendAttrs["no-cache"] = ""
	}

	// Create channels for progress tracking
	//ch := make(chan *client.SolveStatus, buildProgressBufferSize)
	ch := make(chan *client.SolveStatus)

	go func() {
    		for range ch {
        		// drain
    		}
	}()

	_, err = bkClient.Solve(ctx, nil, *solveOpt, ch)
	if err != nil {
    		http.Error(w, fmt.Sprintf("Build failed: %v", err), http.StatusInternalServerError)
    		return
	}
	
	fmt.Println("juice8")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Image %s built successfully", req.ImageName),
		"image":   req.ImageName,
	})
}

func BuildImage(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req BuildImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Dockerfile) == "" {
		http.Error(w, "Dockerfile content is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ImageName) == "" {
		http.Error(w, "Image name is required", http.StatusBadRequest)
		return
	}

	// Normalize + validate image name (also ensures it has a tag)
	named, err := reference.ParseNormalizedNamed(strings.ToLower(strings.TrimSpace(req.ImageName)))
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid image name: %v", err), http.StatusBadRequest)
		return
	}
	named = reference.TagNameOnly(named)
	req.ImageName = named.String()

	// Validate that Dockerfile contains FROM instruction (case-insensitive)
	hasFrom := false
	lines := strings.Split(req.Dockerfile, "\n")
	for _, line := range lines {
		fmt.Println(line)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM ") {
			hasFrom = true
		}
		break
	}
	if !hasFrom {
		http.Error(w, "Dockerfile must contain a FROM instruction", http.StatusBadRequest)
		return
	}
	fmt.Println("juice1")

	// Set default values
	if strings.TrimSpace(req.Platform) == "" {
		req.Platform = "linux/amd64"
	}

	// Create a temporary directory for the build context
	tmpDir, err := os.MkdirTemp("", "buildkit-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp directory: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Write the Dockerfile to the temp directory
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(req.Dockerfile), 0600); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write Dockerfile: %v", err), http.StatusInternalServerError)
		return
	}

	// BuildKit connection + timeout
	ctx, cancel := context.WithTimeout(r.Context(), buildTimeout)
	defer cancel()
	fmt.Println("juice2")

	bkClient, err := client.New(ctx, "unix:///run/buildkit/buildkitd.sock")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to BuildKit: %v", err), http.StatusInternalServerError)
		return
	}
	defer bkClient.Close()

	// Configure build options
	/*solveOpt := &client.SolveOpt{
		LocalDirs: map[string]string{
			"context":    tmpDir,
			"dockerfile": tmpDir,
		},
		Frontend: "dockerfile.v0",
		FrontendAttrs: map[string]string{
			"filename": "Dockerfile",
			"platform": req.Platform,
		},
		Exports: []client.ExportEntry{
			{
				// This forces BuildKit to use the containerd worker
				// and ensures it shows up in:
				// sudo ctr -n buildkit images list
				Type: "containerd",
				Attrs: map[string]string{
					"name":   req.ImageName,
					"unpack": "true",
				},
			},
		},
	}*/
	solveOpt := &client.SolveOpt{
                LocalDirs: map[string]string{
                        "context":    tmpDir,
                        "dockerfile": tmpDir,
                },
                Frontend: "dockerfile.v0",
                FrontendAttrs: map[string]string{
                        "filename": "Dockerfile",
                },
                Exports: []client.ExportEntry{
                        {
                                Type: client.ExporterImage, // Push to containerd
                                Attrs: map[string]string{
                                        "name": req.ImageName,
                                        "push": "false", // store locally in containerd
                                },
                        },
                },
        }
	fmt.Println("juice3")

	if req.NoCache {
		solveOpt.FrontendAttrs["no-cache"] = ""
	}

	// Progress channel MUST be drained or Solve can hang
	ch := make(chan *client.SolveStatus)

	go func() {
		for range ch {
			// drain progress updates (optional: log them)
		}
	}()

	fmt.Println("juice4")
	// Run solve synchronously (no done channel, no deadlock)
	_, err = bkClient.Solve(ctx, nil, *solveOpt, ch)
	if err != nil {
		http.Error(w, fmt.Sprintf("Build failed: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Println("juice5")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Image %s built successfully", req.ImageName),
		"image":   req.ImageName,
	})
}

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
