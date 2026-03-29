package service_ledger

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// getInstallerDir is a helper function that returns the path to the service_installers directory.
// This is used by multiple test functions to avoid code duplication.
func getInstallerDir(t *testing.T) string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get current file path")
	}
	dir := filepath.Dir(currentFile)
	return filepath.Join(dir, "service_installers")
}

// escapeSingleQuoteForBash escapes single quotes in a string for safe use within
// single-quoted bash strings. It uses the '\'' pattern: end the single-quoted string,
// add an escaped single quote, then start a new single-quoted string. This is the
// standard bash approach for including literal single quotes within single-quoted strings.
// Example: "can't" becomes "can'\''t" which bash interprets as: can + ' + t
func escapeSingleQuoteForBash(s string) string {
	// Replace each ' with '\'' (end quote, escaped quote, start quote)
	return strings.ReplaceAll(s, "'", "'\\''")
}

// createTestScript generates test installer scripts with configurable exit codes and messages.
// This reduces code duplication across test functions that need to create test scripts.
//
// Parameters:
//   - installerDir: The directory where the script should be created
//   - serviceName: The name of the service (used to name the script file)
//   - exitCode: The exit code the script should return (0 for success, non-zero for failure)
//   - message: The message the script should echo (will be escaped for use in single-quoted bash strings)
//
// Returns:
//   - scriptPath: The full path to the created script
//   - error: Any error encountered during script creation
//
// Note: This function is designed for test purposes where messages are controlled and trusted.
// The escaping handles single-quote contexts specifically. If using similar logic for user-provided
// input in production code, additional validation and escaping would be needed for full shell safety.
func createTestScript(installerDir, serviceName string, exitCode int, message string) (string, error) {
	scriptPath := filepath.Join(installerDir, serviceName+".sh")
	// Escape single quotes for use within single-quoted bash strings.
	// When a string is enclosed in single quotes in bash, all characters are treated
	// literally (no variable expansion, no command substitution, no globbing) except for
	// single quotes themselves, making this escaping sufficient for single-quote contexts.
	escapedMessage := escapeSingleQuoteForBash(message)
	scriptContent := fmt.Sprintf("#!/bin/bash\necho '%s'\nexit %d\n", escapedMessage, exitCode)
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	return scriptPath, err
}

func TestSyncFunctionsBasic(t *testing.T) {
	// Setup: Create a temporary directory for test functions
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	functionDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(functionDir, 0755); err != nil {
		t.Fatalf("Failed to create test function directory: %v", err)
	}

	// Create test function files
	testFunctions := map[string]string{
		"test_hello.py":     "def handler(event):\n    return 'Hello World'",
		"test_process.js":   "module.exports.handler = async (event) => { return 'OK'; }",
		"test_calculate.go": "package main\nfunc main() { println('hello') }",
	}

	for name, content := range testFunctions {
		fnPath := filepath.Join(functionDir, name)
		if err := os.WriteFile(fnPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test function %s: %v", name, err)
		}
	}

	// Call SyncFunctions
	if err := SyncFunctions(); err != nil {
		t.Fatalf("SyncFunctions failed: %v", err)
	}

	// Verify that all functions were added to the ledger
	ledger, err := ReadServiceLedger()
	if err != nil {
		t.Fatalf("Failed to read service ledger: %v", err)
	}

	status, exists := ledger["Functions"]
	if !exists {
		t.Fatal("Functions service not found in ledger")
	}

	// Verify each test function has the correct content and runtime
	expectedRuntimes := map[string]string{
		"test_hello.py":     "python",
		"test_process.js":   "nodejs",
		"test_calculate.go": "go",
	}

	for name, expectedContent := range testFunctions {
		entry, exists := status.Functions[name]
		if !exists {
			t.Errorf("Function %s not found in ledger", name)
			continue
		}

		if entry.Content != expectedContent {
			t.Errorf("Function %s content mismatch. Expected %q, got %q", name, expectedContent, entry.Content)
		}

		expectedRuntime := expectedRuntimes[name]
		if entry.Runtime != expectedRuntime {
			t.Errorf("Function %s runtime mismatch. Expected %s, got %s", name, expectedRuntime, entry.Runtime)
		}

		if len(entry.Logs) != 0 {
			t.Errorf("Function %s should have empty logs, got %d entries", name, len(entry.Logs))
		}
	}
}

