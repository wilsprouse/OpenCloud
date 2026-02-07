package service_ledger

import (
	"os"
	"path/filepath"
	"runtime"
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
	installerPath := filepath.Join(installerDir, testServiceName+".sh")
	
	// Create the installer script
	scriptContent := "#!/bin/bash\necho 'Test installer executed successfully'\nexit 0\n"
	if err := os.WriteFile(installerPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath) // Clean up after test
	
	// Execute the installer
	err := executeServiceInstaller(testServiceName)
	if err != nil {
		t.Errorf("executeServiceInstaller should succeed for valid installer: %v", err)
	}
}

func TestExecuteServiceInstallerFailure(t *testing.T) {
	// Get the actual service_installers directory path
	installerDir := getInstallerDir(t)
	
	// Create a test service installer that will fail
	testServiceName := "test_service_failure"
	installerPath := filepath.Join(installerDir, testServiceName+".sh")
	
	// Create the installer script that exits with error
	scriptContent := "#!/bin/bash\necho 'Test installer failed'\nexit 1\n"
	if err := os.WriteFile(installerPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath) // Clean up after test
	
	// Execute the installer - should fail
	err := executeServiceInstaller(testServiceName)
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
	installerPath := filepath.Join(installerDir, testServiceName+".sh")
	
	scriptContent := "#!/bin/bash\necho 'Installing test service'\nexit 0\n"
	if err := os.WriteFile(installerPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath)
	
	// Enable the service
	err := EnableService(testServiceName)
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
	installerPath := filepath.Join(installerDir, testServiceName+".sh")
	
	scriptContent := "#!/bin/bash\necho 'Installation failed'\nexit 1\n"
	if err := os.WriteFile(installerPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test installer: %v", err)
	}
	defer os.Remove(installerPath)
	
	// Enable the service - should fail
	err := EnableService(testServiceName)
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

