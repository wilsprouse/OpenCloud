package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
)

func TestUpdatePipeline(t *testing.T) {
	// Setup: Create a temporary directory for test pipelines
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	pipelineDir := filepath.Join(tmpHome, ".opencloud", "pipelines")
	if err := os.MkdirAll(pipelineDir, 0755); err != nil {
		t.Fatalf("Failed to create test pipeline directory: %v", err)
	}

	// Create a test pipeline in the service ledger
	testPipelineID := "test123"
	testName := "test-pipeline"
	testDescription := "Test Description"
	testCode := "#!/bin/bash\necho 'original code'"
	testBranch := "main"
	testStatus := "idle"
	createdAt := time.Now().Format(time.RFC3339)

	// Write the original pipeline file
	pipelineFileName := sanitizePipelineName(testName) + ".sh"
	pipelinePath := filepath.Join(pipelineDir, pipelineFileName)
	if err := os.WriteFile(pipelinePath, []byte(testCode), 0755); err != nil {
		t.Fatalf("Failed to create test pipeline file: %v", err)
	}

	// Add to service ledger
	if err := service_ledger.UpdatePipelineEntry(testPipelineID, testName, testDescription, testCode, testBranch, testStatus, createdAt); err != nil {
		t.Fatalf("Failed to create test pipeline entry: %v", err)
	}

	// Test updating the pipeline
	updatedCode := "#!/bin/bash\necho 'updated code'"
	updatedDescription := "Updated Description"
	updatedBranch := "develop"

	updateReq := UpdatePipelineRequest{
		Name:        testName,
		Description: updatedDescription,
		Code:        updatedCode,
		Branch:      updatedBranch,
	}

	body, err := json.Marshal(updateReq)
	if err != nil {
		t.Fatalf("Failed to marshal update request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/update-pipeline/"+testPipelineID, bytes.NewReader(body))
	w := httptest.NewRecorder()

	UpdatePipeline(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify the response
	var response Pipeline
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.ID != testPipelineID {
		t.Errorf("Expected ID %s, got %s", testPipelineID, response.ID)
	}
	if response.Name != testName {
		t.Errorf("Expected name %s, got %s", testName, response.Name)
	}
	if response.Description != updatedDescription {
		t.Errorf("Expected description %s, got %s", updatedDescription, response.Description)
	}
	if response.Code != updatedCode {
		t.Errorf("Expected code %s, got %s", updatedCode, response.Code)
	}
	if response.Branch != updatedBranch {
		t.Errorf("Expected branch %s, got %s", updatedBranch, response.Branch)
	}

	// Verify the file was updated
	fileContent, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("Failed to read updated pipeline file: %v", err)
	}
	if string(fileContent) != updatedCode {
		t.Errorf("Expected file content %s, got %s", updatedCode, string(fileContent))
	}

	// Verify the service ledger was updated
	ledgerEntry, err := service_ledger.GetPipelineEntry(testPipelineID)
	if err != nil {
		t.Fatalf("Failed to get pipeline entry from ledger: %v", err)
	}
	if ledgerEntry == nil {
		t.Fatal("Pipeline entry not found in ledger")
	}
	if ledgerEntry.Description != updatedDescription {
		t.Errorf("Expected ledger description %s, got %s", updatedDescription, ledgerEntry.Description)
	}
	if ledgerEntry.Code != updatedCode {
		t.Errorf("Expected ledger code %s, got %s", updatedCode, ledgerEntry.Code)
	}
	if ledgerEntry.Branch != updatedBranch {
		t.Errorf("Expected ledger branch %s, got %s", updatedBranch, ledgerEntry.Branch)
	}
}

func TestUpdatePipelineWithNameChange(t *testing.T) {
	// Setup: Create a temporary directory for test pipelines
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	pipelineDir := filepath.Join(tmpHome, ".opencloud", "pipelines")
	if err := os.MkdirAll(pipelineDir, 0755); err != nil {
		t.Fatalf("Failed to create test pipeline directory: %v", err)
	}

	// Create a test pipeline
	testPipelineID := "test456"
	testName := "old-name"
	testCode := "#!/bin/bash\necho 'test'"
	createdAt := time.Now().Format(time.RFC3339)

	// Write the original pipeline file
	oldFileName := sanitizePipelineName(testName) + ".sh"
	oldFilePath := filepath.Join(pipelineDir, oldFileName)
	if err := os.WriteFile(oldFilePath, []byte(testCode), 0755); err != nil {
		t.Fatalf("Failed to create test pipeline file: %v", err)
	}

	// Add to service ledger
	if err := service_ledger.UpdatePipelineEntry(testPipelineID, testName, "", testCode, "main", "idle", createdAt); err != nil {
		t.Fatalf("Failed to create test pipeline entry: %v", err)
	}

	// Update with a new name
	newName := "new-name"
	updateReq := UpdatePipelineRequest{
		Name:        newName,
		Description: "Updated",
		Code:        testCode,
		Branch:      "main",
	}

	body, err := json.Marshal(updateReq)
	if err != nil {
		t.Fatalf("Failed to marshal update request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/update-pipeline/"+testPipelineID, bytes.NewReader(body))
	w := httptest.NewRecorder()

	UpdatePipeline(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify old file was deleted
	if _, err := os.Stat(oldFilePath); !os.IsNotExist(err) {
		t.Error("Old pipeline file still exists after name change")
	}

	// Verify new file was created
	newFileName := sanitizePipelineName(newName) + ".sh"
	newFilePath := filepath.Join(pipelineDir, newFileName)
	if _, err := os.Stat(newFilePath); os.IsNotExist(err) {
		t.Error("New pipeline file was not created after name change")
	}
}

func TestUpdatePipelineNotFound(t *testing.T) {
	// Setup: Create a temporary directory
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	updateReq := UpdatePipelineRequest{
		Name:        "test",
		Description: "test",
		Code:        "#!/bin/bash\necho 'test'",
		Branch:      "main",
	}

	body, err := json.Marshal(updateReq)
	if err != nil {
		t.Fatalf("Failed to marshal update request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/update-pipeline/nonexistent", bytes.NewReader(body))
	w := httptest.NewRecorder()

	UpdatePipeline(w, req)

	// Should return 404 for nonexistent pipeline
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}
}