func TestSyncFunctionsContentUpdate(t *testing.T) {
	// Setup: Create a temporary directory for test functions
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	functionDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(functionDir, 0755); err != nil {
		t.Fatalf("Failed to create test function directory: %v", err)
	}

	// Create a test function file
	fnName := "test_update.py"
	fnContent := "def handler(event):\n    return 'Version 1'"
	fnPath := filepath.Join(functionDir, fnName)
	if err := os.WriteFile(fnPath, []byte(fnContent), 0644); err != nil {
		t.Fatalf("Failed to create test function: %v", err)
	}

	// Add function to ledger with trigger and schedule
	if err := UpdateFunctionEntry(fnName, "python", "cron", "0 0 * * *", fnContent); err != nil {
		t.Fatalf("Failed to create function entry: %v", err)
	}

	// Add a log entry
	ledger, _ := ReadServiceLedger()
	status := ledger["Functions"]
	if status.Functions == nil {
		status.Functions = make(map[string]FunctionEntry)
	}
	entry := status.Functions[fnName]
	entry.Logs = []FunctionLog{
		{
			Timestamp: "2024-01-01T00:00:00Z",
			Output:    "Test log",
			Status:    "success",
		},
	}
	status.Functions[fnName] = entry
	ledger["Functions"] = status
	WriteServiceLedger(ledger)

	// Update the file content
	newContent := "def handler(event):\n    return 'Version 2'"
	if err := os.WriteFile(fnPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("Failed to update test function: %v", err)
	}

	// Call SyncFunctions
	if err := SyncFunctions(); err != nil {
		t.Fatalf("SyncFunctions failed: %v", err)
	}

	// Verify that content was updated but trigger, schedule, and logs were preserved
	ledger, err := ReadServiceLedger()
	if err != nil {
		t.Fatalf("Failed to read service ledger: %v", err)
	}

	syncedEntry := ledger["Functions"].Functions[fnName]

	if syncedEntry.Content != newContent {
		t.Errorf("Expected content to be updated to %q, got %q", newContent, syncedEntry.Content)
	}

	if syncedEntry.Trigger != "cron" {
		t.Errorf("Expected trigger 'cron', got %s", syncedEntry.Trigger)
	}

	if syncedEntry.Schedule != "0 0 * * *" {
		t.Errorf("Expected schedule '0 0 * * *', got %s", syncedEntry.Schedule)
	}

	if len(syncedEntry.Logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(syncedEntry.Logs))
	}
}

func TestSyncFunctionsEmptyDirectory(t *testing.T) {
	// Setup: Create a temporary directory for test functions
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	functionDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(functionDir, 0755); err != nil {
		t.Fatalf("Failed to create test function directory: %v", err)
	}

	// Call SyncFunctions with empty directory - should not error
	if err := SyncFunctions(); err != nil {
		t.Fatalf("SyncFunctions failed: %v", err)
	}

	// Just verify that the ledger can be read without error
	_, err := ReadServiceLedger()
	if err != nil {
		t.Fatalf("Failed to read service ledger: %v", err)
	}
}

func TestSyncFunctionsNoDirectory(t *testing.T) {
	// Setup: Create a temporary home without functions directory
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Call SyncFunctions without creating the directory
	if err := SyncFunctions(); err != nil {
		t.Fatalf("SyncFunctions should not fail when directory doesn't exist: %v", err)
	}

	// Verify that the ledger is not corrupted
	_, err := ReadServiceLedger()
	if err != nil {
		t.Errorf("Service ledger should be readable: %v", err)
	}
}

