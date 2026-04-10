package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	service_ledger "github.com/WavexSoftware/OpenCloud/service_ledger"
)

// Blob represents a single object stored in blob storage.
type Blob struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	ContentType  string `json:"contentType"`
	LastModified string `json:"lastModified"`
	Bucket       string `json:"bucket"`
}

// Bucket represents a blob storage bucket with aggregate metadata.
type Bucket struct {
	Name           string `json:"name"`
	ObjectCount    int    `json:"objectCount"`
	TotalSize      int64  `json:"totalSize"`
	LastModified   string `json:"lastModified"`
	ContainerMount bool   `json:"containerMount"`
}

// ListBlobBuckets returns a list of blob storage buckets with metadata.
func ListBlobBuckets(w http.ResponseWriter, r *http.Request) {
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

	var buckets []Bucket
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		bucketPath := filepath.Join(root, entry.Name())
		bucketInfo, err := os.Stat(bucketPath)
		if err != nil {
			continue
		}

		// Count objects and calculate total size
		files, _ := os.ReadDir(bucketPath)
		objectCount := 0
		var totalSize int64
		var lastModified time.Time = bucketInfo.ModTime()

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			objectCount++
			info, _ := os.Stat(filepath.Join(bucketPath, file.Name()))
			totalSize += info.Size()
			if info.ModTime().After(lastModified) {
				lastModified = info.ModTime()
			}
		}

		buckets = append(buckets, Bucket{
			Name:         entry.Name(),
			ObjectCount:  objectCount,
			TotalSize:    totalSize,
			LastModified: lastModified.UTC().Format(time.RFC3339),
		})
	}

	// Enrich buckets with container mount status from the service ledger
	allEntries, _ := service_ledger.GetAllBucketEntries()
	for i := range buckets {
		if entry, ok := allEntries[buckets[i].Name]; ok {
			buckets[i].ContainerMount = entry.ContainerMount
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buckets)
}

