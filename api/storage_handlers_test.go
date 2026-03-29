package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestImageInfoEmptySliceMarshalsToJSONArray guards the frontend contract used by
// /storage/containers: an empty image list must encode as [] rather than null.
func TestImageInfoEmptySliceMarshalsToJSONArray(t *testing.T) {
	result := make([]ImageInfo, 0)

	body, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal image list: %v", err)
	}

	if string(body) != "[]" {
		t.Fatalf("Expected empty image list to marshal as [], got %s", string(body))
	}
}

// TestBuildImageInvalidMethod tests that BuildImage rejects non-POST requests
func TestBuildImageInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/build-image", nil)
	w := httptest.NewRecorder()

	BuildImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

// TestBuildImageInvalidJSON tests that BuildImage rejects invalid JSON
func TestBuildImageInvalidJSON(t *testing.T) {
	invalidJSON := bytes.NewBufferString("{invalid json")
	req := httptest.NewRequest(http.MethodPost, "/build-image", invalidJSON)
	w := httptest.NewRecorder()

	BuildImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestBuildImageMissingDockerfile tests that BuildImage rejects missing dockerfile
func TestBuildImageMissingDockerfile(t *testing.T) {
	reqBody := BuildImageRequest{
		ImageName: "test-image",
		Context:   ".",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/build-image", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	BuildImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestBuildImageMissingImageName tests that BuildImage rejects missing image name
func TestBuildImageMissingImageName(t *testing.T) {
	reqBody := BuildImageRequest{
		Dockerfile: "FROM alpine:latest\nRUN echo 'test'",
		Context:    ".",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/build-image", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	BuildImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestBuildImageRequestValidation tests the validation of BuildImageRequest
func TestBuildImageRequestValidation(t *testing.T) {
	testCases := []struct {
		name           string
		request        BuildImageRequest
		expectedStatus int
		description    string
	}{
		{
			name: "Valid request with all fields",
			request: BuildImageRequest{
				Dockerfile: "FROM alpine:latest",
				ImageName:  "test-image:latest",
				Context:    "/tmp/build",
				NoCache:    true,
				Platform:   "linux/amd64",
			},
			expectedStatus: 0, // Any status is acceptable - may fail at Podman connection
			description:    "Should pass validation",
		},
		{
			name: "Valid request with minimal fields",
			request: BuildImageRequest{
				Dockerfile: "FROM alpine:latest",
				ImageName:  "test-image",
			},
			expectedStatus: 0, // Any status is acceptable - may fail at Podman connection
			description:    "Should use default values for context and platform",
		},
		{
			name: "Invalid - empty dockerfile",
			request: BuildImageRequest{
				Dockerfile: "",
				ImageName:  "test-image",
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject empty dockerfile",
		},
		{
			name: "Invalid - empty image name",
			request: BuildImageRequest{
				Dockerfile: "FROM alpine:latest",
				ImageName:  "",
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject empty image name",
		},
		{
			name: "Invalid - both empty",
			request: BuildImageRequest{
				Dockerfile: "",
				ImageName:  "",
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject when both required fields are empty",
		},
		{
			name: "Invalid - dockerfile without FROM",
			request: BuildImageRequest{
				Dockerfile: "RUN echo 'test'",
				ImageName:  "test-image",
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject dockerfile that doesn't have FROM instruction",
		},
		{
			name: "Valid - dockerfile with comments before FROM",
			request: BuildImageRequest{
				Dockerfile: "# This is a comment\n# syntax=docker/dockerfile:1\nFROM alpine:latest\nRUN echo test",
				ImageName:  "test-image",
			},
			expectedStatus: 0, // Valid, may fail at Podman connection
			description:    "Should accept dockerfile with comments before FROM",
		},
		{
			name: "Valid - dockerfile with lowercase from",
			request: BuildImageRequest{
				Dockerfile: "from alpine:latest\nRUN echo test",
				ImageName:  "test-image",
			},
			expectedStatus: 0, // Valid, may fail at Podman connection
			description:    "Should accept dockerfile with lowercase from",
		},
		{
			name: "Invalid - image name with path traversal",
			request: BuildImageRequest{
				Dockerfile: "FROM alpine:latest",
				ImageName:  "../../../etc/passwd",
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject image name with path traversal attempt",
		},
		{
			name: "Invalid - image name with double slashes",
			request: BuildImageRequest{
				Dockerfile: "FROM alpine:latest",
				ImageName:  "registry.io//malicious",
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject image name with double slashes",
		},
		{
			name: "Invalid - image name with absolute path",
			request: BuildImageRequest{
				Dockerfile: "FROM alpine:latest",
				ImageName:  "/etc/passwd",
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject image name starting with slash",
		},
		{
			name: "Invalid - image name with backslash",
			request: BuildImageRequest{
				Dockerfile: "FROM alpine:latest",
				ImageName:  "test\\image",
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject image name with backslash",
		},
		{
			name: "Valid - image with registry and tag",
			request: BuildImageRequest{
				Dockerfile: "FROM alpine:latest",
				ImageName:  "registry.io/namespace/myapp:v1.0",
			},
			expectedStatus: 0, // Valid, may fail at Podman connection
			description:    "Should accept properly formatted image with registry",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/build-image", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			BuildImage(w, req)

			resp := w.Result()
			// For valid requests, we accept any status since Podman may not be available in test
			// For invalid requests, we check for BadRequest
			if tc.expectedStatus != 0 && resp.StatusCode != tc.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d", tc.description, tc.expectedStatus, resp.StatusCode)
			} else if tc.expectedStatus == 0 && resp.StatusCode == http.StatusBadRequest {
				t.Errorf("%s: Request validation should pass but got BadRequest", tc.description)
			}
		})
	}
}

// TestGetContainerRegistryHandler tests the GetContainerRegistry handler
func TestGetContainerRegistryHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/get-containers", nil)
	w := httptest.NewRecorder()

	// This may fail to connect to Podman in the test environment, but we're testing the handler setup
	GetContainerRegistry(w, req)

	resp := w.Result()
	// In a test environment without Podman, we expect an error
	// This test verifies the handler doesn't panic
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Logf("Handler returned status %d (expected OK or Internal Server Error in test env)", resp.StatusCode)
	}
}

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

// TestDeleteImageInvalidMethod tests that DeleteImage rejects non-POST requests
func TestDeleteImageInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/delete-image", nil)
	w := httptest.NewRecorder()

	DeleteImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

// TestDeleteImageInvalidJSON tests that DeleteImage rejects invalid JSON
func TestDeleteImageInvalidJSON(t *testing.T) {
	invalidJSON := bytes.NewBufferString("{invalid json")
	req := httptest.NewRequest(http.MethodPost, "/delete-image", invalidJSON)
	w := httptest.NewRecorder()

	DeleteImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestDeleteImageMissingImageName tests that DeleteImage rejects a missing imageName field
func TestDeleteImageMissingImageName(t *testing.T) {
	body, _ := json.Marshal(DeleteImageRequest{ImageName: ""})
	req := httptest.NewRequest(http.MethodPost, "/delete-image", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	DeleteImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestDeleteImageConnectsToPodman tests that a valid request reaches the Podman connection step
func TestDeleteImageConnectsToPodman(t *testing.T) {
	body, _ := json.Marshal(DeleteImageRequest{ImageName: "my-app:latest"})
	req := httptest.NewRequest(http.MethodPost, "/delete-image", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	DeleteImage(w, req)

	resp := w.Result()
	// In a test environment without Podman, we expect an InternalServerError at the connection step.
	// A BadRequest here would indicate incorrect validation logic.
	if resp.StatusCode == http.StatusBadRequest {
		t.Errorf("Valid request should not return BadRequest; got %d", resp.StatusCode)
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