func TestExecuteServiceInstallerNonExistent(t *testing.T) {
	// Test with a service that doesn't have an installer script
	// This should not fail - it should return nil
	err := executeServiceInstaller("nonexistent_service")
	if err != nil {
		t.Errorf("executeServiceInstaller should not fail for non-existent installer: %v", err)
	}
}

func TestExecuteServiceInstallerSuccess(t *testing.T) {
	// Get the actual service_installers directory path
	installerDir := getInstallerDir(t)
	
	// Create a test service installer that will succeed
	testServiceName := "test_service_success"
	installerPath, err := createTestScript(installerDir, testServiceName, 0, "Test installer executed successfully")
	if err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath) // Clean up after test
	
	// Execute the installer
	err = executeServiceInstaller(testServiceName)
	if err != nil {
		t.Errorf("executeServiceInstaller should succeed for valid installer: %v", err)
	}
}

func TestExecuteServiceInstallerFailure(t *testing.T) {
	// Get the actual service_installers directory path
	installerDir := getInstallerDir(t)
	
	// Create a test service installer that will fail
	testServiceName := "test_service_failure"
	installerPath, err := createTestScript(installerDir, testServiceName, 1, "Test installer failed")
	if err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath) // Clean up after test
	
	// Execute the installer - should fail
	err = executeServiceInstaller(testServiceName)
	if err == nil {
		t.Error("executeServiceInstaller should fail for failing installer script")
	}
}

func TestEnableServiceWithInstaller(t *testing.T) {
	// Setup: Create a temporary ledger
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	// Initialize the ledger
	if err := InitializeServiceLedger(); err != nil {
		t.Fatalf("Failed to initialize ledger: %v", err)
	}
	
	// Get the actual service_installers directory path
	installerDir := getInstallerDir(t)
	
	// Create a test service installer
	testServiceName := "test_enable_service"
	installerPath, err := createTestScript(installerDir, testServiceName, 0, "Installing test service")
	if err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath)
	
	// Enable the service
	err = EnableService(testServiceName)
	if err != nil {
		t.Errorf("EnableService should succeed when installer succeeds: %v", err)
	}
	
	// Verify the service is enabled in the ledger
	enabled, err := IsServiceEnabled(testServiceName)
	if err != nil {
		t.Fatalf("Failed to check service status: %v", err)
	}
	
	if !enabled {
		t.Error("Service should be enabled after successful EnableService call")
	}
}

func TestEnableServiceWithFailingInstaller(t *testing.T) {
	// Setup: Create a temporary ledger
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	// Initialize the ledger
	if err := InitializeServiceLedger(); err != nil {
		t.Fatalf("Failed to initialize ledger: %v", err)
	}
	
	// Get the actual service_installers directory path
	installerDir := getInstallerDir(t)
	
	// Create a test service installer that fails
	testServiceName := "test_enable_fail"
	installerPath, err := createTestScript(installerDir, testServiceName, 1, "Installation failed")
	if err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath)
	
	// Enable the service - should fail
	err = EnableService(testServiceName)
	if err == nil {
		t.Error("EnableService should fail when installer fails")
	}
	
	// Verify the service is NOT enabled in the ledger
	enabled, err := IsServiceEnabled(testServiceName)
	if err != nil {
		t.Fatalf("Failed to check service status: %v", err)
	}
	
	if enabled {
		t.Error("Service should NOT be enabled when installer fails")
	}
}

