package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestDeletePipeline(t *testing.T) {
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
	testPipelineID := "test-delete-123"
	testName := "test-delete-pipeline"
	testCode := "#!/bin/bash\necho 'test'"
	createdAt := time.Now().Format(time.RFC3339)

	// Write the pipeline file
	pipelineFileName := sanitizePipelineName(testName) + ".sh"
	pipelinePath := filepath.Join(pipelineDir, pipelineFileName)
	if err := os.WriteFile(pipelinePath, []byte(testCode), 0755); err != nil {
		t.Fatalf("Failed to create test pipeline file: %v", err)
	}

	// Add to service ledger
	if err := service_ledger.UpdatePipelineEntry(testPipelineID, testName, "", testCode, "main", "idle", createdAt); err != nil {
		t.Fatalf("Failed to create test pipeline entry: %v", err)
	}

	// Delete the pipeline
	req := httptest.NewRequest(http.MethodDelete, "/delete-pipeline/"+testPipelineID, nil)
	w := httptest.NewRecorder()

	DeletePipeline(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify the file was deleted
	if _, err := os.Stat(pipelinePath); !os.IsNotExist(err) {
		t.Error("Pipeline file still exists after deletion")
	}

	// Verify it was removed from ledger
	ledgerEntry, err := service_ledger.GetPipelineEntry(testPipelineID)
	if err != nil {
		t.Fatalf("Failed to check pipeline entry: %v", err)
	}
	if ledgerEntry != nil {
		t.Error("Pipeline still exists in ledger after deletion")
	}
}

func TestRunPipeline(t *testing.T) {
	// Setup: Create a temporary directory for test pipelines
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	pipelineDir := filepath.Join(tmpHome, ".opencloud", "pipelines")
	if err := os.MkdirAll(pipelineDir, 0755); err != nil {
		t.Fatalf("Failed to create test pipeline directory: %v", err)
	}

	// Create a test pipeline with a simple echo command
	testPipelineID := "test-run-123"
	testName := "test-run-pipeline"
	testCode := "#!/bin/bash\necho 'Hello from pipeline'"
	createdAt := time.Now().Format(time.RFC3339)

	// Write the pipeline file
	pipelineFileName := sanitizePipelineName(testName) + ".sh"
	pipelinePath := filepath.Join(pipelineDir, pipelineFileName)
	if err := os.WriteFile(pipelinePath, []byte(testCode), 0755); err != nil {
		t.Fatalf("Failed to create test pipeline file: %v", err)
	}

	// Add to service ledger
	if err := service_ledger.UpdatePipelineEntry(testPipelineID, testName, "", testCode, "main", "idle", createdAt); err != nil {
		t.Fatalf("Failed to create test pipeline entry: %v", err)
	}

	// Run the pipeline
	req := httptest.NewRequest(http.MethodPost, "/run-pipeline/"+testPipelineID, nil)
	w := httptest.NewRecorder()

	RunPipeline(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify the response contains success message
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "running" {
		t.Errorf("Expected status 'running', got '%s'", response["status"])
	}

	// Wait a moment for the goroutine to complete
	time.Sleep(500 * time.Millisecond)

	// Verify log file was created
	logDir := filepath.Join(tmpHome, ".opencloud", "logs", "pipelines")
	logFileName := sanitizePipelineName(testName) + ".log"
	logFilePath := filepath.Join(logDir, logFileName)

	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		t.Error("Log file was not created")
	} else {
		// Read and verify log content
		logContent, err := os.ReadFile(logFilePath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		logStr := string(logContent)
		if !strings.Contains(logStr, "Hello from pipeline") {
			t.Errorf("Log file does not contain expected output. Got: %s", logStr)
		}
	}
}

func TestGetPipelineLogs(t *testing.T) {
	// Setup: Create a temporary directory for test pipelines
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create test pipeline entry
	testPipelineID := "test-logs-123"
	testName := "test-logs-pipeline"
	createdAt := time.Now().Format(time.RFC3339)

	if err := service_ledger.UpdatePipelineEntry(testPipelineID, testName, "", "echo test", "main", "idle", createdAt); err != nil {
		t.Fatalf("Failed to create test pipeline entry: %v", err)
	}

	// Create log directory and file
	logDir := filepath.Join(tmpHome, ".opencloud", "logs", "pipelines")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	logFileName := sanitizePipelineName(testName) + ".log"
	logFilePath := filepath.Join(logDir, logFileName)

	// Write test log entries
	timestamp1 := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	timestamp2 := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	logContent := fmt.Sprintf(
		"===EXECUTION_START:%s|SUCCESS===\nFirst execution output\n===EXECUTION_END===\n"+
			"===EXECUTION_START:%s|ERROR===\nSecond execution failed\n===EXECUTION_END===\n",
		timestamp1, timestamp2)

	if err := os.WriteFile(logFilePath, []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to write log file: %v", err)
	}

	// Get pipeline logs
	req := httptest.NewRequest(http.MethodGet, "/get-pipeline-logs/"+testPipelineID, nil)
	w := httptest.NewRecorder()

	GetPipelineLogs(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify the response contains logs
	var logs []PipelineLog
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(logs) != 2 {
		t.Errorf("Expected 2 log entries, got %d", len(logs))
	}

	// Verify first log entry
	if len(logs) > 0 {
		if logs[0].Status != "success" {
			t.Errorf("Expected first log status 'success', got '%s'", logs[0].Status)
		}
		if logs[0].Output != "First execution output" {
			t.Errorf("Expected first log output 'First execution output', got '%s'", logs[0].Output)
		}
	}

	// Verify second log entry
	if len(logs) > 1 {
		if logs[1].Status != "error" {
			t.Errorf("Expected second log status 'error', got '%s'", logs[1].Status)
		}
		if logs[1].Error != "Second execution failed" {
			t.Errorf("Expected second log error 'Second execution failed', got '%s'", logs[1].Error)
		}
	}
}

func TestGetPipelineLogsEmpty(t *testing.T) {
	// Setup: Create a temporary directory
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create test pipeline entry
	testPipelineID := "test-empty-logs-123"
	testName := "test-empty-logs-pipeline"
	createdAt := time.Now().Format(time.RFC3339)

	if err := service_ledger.UpdatePipelineEntry(testPipelineID, testName, "", "echo test", "main", "idle", createdAt); err != nil {
		t.Fatalf("Failed to create test pipeline entry: %v", err)
	}

	// Get pipeline logs (no log file exists)
	req := httptest.NewRequest(http.MethodGet, "/get-pipeline-logs/"+testPipelineID, nil)
	w := httptest.NewRecorder()

	GetPipelineLogs(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify the response contains empty array
	var logs []PipelineLog
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(logs) != 0 {
		t.Errorf("Expected 0 log entries for non-existent log file, got %d", len(logs))
	}
}