// ListContainerMountBuckets returns blob storage buckets that are marked as container volume mounts.
func ListContainerMountBuckets(w http.ResponseWriter, r *http.Request) {
	allEntries, err := service_ledger.GetAllBucketEntries()
	if err != nil {
		http.Error(w, "Failed to read bucket entries", http.StatusInternalServerError)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	root := filepath.Join(home, ".opencloud", "blob_storage")

	var buckets []Bucket
	for name, entry := range allEntries {
		if !entry.ContainerMount {
			continue
		}

		bucketPath := filepath.Join(root, name)
		bucketInfo, err := os.Stat(bucketPath)
		if err != nil {
			continue // bucket directory doesn't exist on disk, skip
		}

		files, _ := os.ReadDir(bucketPath)
		objectCount := 0
		var totalSize int64
		var lastModified time.Time = bucketInfo.ModTime()

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			objectCount++
			info, _ := os.Stat(filepath.Join(bucketPath, file.Name()))
			totalSize += info.Size()
			if info.ModTime().After(lastModified) {
				lastModified = info.ModTime()
			}
		}

		buckets = append(buckets, Bucket{
			Name:           name,
			ObjectCount:    objectCount,
			TotalSize:      totalSize,
			LastModified:   lastModified.UTC().Format(time.RFC3339),
			ContainerMount: true,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buckets)
}

// GetBlobBuckets returns blobs from all buckets or a specific bucket if specified.
func GetBlobBuckets(w http.ResponseWriter, r *http.Request) {
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	// Check if a specific bucket is requested via query parameter
	bucketFilter := r.URL.Query().Get("bucket")

	root := filepath.Join(home, ".opencloud", "blob_storage")
	entries, err := os.ReadDir(root)
	if err != nil {
		http.Error(w, "Failed to read blob storage directory", http.StatusInternalServerError)
		return
	}

	var blobs []Blob
	for _, bucket := range entries {
		if !bucket.IsDir() {
			continue
		}

		// Skip if a specific bucket is requested and this isn't it
		if bucketFilter != "" && bucket.Name() != bucketFilter {
			continue
		}

		bucketPath := filepath.Join(root, bucket.Name())

		files, _ := os.ReadDir(bucketPath)
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			info, _ := os.Stat(filepath.Join(bucketPath, file.Name()))

			blobs = append(blobs, Blob{
				ID:           fmt.Sprintf("%s-%s", bucket.Name(), file.Name()), // simple unique ID
				Name:         file.Name(),
				Size:         info.Size(),
				ContentType:  mime.TypeByExtension(filepath.Ext(file.Name())),
				LastModified: info.ModTime().UTC().Format(time.RFC3339),
				Bucket:       bucket.Name(),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blobs)
}

// CreateBucket creates a new blob storage bucket
func CreateBucket(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name           string `json:"name"`
		ContainerMount bool   `json:"containerMount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate bucket name: required, no spaces, and max 50 characters
	if body.Name == "" {
		http.Error(w, "Bucket name is required", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(body.Name, " \t\n\r") {
		http.Error(w, "Bucket name cannot contain spaces", http.StatusBadRequest)
		return
	}
	if len(body.Name) > 50 {
		http.Error(w, "Bucket name must be 50 characters or fewer", http.StatusBadRequest)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	bucketPath := filepath.Join(home, ".opencloud", "blob_storage", body.Name)
	if err := os.Mkdir(bucketPath, 0755); err != nil {
		http.Error(w, "Failed to create bucket", http.StatusInternalServerError)
		return
	}

	if ledgerErr := service_ledger.UpdateBucketEntry(body.Name, time.Now().UTC().Format(time.RFC3339), body.ContainerMount); ledgerErr != nil {
		log.Printf("Warning: failed to record bucket %s in service ledger: %v", body.Name, ledgerErr)
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "bucket": body.Name})
}

// RenameBucket renames an existing blob storage bucket
func RenameBucket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		CurrentName string `json:"currentName"`
		NewName     string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate current name is provided
	if body.CurrentName == "" {
		http.Error(w, "Current bucket name is required", http.StatusBadRequest)
		return
	}

	// Validate new name: required, no spaces, and max 50 characters
	if body.NewName == "" {
		http.Error(w, "New bucket name is required", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(body.NewName, " \t\n\r") {
		http.Error(w, "Bucket name cannot contain spaces", http.StatusBadRequest)
		return
	}
	if len(body.NewName) > 50 {
		http.Error(w, "Bucket name must be 50 characters or fewer", http.StatusBadRequest)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	basePath := filepath.Join(home, ".opencloud", "blob_storage")
	currentPath := filepath.Join(basePath, body.CurrentName)
	newPath := filepath.Join(basePath, body.NewName)

	// Ensure the current bucket exists
	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		http.Error(w, "Bucket not found", http.StatusNotFound)
		return
	}

	// Ensure the new name is not already taken
	if _, err := os.Stat(newPath); err == nil {
		http.Error(w, "A bucket with that name already exists", http.StatusConflict)
		return
	}

	if err := os.Rename(currentPath, newPath); err != nil {
		http.Error(w, "Failed to rename bucket", http.StatusInternalServerError)
		return
	}

	if ledgerErr := service_ledger.RenameBucketEntry(body.CurrentName, body.NewName); ledgerErr != nil {
		log.Printf("Warning: failed to rename bucket %s to %s in service ledger: %v", body.CurrentName, body.NewName, ledgerErr)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "bucket": body.NewName})
}

// UploadObject uploads a file to a blob storage bucket.
// It uses streaming multipart parsing so that files of any size can be uploaded
// without buffering the entire request body in memory or temporary files.
// The "bucket" field must appear before the "file" field in the multipart form.
func UploadObject(w http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Error parsing multipart form", http.StatusBadRequest)
		return
	}

	var bucket string
	var filename string

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error reading multipart data", http.StatusBadRequest)
			return
		}

		if part.FormName() == "bucket" {
			// Read the plain-text bucket field value.
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, part); err != nil {
				http.Error(w, "Error reading bucket field", http.StatusBadRequest)
				return
			}
			bucket = buf.String()
		} else if part.FileName() != "" {
			// Stream the file part directly to disk without buffering.
			if bucket == "" {
				http.Error(w, "Bucket field must appear before file in form", http.StatusBadRequest)
				return
			}
			filename = part.FileName()

			home, err := os.UserHomeDir()
			if err != nil {
				http.Error(w, "Error determining home directory", http.StatusInternalServerError)
				return
			}
			bucketPath := filepath.Join(home, ".opencloud", "blob_storage", bucket)
			if err := os.MkdirAll(bucketPath, 0755); err != nil {
				http.Error(w, "Error creating bucket directory", http.StatusInternalServerError)
				return
			}

			dst, err := os.Create(filepath.Join(bucketPath, filename))
			if err != nil {
				http.Error(w, "Error creating file", http.StatusInternalServerError)
				return
			}
			defer dst.Close()

			if _, err := io.Copy(dst, part); err != nil {
				fmt.Println(err)
				http.Error(w, "Error writing file", http.StatusInternalServerError)
				return
			}
		}
	}

	if bucket == "" || filename == "" {
		http.Error(w, "Missing bucket or file", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"filename": filename,
		"bucket":   bucket,
	})
}

// DeleteBucket deletes a blob storage bucket and all of its contents.
func DeleteBucket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if body.Name == "" {
		http.Error(w, "Bucket name is required", http.StatusBadRequest)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	bucketPath := filepath.Join(home, ".opencloud", "blob_storage", body.Name)

	if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
		http.Error(w, "Bucket not found", http.StatusNotFound)
		return
	}

	if err := os.RemoveAll(bucketPath); err != nil {
		http.Error(w, "Failed to delete bucket", http.StatusInternalServerError)
		return
	}

	if ledgerErr := service_ledger.DeleteBucketEntry(body.Name); ledgerErr != nil {
		log.Printf("Warning: failed to remove bucket %s from service ledger: %v", body.Name, ledgerErr)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "bucket": body.Name})
}

// DeleteObject deletes a file from blob storage
func DeleteObject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Bucket string `json:"bucket"`
		Name   string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	home, _ := os.UserHomeDir()
	filePath := filepath.Join(home, ".opencloud", "blob_storage", req.Bucket, req.Name)

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
		"status": "deleted",
		"bucket": req.Bucket,
		"name":   req.Name,
	})
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

	bucket, ok1 := body["bucket"]
	name, ok2 := body["name"]
	if !ok1 || !ok2 || bucket == "" || name == "" {
		http.Error(w, "Missing bucket or name", http.StatusBadRequest)
		return
	}

	// Adjust this path to match your storage layout
	home, _ := os.UserHomeDir()
	filePath := filepath.Join(home, ".opencloud", "blob_storage", bucket, name)

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