func TestEnableServiceWithoutInstaller(t *testing.T) {
	// Setup: Create a temporary ledger
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	// Initialize the ledger
	if err := InitializeServiceLedger(); err != nil {
		t.Fatalf("Failed to initialize ledger: %v", err)
	}
	
	// Enable a service without an installer - should succeed
	testServiceName := "service_without_installer"
	err := EnableService(testServiceName)
	if err != nil {
		t.Errorf("EnableService should succeed even without installer: %v", err)
	}
	
	// Verify the service is enabled
	enabled, err := IsServiceEnabled(testServiceName)
	if err != nil {
		t.Fatalf("Failed to check service status: %v", err)
	}
	
	if !enabled {
		t.Error("Service should be enabled even without installer")
	}
}

func TestEnableServiceWithStreamSuccess(t *testing.T) {
	// Setup: Create a temporary ledger
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	if err := InitializeServiceLedger(); err != nil {
		t.Fatalf("Failed to initialize ledger: %v", err)
	}

	// Create an installer script that emits multiple output lines
	installerDir := getInstallerDir(t)
	testServiceName := "test_stream_success"
	installerPath, err := createTestScript(installerDir, testServiceName, 0, "Stream line one")
	if err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath)

	// Invoke the stream handler via an httptest recorder
	body := strings.NewReader(`{"service":"` + testServiceName + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/enable-service-stream", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	EnableServiceStreamHandler(rr, req)

	resp := rr.Result()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %q", resp.Header.Get("Content-Type"))
	}

	responseBody := rr.Body.String()

	// The SSE response should contain the installer output line
	if !strings.Contains(responseBody, "Stream line one") {
		t.Errorf("Response should contain installer output, got: %s", responseBody)
	}

	// The SSE response should contain the done event
	if !strings.Contains(responseBody, "event: done") {
		t.Errorf("Response should contain done event, got: %s", responseBody)
	}

	// Verify the service was actually enabled in the ledger
	enabled, err := IsServiceEnabled(testServiceName)
	if err != nil {
		t.Fatalf("Failed to check service status: %v", err)
	}
	if !enabled {
		t.Error("Service should be enabled after streaming enable completes successfully")
	}
}

func TestEnableServiceWithStreamFailingInstaller(t *testing.T) {
	// Setup: Create a temporary ledger
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	if err := InitializeServiceLedger(); err != nil {
		t.Fatalf("Failed to initialize ledger: %v", err)
	}

	// Create an installer script that exits with non-zero exit code
	installerDir := getInstallerDir(t)
	testServiceName := "test_stream_fail"
	installerPath, err := createTestScript(installerDir, testServiceName, 1, "Stream install failed")
	if err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath)

	body := strings.NewReader(`{"service":"` + testServiceName + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/enable-service-stream", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	EnableServiceStreamHandler(rr, req)

	responseBody := rr.Body.String()

	// The SSE response should contain the error event
	if !strings.Contains(responseBody, "event: error") {
		t.Errorf("Response should contain error event for failing installer, got: %s", responseBody)
	}

	// The service should NOT be marked as enabled
	enabled, err := IsServiceEnabled(testServiceName)
	if err != nil {
		t.Fatalf("Failed to check service status: %v", err)
	}
	if enabled {
		t.Error("Service should NOT be enabled when installer fails")
	}
}

func TestEnableServiceWithStreamNoInstaller(t *testing.T) {
	// Setup: Create a temporary ledger
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	if err := InitializeServiceLedger(); err != nil {
		t.Fatalf("Failed to initialize ledger: %v", err)
	}

	// Use a service name that has no installer script
	testServiceName := "stream_service_no_installer"

	body := strings.NewReader(`{"service":"` + testServiceName + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/enable-service-stream", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	EnableServiceStreamHandler(rr, req)

	responseBody := rr.Body.String()

	// Should still send the done event since no installer means success
	if !strings.Contains(responseBody, "event: done") {
		t.Errorf("Response should contain done event for service without installer, got: %s", responseBody)
	}

	// The service should be enabled
	enabled, err := IsServiceEnabled(testServiceName)
	if err != nil {
		t.Fatalf("Failed to check service status: %v", err)
	}
	if !enabled {
		t.Error("Service should be enabled when no installer is present")
	}
}

func TestEnableServiceStreamHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/enable-service-stream", nil)
	rr := httptest.NewRecorder()

	EnableServiceStreamHandler(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", rr.Code)
	}
}

