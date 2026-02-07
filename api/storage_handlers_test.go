package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
			expectedStatus: 0, // Any status is acceptable - will fail at buildkit/containerd connection
			description:    "Should pass validation",
		},
		{
			name: "Valid request with minimal fields",
			request: BuildImageRequest{
				Dockerfile: "FROM alpine:latest",
				ImageName:  "test-image",
			},
			expectedStatus: 0, // Any status is acceptable - will fail at buildkit/containerd connection
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
			expectedStatus: 0, // Valid, will fail at buildkit connection
			description:    "Should accept dockerfile with comments before FROM",
		},
		{
			name: "Valid - dockerfile with lowercase from",
			request: BuildImageRequest{
				Dockerfile: "from alpine:latest\nRUN echo test",
				ImageName:  "test-image",
			},
			expectedStatus: 0, // Valid, will fail at buildkit connection
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
			expectedStatus: 0, // Valid, will fail at buildkit connection
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
			// For valid requests, we accept any status since buildkit/containerd may not be available in test
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

	// This will fail to connect to containerd in test environment, but we're testing the handler setup
	GetContainerRegistry(w, req)

	resp := w.Result()
	// In a test environment without containerd, we expect an error
	// This test verifies the handler doesn't panic
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Logf("Handler returned status %d (expected OK or Internal Server Error in test env)", resp.StatusCode)
	}
}

// TestListBlobContainers tests the blob container listing
func TestListBlobContainers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/list-blob-containers", nil)
	w := httptest.NewRecorder()

	ListBlobContainers(w, req)

	resp := w.Result()
	// Should return 200 or 500 depending on whether .opencloud directory exists
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", resp.StatusCode)
	}
}

// TestGetBlobBuckets tests the blob bucket retrieval
func TestGetBlobBuckets(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/get-blob-buckets", nil)
	w := httptest.NewRecorder()

	GetBlobBuckets(w, req)

	resp := w.Result()
	// Should return 200 or 500 depending on whether .opencloud directory exists
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", resp.StatusCode)
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
			name: "Missing container",
			body: map[string]string{"name": "test.txt"},
		},
		{
			name: "Missing name",
			body: map[string]string{"container": "test-container"},
		},
		{
			name: "Empty container",
			body: map[string]string{"container": "", "name": "test.txt"},
		},
		{
			name: "Empty name",
			body: map[string]string{"container": "test-container", "name": ""},
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
