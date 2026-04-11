package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	service_ledger "github.com/WavexSoftware/OpenCloud/service_ledger"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	entitiesTypes "github.com/containers/podman/v5/pkg/domain/entities/types"
)

// TestListBlobBuckets tests the blob bucket listing
func TestListBlobBuckets(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/list-blob-buckets", nil)
	w := httptest.NewRecorder()

	ListBlobBuckets(w, req)

	resp := w.Result()
	// Should return 200 or 500 depending on whether .opencloud directory exists
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", resp.StatusCode)
	}
}

// TestGetBlobBuckets tests the blob bucket retrieval
func TestGetBlobBuckets(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/get-blobs", nil)
	w := httptest.NewRecorder()

	GetBlobBuckets(w, req)

	resp := w.Result()
	// Should return 200 or 500 depending on whether .opencloud directory exists
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", resp.StatusCode)
	}
}

// newUploadRequest builds a multipart POST request with bucket and file fields.
// When bucketFirst is true the bucket field precedes the file, which is the
// order required by the streaming UploadObject handler.
func newUploadRequest(t *testing.T, bucket, filename string, fileContent []byte, bucketFirst bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	if bucketFirst {
		if err := mw.WriteField("bucket", bucket); err != nil {
			t.Fatalf("WriteField bucket: %v", err)
		}
	}

	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(fileContent); err != nil {
		t.Fatalf("Write file content: %v", err)
	}

	if !bucketFirst {
		if err := mw.WriteField("bucket", bucket); err != nil {
			t.Fatalf("WriteField bucket: %v", err)
		}
	}

	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload-object", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// TestUploadObjectSuccess tests that a valid upload writes the file to disk.
func TestUploadObjectSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	req := newUploadRequest(t, "test-bucket", "hello.txt", []byte("hello world"), true)
	w := httptest.NewRecorder()

	UploadObject(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	uploadedPath := filepath.Join(tmpDir, ".opencloud", "blob_storage", "test-bucket", "hello.txt")
	data, err := os.ReadFile(uploadedPath)
	if err != nil {
		t.Fatalf("Uploaded file not found: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("File content mismatch: got %q", string(data))
	}
}

// TestUploadObjectLargeFile tests that files larger than the old 10 MB in-memory
// limit upload successfully using the streaming multipart handler.
func TestUploadObjectLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// 15 MB payload — larger than the previous ParseMultipartForm(10<<20) limit.
	largeContent := bytes.Repeat([]byte("x"), 15<<20)
	req := newUploadRequest(t, "large-bucket", "big.bin", largeContent, true)
	w := httptest.NewRecorder()

	UploadObject(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	uploadedPath := filepath.Join(tmpDir, ".opencloud", "blob_storage", "large-bucket", "big.bin")
	info, err := os.Stat(uploadedPath)
	if err != nil {
		t.Fatalf("Uploaded file not found: %v", err)
	}
	if info.Size() != int64(len(largeContent)) {
		t.Errorf("File size mismatch: want %d, got %d", len(largeContent), info.Size())
	}
}

// zeroReader is an io.Reader that returns an endless stream of zero bytes,
// used by large-file tests to generate payload without heap allocations.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// newStreamingUploadRequest builds a multipart POST whose file body is generated
// on the fly by src, up to fileSize bytes. Using a pipe avoids allocating the
// entire payload in memory, making it suitable for very large file tests.
// bucket is always written as the first field.
func newStreamingUploadRequest(t *testing.T, bucket, filename string, src io.Reader, fileSize int64) *http.Request {
	t.Helper()
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		var writeErr error
		defer func() {
			mw.Close()
			pw.CloseWithError(writeErr)
		}()

		if err := mw.WriteField("bucket", bucket); err != nil {
			writeErr = err
			return
		}
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			writeErr = err
			return
		}
		if _, err := io.Copy(fw, io.LimitReader(src, fileSize)); err != nil {
			writeErr = err
		}
	}()

	req := httptest.NewRequest(http.MethodPost, "/upload-object", pr)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// TestUploadObjectVeryLargeFile verifies that a 500 MB upload succeeds.
// The payload is streamed from a zero-reader so no heap allocation is needed.
func TestUploadObjectVeryLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	const fileSize = 500 << 20 // 500 MB
	req := newStreamingUploadRequest(t, "large-bucket", "huge.bin", zeroReader{}, fileSize)
	w := httptest.NewRecorder()

	UploadObject(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	uploadedPath := filepath.Join(tmpDir, ".opencloud", "blob_storage", "large-bucket", "huge.bin")
	info, err := os.Stat(uploadedPath)
	if err != nil {
		t.Fatalf("Uploaded file not found: %v", err)
	}
	if info.Size() != fileSize {
		t.Errorf("File size mismatch: want %d, got %d", fileSize, info.Size())
	}
}

// TestUploadObjectBucketAfterFile tests that placing the bucket field after
// the file field returns 400 — the streaming handler requires the bucket name
// before it can begin writing the file to disk.
func TestUploadObjectBucketAfterFile(t *testing.T) {
	req := newUploadRequest(t, "my-bucket", "test.txt", []byte("data"), false)
	w := httptest.NewRecorder()

	UploadObject(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d when bucket follows file, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestUploadObjectInvalidMultipart tests that a non-multipart request returns 400.
func TestUploadObjectInvalidMultipart(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/upload-object", bytes.NewBufferString("not-multipart"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	UploadObject(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestUploadObjectMissingBucket tests that a request with no bucket field returns 400.
func TestUploadObjectMissingBucket(t *testing.T) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte("data")); err != nil {
		t.Fatalf("Write file content: %v", err)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload-object", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	UploadObject(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestCreateBucketInvalidJSON tests CreateBucket with invalid JSON
func TestCreateBucketInvalidJSON(t *testing.T) {
	invalidJSON := bytes.NewBufferString("{invalid json")
	req := httptest.NewRequest(http.MethodPost, "/create-bucket", invalidJSON)
	w := httptest.NewRecorder()

	CreateBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestDeleteObjectInvalidJSON tests DeleteObject with invalid JSON
func TestDeleteObjectInvalidJSON(t *testing.T) {
	invalidJSON := bytes.NewBufferString("{invalid json")
	req := httptest.NewRequest(http.MethodPost, "/delete-object", invalidJSON)
	w := httptest.NewRecorder()

	DeleteObject(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestDownloadObjectInvalidMethod tests DownloadObject with wrong HTTP method
func TestDownloadObjectInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/download-object", nil)
	w := httptest.NewRecorder()

	DownloadObject(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

// TestDownloadObjectInvalidJSON tests DownloadObject with invalid JSON
func TestDownloadObjectInvalidJSON(t *testing.T) {
	invalidJSON := bytes.NewBufferString("{invalid json")
	req := httptest.NewRequest(http.MethodPost, "/download-object", invalidJSON)
	w := httptest.NewRecorder()

	DownloadObject(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestDownloadObjectMissingFields tests DownloadObject with missing required fields
func TestDownloadObjectMissingFields(t *testing.T) {
	testCases := []struct {
		name string
		body map[string]string
	}{
		{
			name: "Missing bucket",
			body: map[string]string{"name": "test.txt"},
		},
		{
			name: "Missing name",
			body: map[string]string{"bucket": "test-bucket"},
		},
		{
			name: "Empty bucket",
			body: map[string]string{"bucket": "", "name": "test.txt"},
		},
		{
			name: "Empty name",
			body: map[string]string{"bucket": "test-bucket", "name": ""},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/download-object", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			DownloadObject(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}
		})
	}
}

// TestRenameBucketInvalidMethod tests that RenameBucket rejects non-PUT requests
func TestRenameBucketInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rename-bucket", nil)
	w := httptest.NewRecorder()

	RenameBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

// TestRenameBucketInvalidJSON tests that RenameBucket rejects invalid JSON
func TestRenameBucketInvalidJSON(t *testing.T) {
	invalidJSON := bytes.NewBufferString("{invalid json")
	req := httptest.NewRequest(http.MethodPut, "/rename-bucket", invalidJSON)
	w := httptest.NewRecorder()

	RenameBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestRenameBucketValidation tests the input validation rules for RenameBucket
func TestRenameBucketValidation(t *testing.T) {
	testCases := []struct {
		name           string
		currentName    string
		newName        string
		expectedStatus int
		description    string
	}{
		{
			name:           "Missing currentName",
			currentName:    "",
			newName:        "new-name",
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject empty currentName",
		},
		{
			name:           "Missing newName",
			currentName:    "old-name",
			newName:        "",
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject empty newName",
		},
		{
			name:           "New name with space",
			currentName:    "old-name",
			newName:        "new name",
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject newName containing a space",
		},
		{
			name:           "New name with tab",
			currentName:    "old-name",
			newName:        "new\tname",
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject newName containing a tab",
		},
		{
			name:           "New name exceeds 50 characters",
			currentName:    "old-name",
			newName:        "this-name-is-way-too-long-and-exceeds-fifty-characters",
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject newName longer than 50 characters",
		},
		{
			name:           "Non-existent current bucket",
			currentName:    "does-not-exist-12345",
			newName:        "new-name-12345",
			expectedStatus: http.StatusNotFound,
			description:    "Should return 404 when current bucket does not exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{
				"currentName": tc.currentName,
				"newName":     tc.newName,
			})
			req := httptest.NewRequest(http.MethodPut, "/rename-bucket", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			RenameBucket(w, req)

			resp := w.Result()
			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d", tc.description, tc.expectedStatus, resp.StatusCode)
			}
		})
	}
}

// TestRenameBucketSuccess tests that RenameBucket succeeds when the bucket exists
func TestRenameBucketSuccess(t *testing.T) {
	// Create a temporary directory to act as the blob_storage base
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine home directory, skipping test")
	}

	basePath := filepath.Join(home, ".opencloud", "blob_storage")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		t.Skipf("Cannot create blob_storage directory: %v", err)
	}

	// Create a source bucket
	srcName := "test-rename-src-bucket"
	dstName := "test-rename-dst-bucket"
	srcPath := filepath.Join(basePath, srcName)
	dstPath := filepath.Join(basePath, dstName)

	// Clean up before and after the test
	os.RemoveAll(srcPath)
	os.RemoveAll(dstPath)
	defer os.RemoveAll(srcPath)
	defer os.RemoveAll(dstPath)

	if err := os.Mkdir(srcPath, 0755); err != nil {
		t.Fatalf("Failed to create source bucket for test: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"currentName": srcName,
		"newName":     dstName,
	})
	req := httptest.NewRequest(http.MethodPut, "/rename-bucket", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	RenameBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify source directory was removed and destination exists
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Errorf("Expected source bucket to be removed after rename")
	}
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Errorf("Expected destination bucket to exist after rename")
	}
}

// TestRenameBucketConflict tests that RenameBucket rejects a rename when the new name is taken
func TestRenameBucketConflict(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine home directory, skipping test")
	}

	basePath := filepath.Join(home, ".opencloud", "blob_storage")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		t.Skipf("Cannot create blob_storage directory: %v", err)
	}

	srcName := "test-conflict-src"
	dstName := "test-conflict-dst"
	srcPath := filepath.Join(basePath, srcName)
	dstPath := filepath.Join(basePath, dstName)

	os.RemoveAll(srcPath)
	os.RemoveAll(dstPath)
	defer os.RemoveAll(srcPath)
	defer os.RemoveAll(dstPath)

	// Create both source and destination directories
	if err := os.Mkdir(srcPath, 0755); err != nil {
		t.Fatalf("Failed to create source bucket: %v", err)
	}
	if err := os.Mkdir(dstPath, 0755); err != nil {
		t.Fatalf("Failed to create destination bucket: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"currentName": srcName,
		"newName":     dstName,
	})
	req := httptest.NewRequest(http.MethodPut, "/rename-bucket", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	RenameBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

// TestDeleteBucketInvalidMethod tests that DeleteBucket rejects non-DELETE requests
func TestDeleteBucketInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/delete-bucket", nil)
	w := httptest.NewRecorder()

	DeleteBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

// TestDeleteBucketInvalidJSON tests that DeleteBucket rejects invalid JSON
func TestDeleteBucketInvalidJSON(t *testing.T) {
	invalidJSON := bytes.NewBufferString("{invalid json")
	req := httptest.NewRequest(http.MethodDelete, "/delete-bucket", invalidJSON)
	w := httptest.NewRecorder()

	DeleteBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestDeleteBucketMissingName tests that DeleteBucket rejects a request with no bucket name
func TestDeleteBucketMissingName(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"name": ""})
	req := httptest.NewRequest(http.MethodDelete, "/delete-bucket", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	DeleteBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestDeleteBucketNotFound tests that DeleteBucket returns 404 for a non-existent bucket
func TestDeleteBucketNotFound(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"name": "bucket-that-does-not-exist-xyz"})
	req := httptest.NewRequest(http.MethodDelete, "/delete-bucket", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	DeleteBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

// TestDeleteBucketSuccess tests that DeleteBucket removes an existing bucket and its objects
func TestDeleteBucketSuccess(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine home directory, skipping test")
	}

	basePath := filepath.Join(home, ".opencloud", "blob_storage")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		t.Skipf("Cannot create blob_storage directory: %v", err)
	}

	bucketName := "test-delete-bucket-success"
	bucketPath := filepath.Join(basePath, bucketName)

	os.RemoveAll(bucketPath)

	if err := os.Mkdir(bucketPath, 0755); err != nil {
		t.Fatalf("Failed to create test bucket: %v", err)
	}

	// Place an object inside the bucket to verify recursive deletion
	objectPath := filepath.Join(bucketPath, "test-object.txt")
	if err := os.WriteFile(objectPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test object: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"name": bucketName})
	req := httptest.NewRequest(http.MethodDelete, "/delete-bucket", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	DeleteBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the bucket directory was removed
	if _, err := os.Stat(bucketPath); !os.IsNotExist(err) {
		t.Errorf("Expected bucket directory to be removed after deletion")
		os.RemoveAll(bucketPath)
	}
}

// TestCreateBucketWithContainerMount tests that creating a bucket with containerMount=true
// succeeds, the bucket directory is created on disk, and a Podman named volume is requested.
func TestCreateBucketWithContainerMount(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	blobStoragePath := filepath.Join(tmpDir, ".opencloud", "blob_storage")
	if err := os.MkdirAll(blobStoragePath, 0755); err != nil {
		t.Fatalf("Failed to create blob_storage directory: %v", err)
	}

	// Track what volume name was requested
	var capturedVolumeName string
	origCreate := createPodmanVolume
	origConn := blobStoragePodmanConnection
	t.Cleanup(func() {
		createPodmanVolume = origCreate
		blobStoragePodmanConnection = origConn
	})
	blobStoragePodmanConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	createPodmanVolume = func(ctx context.Context, opts entitiesTypes.VolumeCreateOptions, _ *volumes.CreateOptions) (*entitiesTypes.VolumeConfigResponse, error) {
		capturedVolumeName = opts.Name
		return nil, nil
	}

	bucketName := "mount-test-bucket"
	body, _ := json.Marshal(map[string]interface{}{
		"name":           bucketName,
		"containerMount": true,
	})

	req := httptest.NewRequest(http.MethodPost, "/create-bucket", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	CreateBucket(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Verify the bucket directory was created
	bucketPath := filepath.Join(blobStoragePath, bucketName)
	if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
		t.Errorf("Expected bucket directory to exist at %s", bucketPath)
	}

	// Verify a Podman volume was requested with the expected name
	expectedVolumeName := podmanVolumePrefix + bucketName
	if capturedVolumeName != expectedVolumeName {
		t.Errorf("Expected Podman volume name %q, got %q", expectedVolumeName, capturedVolumeName)
	}

	// Verify response body
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["bucket"] != bucketName {
		t.Errorf("Expected bucket name %q in response, got %q", bucketName, result["bucket"])
	}
	if result["volumeName"] != expectedVolumeName {
		t.Errorf("Expected volumeName %q in response, got %q", expectedVolumeName, result["volumeName"])
	}
}

// TestListContainerMountBucketsEmpty tests that listing container mount buckets returns
// an empty result when no buckets are marked as container mounts.
func TestListContainerMountBucketsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	blobStoragePath := filepath.Join(tmpDir, ".opencloud", "blob_storage")
	if err := os.MkdirAll(blobStoragePath, 0755); err != nil {
		t.Fatalf("Failed to create blob_storage directory: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/list-container-mount-buckets", nil)
	w := httptest.NewRecorder()

	ListContainerMountBuckets(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var buckets []Bucket
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &buckets); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(buckets) != 0 {
		t.Errorf("Expected 0 container mount buckets, got %d", len(buckets))
	}
}

// TestListContainerMountBuckets tests that only buckets marked as container mounts
// are returned by the ListContainerMountBuckets handler.
func TestListContainerMountBuckets(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	blobStoragePath := filepath.Join(tmpDir, ".opencloud", "blob_storage")
	if err := os.MkdirAll(blobStoragePath, 0755); err != nil {
		t.Fatalf("Failed to create blob_storage directory: %v", err)
	}

	// Create two buckets on disk: one mount, one non-mount
	mountBucket := "mount-bucket"
	normalBucket := "normal-bucket"
	for _, name := range []string{mountBucket, normalBucket} {
		if err := os.Mkdir(filepath.Join(blobStoragePath, name), 0755); err != nil {
			t.Fatalf("Failed to create bucket dir %q: %v", name, err)
		}
	}

	// Add a file to the mount bucket so we can verify objectCount/totalSize
	testContent := []byte("hello mount")
	if err := os.WriteFile(filepath.Join(blobStoragePath, mountBucket, "test.txt"), testContent, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Register buckets in the service ledger directly since the bucket dirs already exist
	if err := service_ledger.UpdateBucketEntry(mountBucket, "2024-01-01T00:00:00Z", true, "opencloud-"+mountBucket); err != nil {
		t.Fatalf("Failed to update mount bucket ledger entry: %v", err)
	}
	if err := service_ledger.UpdateBucketEntry(normalBucket, "2024-01-01T00:00:00Z", false, ""); err != nil {
		t.Fatalf("Failed to update normal bucket ledger entry: %v", err)
	}

	// Now call ListContainerMountBuckets
	req := httptest.NewRequest(http.MethodGet, "/list-container-mount-buckets", nil)
	w := httptest.NewRecorder()

	ListContainerMountBuckets(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var buckets []Bucket
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &buckets); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Only the mount bucket should be returned
	if len(buckets) != 1 {
		t.Fatalf("Expected 1 container mount bucket, got %d", len(buckets))
	}

	if buckets[0].Name != mountBucket {
		t.Errorf("Expected bucket name %q, got %q", mountBucket, buckets[0].Name)
	}
	if !buckets[0].ContainerMount {
		t.Error("Expected ContainerMount to be true")
	}
	if buckets[0].ObjectCount != 1 {
		t.Errorf("Expected objectCount 1, got %d", buckets[0].ObjectCount)
	}
	if buckets[0].TotalSize != int64(len(testContent)) {
		t.Errorf("Expected totalSize %d, got %d", len(testContent), buckets[0].TotalSize)
	}
}