func TestEnableServiceStreamHandlerMissingService(t *testing.T) {
	body := strings.NewReader(`{"service":""}`)
	req := httptest.NewRequest(http.MethodPost, "/enable-service-stream", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	EnableServiceStreamHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing service, got %d", rr.Code)
	}
}

// resetContainerImages removes all container image entries from the shared service ledger.
// This is used by tests that rely on an exact count or clean state, since the ledger file
// is stored in the source tree and persists across test runs.
func resetContainerImages(t *testing.T) {
	t.Helper()
	ledgerMutex.Lock()
	defer ledgerMutex.Unlock()
	ledger, err := ReadServiceLedger()
	if err != nil {
		t.Fatalf("resetContainerImages: failed to read ledger: %v", err)
	}
	if status, exists := ledger["container_registry"]; exists {
		status.ContainerImages = make(map[string]ContainerImageEntry)
		ledger["container_registry"] = status
	}
	if err := WriteServiceLedger(ledger); err != nil {
		t.Fatalf("resetContainerImages: failed to write ledger: %v", err)
	}
}

// TestUpdateContainerImageEntry tests that a container image entry is stored in the ledger
func TestUpdateContainerImageEntry(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	resetContainerImages(t)
	t.Cleanup(func() { resetContainerImages(t) })

	imageName := "my-app:latest"
	dockerfile := "FROM alpine:latest\nRUN echo hello"
	context := "."
	platform := "linux/amd64"
	noCache := false
	builtAt := "2024-01-01T00:00:00Z"

	if err := UpdateContainerImageEntry(imageName, dockerfile, context, platform, noCache, builtAt); err != nil {
		t.Fatalf("UpdateContainerImageEntry failed: %v", err)
	}

	entry, err := GetContainerImageEntry(imageName)
	if err != nil {
		t.Fatalf("GetContainerImageEntry failed: %v", err)
	}
	if entry == nil {
		t.Fatal("Expected container image entry, got nil")
	}

	if entry.ImageName != imageName {
		t.Errorf("Expected imageName %q, got %q", imageName, entry.ImageName)
	}
	if entry.Dockerfile != dockerfile {
		t.Errorf("Expected dockerfile %q, got %q", dockerfile, entry.Dockerfile)
	}
	if entry.Context != context {
		t.Errorf("Expected context %q, got %q", context, entry.Context)
	}
	if entry.Platform != platform {
		t.Errorf("Expected platform %q, got %q", platform, entry.Platform)
	}
	if entry.NoCache != noCache {
		t.Errorf("Expected noCache %v, got %v", noCache, entry.NoCache)
	}
	if entry.BuiltAt != builtAt {
		t.Errorf("Expected builtAt %q, got %q", builtAt, entry.BuiltAt)
	}
}

// TestUpdateContainerImageEntryOverwrite tests that updating an existing image overwrites its fields
func TestUpdateContainerImageEntryOverwrite(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	resetContainerImages(t)
	t.Cleanup(func() { resetContainerImages(t) })

	imageName := "my-app:v1"
	firstDockerfile := "FROM alpine:latest"
	secondDockerfile := "FROM ubuntu:22.04\nRUN apt-get update"

	if err := UpdateContainerImageEntry(imageName, firstDockerfile, ".", "", false, "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("First UpdateContainerImageEntry failed: %v", err)
	}
	if err := UpdateContainerImageEntry(imageName, secondDockerfile, ".", "linux/arm64", true, "2024-06-01T00:00:00Z"); err != nil {
		t.Fatalf("Second UpdateContainerImageEntry failed: %v", err)
	}

	entry, err := GetContainerImageEntry(imageName)
	if err != nil {
		t.Fatalf("GetContainerImageEntry failed: %v", err)
	}
	if entry == nil {
		t.Fatal("Expected container image entry, got nil")
	}

	if entry.Dockerfile != secondDockerfile {
		t.Errorf("Expected updated dockerfile %q, got %q", secondDockerfile, entry.Dockerfile)
	}
	if entry.Platform != "linux/arm64" {
		t.Errorf("Expected updated platform %q, got %q", "linux/arm64", entry.Platform)
	}
	if !entry.NoCache {
		t.Error("Expected noCache to be true after update")
	}
}

