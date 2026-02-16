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
	"github.com/moby/buildkit/util/progress/progressui"
)

// Pre-compiled regex patterns for image name validation
var (
	// Pattern for lowercase image names (after normalization)
	imageNamePatternLower = regexp.MustCompile(`^[a-z0-9]+(([._-]|__)[a-z0-9]+)*(:[a-z0-9]+(([._-]|__)[a-z0-9]+)*)*(/[a-z0-9]+(([._-]|__)[a-z0-9]+)*(:[a-z0-9]+(([._-]|__)[a-z0-9]+)*)*)*(@sha256:[a-f0-9]{64})?$`)
	// Pattern for mixed-case image names
	imageNamePatternMixed = regexp.MustCompile(`^[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*(:[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*)*(/[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*(:[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*)*)*$`)
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
type BuildImageRequest struct {
	Dockerfile string `json:"dockerfile"`
	ImageName  string `json:"imageName"`
	Context    string `json:"context"`   // optional, default "."
	NoCache    bool   `json:"nocache"`   // optional
	Platform   string `json:"platform"`  // optional, default "linux/amd64"
}

// BuildImage builds a container image using BuildKit + containerd
func BuildImage(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
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
