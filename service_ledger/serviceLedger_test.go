package service_ledger

import (
	"os"
	"path/filepath"
	"testing"
)

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