// TestDeleteContainerImageEntry tests that a container image entry can be removed from the ledger
func TestDeleteContainerImageEntry(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	resetContainerImages(t)
	t.Cleanup(func() { resetContainerImages(t) })

	imageName := "to-delete:latest"
	if err := UpdateContainerImageEntry(imageName, "FROM alpine:latest", ".", "", false, "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("UpdateContainerImageEntry failed: %v", err)
	}

	if err := DeleteContainerImageEntry(imageName); err != nil {
		t.Fatalf("DeleteContainerImageEntry failed: %v", err)
	}

	entry, err := GetContainerImageEntry(imageName)
	if err != nil {
		t.Fatalf("GetContainerImageEntry after delete failed: %v", err)
	}
	if entry != nil {
		t.Error("Expected nil entry after deletion, but got one")
	}
}

// TestDeleteContainerImageEntryNonExistent tests that deleting a non-existent entry is a no-op
func TestDeleteContainerImageEntryNonExistent(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Should not return an error for a non-existent image
	if err := DeleteContainerImageEntry("does-not-exist:latest"); err != nil {
		t.Errorf("DeleteContainerImageEntry should not fail for non-existent entry: %v", err)
	}
}

// TestGetContainerImageEntryNonExistent tests that getting a non-existent entry returns nil
func TestGetContainerImageEntryNonExistent(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	entry, err := GetContainerImageEntry("not-there:latest")
	if err != nil {
		t.Fatalf("GetContainerImageEntry returned unexpected error: %v", err)
	}
	if entry != nil {
		t.Error("Expected nil for non-existent entry, got non-nil")
	}
}

// TestGetAllContainerImageEntries tests that all stored images are returned
func TestGetAllContainerImageEntries(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	resetContainerImages(t)
	t.Cleanup(func() { resetContainerImages(t) })

	images := map[string]string{
		"app-a:latest": "FROM alpine:latest\nRUN echo a",
		"app-b:v2":     "FROM ubuntu:22.04\nRUN echo b",
	}
	for name, df := range images {
		if err := UpdateContainerImageEntry(name, df, ".", "", false, "2024-01-01T00:00:00Z"); err != nil {
			t.Fatalf("UpdateContainerImageEntry(%s) failed: %v", name, err)
		}
	}

	all, err := GetAllContainerImageEntries()
	if err != nil {
		t.Fatalf("GetAllContainerImageEntries failed: %v", err)
	}

	if len(all) != len(images) {
		t.Errorf("Expected %d entries, got %d", len(images), len(all))
	}
	for name, expectedDF := range images {
		entry, ok := all[name]
		if !ok {
			t.Errorf("Expected entry for %q not found", name)
			continue
		}
		if entry.Dockerfile != expectedDF {
			t.Errorf("Entry %q: expected dockerfile %q, got %q", name, expectedDF, entry.Dockerfile)
		}
	}
}

// TestGetAllContainerImageEntriesEmpty tests that an empty map is returned when no images are stored
func TestGetAllContainerImageEntriesEmpty(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	resetContainerImages(t)
	t.Cleanup(func() { resetContainerImages(t) })

	all, err := GetAllContainerImageEntries()
	if err != nil {
		t.Fatalf("GetAllContainerImageEntries failed: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(all))
	}
}

