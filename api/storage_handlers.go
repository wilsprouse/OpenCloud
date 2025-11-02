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
	"github.com/docker/docker/api/types/image"
    "github.com/docker/docker/client"
)

type Blob struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	ContentType  string `json:"contentType"`
	LastModified string `json:"lastModified"`
	Container    string `json:"container"`
}

func GetContainerRegistry(w http.ResponseWriter, r *http.Request) {

	ctx := context.Background()

    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        panic(err)
    }

 //   images, err := cli.ImageList(ctx, types.ImageListOptions{
	//images, err := cli.ImageList(ctx, types.ImageListOptions{
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

/*

GetBlobBuckets()
- Reads from ~/.opencloud/blob_storage

*/
func GetBlobBuckets(w http.ResponseWriter, r *http.Request) {
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

	var blobs []Blob
	for _, container := range entries {
		if !container.IsDir() {
			continue
		}
		containerPath := filepath.Join(root, container.Name())

		files, _ := os.ReadDir(containerPath)
		for _, file := range files {
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
