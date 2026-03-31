package storage

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	opencloudapi "github.com/WavexSoftware/OpenCloud/api"
)

// TestImageInfoEmptySliceMarshalsToJSONArray guards the frontend contract used by
// /storage/containers: an empty image list must encode as [] rather than null.
func TestImageInfoEmptySliceMarshalsToJSONArray(t *testing.T) {
	result := make([]opencloudapi.ImageInfo, 0)

	body, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal image list: %v", err)
	}

	if string(body) != "[]" {
		t.Fatalf("Expected empty image list to marshal as [], got %s", string(body))
	}
}

// TestExpandImageTagsSingleTag verifies that an image with one tag produces exactly
// one ImageInfo entry and that the localhost/ prefix is stripped from the display name.
func TestExpandImageTagsSingleTag(t *testing.T) {
	entry := imageTagEntry{
		ID:       "sha256:abc123",
		RepoTags: []string{"localhost/myapp:latest"},
		Names:    []string{"localhost/myapp:latest"},
	}

	result := expandImageTags(entry)

	if len(result) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result))
	}
	// The localhost/ prefix must have been stripped from the original tag.
	if result[0].Image == "localhost/myapp:latest" {
		t.Errorf("localhost/ prefix was not stripped; Image=%q", result[0].Image)
	}
	if result[0].Image != "myapp:latest" {
		t.Errorf("Expected Image=%q, got %q", "myapp:latest", result[0].Image)
	}
	if len(result[0].RepoTags) != 1 || result[0].RepoTags[0] != "myapp:latest" {
		t.Errorf("Expected RepoTags=[myapp:latest], got %v", result[0].RepoTags)
	}
}

// TestExpandImageTagsMultipleTags verifies that an image with multiple tags (e.g. a
// locally-built image that also carries the upstream base-image tag) produces one
// ImageInfo entry per tag – matching the per-row display of `podman images`.
func TestExpandImageTagsMultipleTags(t *testing.T) {
	entry := imageTagEntry{
		ID: "sha256:abc123",
		RepoTags: []string{
			"localhost/nginx_wil:latest",
			"docker.io/library/nginx:latest",
		},
	}

	result := expandImageTags(entry)

	if len(result) != 2 {
		t.Fatalf("Expected 2 entries (one per tag), got %d", len(result))
	}

	// Both entries must share the same image ID.
	for _, r := range result {
		if r.ID != entry.ID {
			t.Errorf("Expected ID=%q, got %q", entry.ID, r.ID)
		}
		if len(r.RepoTags) != 1 {
			t.Errorf("Expected exactly 1 RepoTag per entry, got %v", r.RepoTags)
		}
	}

	// localhost/ prefix must be stripped from the first tag.
	if result[0].Image != "nginx_wil:latest" {
		t.Errorf("Expected Image=%q, got %q", "nginx_wil:latest", result[0].Image)
	}
	if result[1].Image != "docker.io/library/nginx:latest" {
		t.Errorf("Expected Image=%q, got %q", "docker.io/library/nginx:latest", result[1].Image)
	}
}

// TestExpandImageTagsNoTags verifies that an image with no tags or names produces a
// single entry whose display name is the raw image ID.
func TestExpandImageTagsNoTags(t *testing.T) {
	entry := imageTagEntry{
		ID:       "sha256:deadbeef",
		RepoTags: nil,
		Names:    nil,
	}

	result := expandImageTags(entry)

	if len(result) != 1 {
		t.Fatalf("Expected 1 entry for untagged image, got %d", len(result))
	}
	if result[0].Image != entry.ID {
		t.Errorf("Expected Image=%q (raw ID), got %q", entry.ID, result[0].Image)
	}
	if result[0].RepoTags != nil {
		t.Errorf("Expected nil RepoTags for untagged image, got %v", result[0].RepoTags)
	}
}

// TestExpandImageTagsStripsLocalhostPrefix verifies the localhost/ prefix is removed.
func TestExpandImageTagsStripsLocalhostPrefix(t *testing.T) {
	entry := imageTagEntry{
		ID:       "sha256:abc123",
		RepoTags: []string{"localhost/myservice:v1"},
	}

	result := expandImageTags(entry)

	if len(result) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result))
	}
	if result[0].Image != "myservice:v1" {
		t.Errorf("Expected localhost/ to be stripped; got Image=%q", result[0].Image)
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
	req := httptest.NewRequest(http.MethodGet, "/get-images", nil)
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

// TestPullImageInvalidMethod tests that PullImage rejects non-POST requests.
func TestPullImageInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/pull-image", nil)
	w := httptest.NewRecorder()

	PullImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

// TestPullImageInvalidJSON tests that PullImage rejects malformed JSON.
func TestPullImageInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/pull-image", bytes.NewBufferString("{invalid"))
	w := httptest.NewRecorder()

	PullImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestPullImageMissingImageName tests that PullImage rejects an empty imageName.
func TestPullImageMissingImageName(t *testing.T) {
	body, _ := json.Marshal(PullImageRequest{ImageName: "", Registry: "docker.io"})
	req := httptest.NewRequest(http.MethodPost, "/pull-image", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	PullImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestPullImageInvalidRegistry tests that PullImage rejects an unsupported registry.
func TestPullImageInvalidRegistry(t *testing.T) {
	body, _ := json.Marshal(PullImageRequest{ImageName: "nginx:latest", Registry: "gcr.io"})
	req := httptest.NewRequest(http.MethodPost, "/pull-image", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	PullImage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestPullImageValidRequestReachesPodman tests that a valid PullImage request passes
// all validation and attempts to reach Podman (which may not be running in CI).
func TestPullImageValidRequestReachesPodman(t *testing.T) {
	body, _ := json.Marshal(PullImageRequest{ImageName: "nginx:latest", Registry: "docker.io"})
	req := httptest.NewRequest(http.MethodPost, "/pull-image", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	PullImage(w, req)

	resp := w.Result()
	// A BadRequest here would indicate that validation incorrectly rejected a valid request.
	if resp.StatusCode == http.StatusBadRequest {
		t.Errorf("Valid request should not return BadRequest; got %d", resp.StatusCode)
	}
}

// TestPullImageDefaultsToDockerHub tests that omitting registry defaults to docker.io.
func TestPullImageDefaultsToDockerHub(t *testing.T) {
	body, _ := json.Marshal(PullImageRequest{ImageName: "nginx:latest"})
	req := httptest.NewRequest(http.MethodPost, "/pull-image", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	PullImage(w, req)

	resp := w.Result()
	// BadRequest would indicate the empty registry was incorrectly rejected.
	if resp.StatusCode == http.StatusBadRequest {
		t.Errorf("Empty registry should default to docker.io and not return BadRequest; got %d", resp.StatusCode)
	}
}

// TestPullImageQuayIORegistry tests that quay.io is an accepted registry value.
func TestPullImageQuayIORegistry(t *testing.T) {
	body, _ := json.Marshal(PullImageRequest{ImageName: "prometheus/prometheus:latest", Registry: "quay.io"})
	req := httptest.NewRequest(http.MethodPost, "/pull-image", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	PullImage(w, req)

	resp := w.Result()
	if resp.StatusCode == http.StatusBadRequest {
		t.Errorf("quay.io should be a valid registry; got BadRequest")
	}
}
