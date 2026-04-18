package compute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WavexSoftware/OpenCloud/service_ledger"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/domain/entities/reports"
	podmanTypes "github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
)

// Helper function to save and restore crontab state for tests
func setupCrontabTest(t *testing.T) (cleanup func()) {
	// Save original crontab
	origCrontabCmd := exec.Command("crontab", "-l")
	origCrontabOutput, _ := origCrontabCmd.CombinedOutput()
	origCrontab := string(origCrontabOutput)

	// Clear crontab for test
	cmd := exec.Command("crontab", "-r")
	if err := cmd.Run(); err != nil {
		// Ignore error if crontab doesn't exist
		if !strings.Contains(err.Error(), "no crontab") {
			t.Logf("Warning: Failed to clear crontab: %v", err)
		}
	}

	// Return cleanup function
	return func() {
		// Restore original crontab
		if strings.Contains(origCrontab, "no crontab for") || origCrontab == "" {
			// Clear crontab
			cmd := exec.Command("crontab", "-r")
			if err := cmd.Run(); err != nil {
				// Ignore error if crontab doesn't exist
				if !strings.Contains(err.Error(), "no crontab") {
					t.Logf("Warning: Failed to clear crontab during cleanup: %v", err)
				}
			}
		} else {
			cmd := exec.Command("crontab", "-")
			cmd.Stdin = strings.NewReader(origCrontab)
			if err := cmd.Run(); err != nil {
				t.Logf("Warning: Failed to restore crontab: %v", err)
			}
		}
	}
}

func TestAddCron(t *testing.T) {
	// Skip test if crontab is not available
	if _, err := exec.LookPath("crontab"); err != nil {
		t.Skip("crontab command not available")
	}

	// Setup: Create a temporary directory and function file
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	funcDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatalf("Failed to create test functions directory: %v", err)
	}

	// Create a test function file
	testFuncPath := filepath.Join(funcDir, "test_function.py")
	if err := os.WriteFile(testFuncPath, []byte("print('test')"), 0755); err != nil {
		t.Fatalf("Failed to create test function file: %v", err)
	}

	// Setup and cleanup crontab
	cleanup := setupCrontabTest(t)
	defer cleanup()

	// Test adding a cron job
	testSchedule := "0 0 * * *"
	err := addCron(testFuncPath, testSchedule)
	if err != nil {
		t.Fatalf("addCron failed: %v", err)
	}

	// Verify the cron job was added
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to read crontab: %v", err)
	}

	crontabContent := string(output)
	expectedLogFile := filepath.Join(tmpHome, ".opencloud", "logs", "functions", "cron_test_function.py.log")

	// Check that the cron job contains the function-specific log file
	if !strings.Contains(crontabContent, expectedLogFile) {
		t.Errorf("Crontab does not contain expected log file path.\nExpected: %s\nGot: %s", expectedLogFile, crontabContent)
	}

	// Check that the old generic log file name is NOT present
	if strings.Contains(crontabContent, "go_cron_output.log") {
		t.Error("Crontab still contains old generic log file name 'go_cron_output.log'")
	}

	// Verify the cron job contains the schedule and function path
	if !strings.Contains(crontabContent, testSchedule) {
		t.Errorf("Crontab does not contain expected schedule: %s", testSchedule)
	}
	if !strings.Contains(crontabContent, testFuncPath) {
		t.Errorf("Crontab does not contain expected function path: %s", testFuncPath)
	}

	// Verify logs directory was created
	logsDir := filepath.Join(tmpHome, ".opencloud", "logs")
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		t.Error("Logs directory was not created")
	}
}

func TestAddCronDuplicatePrevention(t *testing.T) {
	// Skip test if crontab is not available
	if _, err := exec.LookPath("crontab"); err != nil {
		t.Skip("crontab command not available")
	}

	// Setup: Create a temporary directory and function file
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	funcDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatalf("Failed to create test functions directory: %v", err)
	}

	// Create a test function file
	testFuncPath := filepath.Join(funcDir, "duplicate_test.py")
	if err := os.WriteFile(testFuncPath, []byte("print('test')"), 0755); err != nil {
		t.Fatalf("Failed to create test function file: %v", err)
	}

	// Setup and cleanup crontab
	cleanup := setupCrontabTest(t)
	defer cleanup()

	// Add the same cron job twice
	testSchedule := "0 0 * * *"
	err := addCron(testFuncPath, testSchedule)
	if err != nil {
		t.Fatalf("First addCron failed: %v", err)
	}

	err = addCron(testFuncPath, testSchedule)
	if err != nil {
		t.Fatalf("Second addCron failed: %v", err)
	}

	// Verify only one entry exists
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to read crontab: %v", err)
	}

	crontabContent := string(output)
	lines := strings.Split(strings.TrimSpace(crontabContent), "\n")

	// Filter out empty lines
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	if nonEmptyLines != 1 {
		t.Errorf("Expected exactly 1 cron job entry, got %d. Crontab content:\n%s", nonEmptyLines, crontabContent)
	}
}

func TestAddCronMultipleFunctions(t *testing.T) {
	// Skip test if crontab is not available
	if _, err := exec.LookPath("crontab"); err != nil {
		t.Skip("crontab command not available")
	}

	// Setup: Create a temporary directory and function files
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	funcDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatalf("Failed to create test functions directory: %v", err)
	}

	// Create multiple test function files
	functions := []struct {
		name     string
		schedule string
	}{
		{"backup.py", "0 0 * * *"},
		{"sync.js", "0 * * * *"},
		{"cleanup.go", "0 0 * * 0"},
	}

	for _, fn := range functions {
		testFuncPath := filepath.Join(funcDir, fn.name)
		if err := os.WriteFile(testFuncPath, []byte("test"), 0755); err != nil {
			t.Fatalf("Failed to create test function file %s: %v", fn.name, err)
		}
	}

	// Setup and cleanup crontab
	cleanup := setupCrontabTest(t)
	defer cleanup()

	// Add all cron jobs
	for _, fn := range functions {
		testFuncPath := filepath.Join(funcDir, fn.name)
		err := addCron(testFuncPath, fn.schedule)
		if err != nil {
			t.Fatalf("addCron failed for %s: %v", fn.name, err)
		}
	}

	// Verify all cron jobs were added with unique log files
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to read crontab: %v", err)
	}

	crontabContent := string(output)

	// Verify each function has its own log file
	for _, fn := range functions {
		expectedLogFile := filepath.Join(tmpHome, ".opencloud", "logs", "functions", "cron_"+fn.name+".log")
		if !strings.Contains(crontabContent, expectedLogFile) {
			t.Errorf("Crontab does not contain expected log file for %s.\nExpected: %s\nCrontab:\n%s", fn.name, expectedLogFile, crontabContent)
		}
	}

	// Verify no generic log file is present
	if strings.Contains(crontabContent, "go_cron_output.log") {
		t.Error("Crontab contains old generic log file name 'go_cron_output.log'")
	}
}

func TestRemoveCron(t *testing.T) {
	// Skip test if crontab is not available
	if _, err := exec.LookPath("crontab"); err != nil {
		t.Skip("crontab command not available")
	}

	// Setup: Create a temporary directory and function file
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	funcDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatalf("Failed to create test functions directory: %v", err)
	}

	// Create a test function file
	testFuncPath := filepath.Join(funcDir, "test_function.py")
	if err := os.WriteFile(testFuncPath, []byte("print('test')"), 0755); err != nil {
		t.Fatalf("Failed to create test function file: %v", err)
	}

	// Setup and cleanup crontab
	cleanup := setupCrontabTest(t)
	defer cleanup()

	// First add a cron job
	testSchedule := "0 0 * * *"
	err := addCron(testFuncPath, testSchedule)
	if err != nil {
		t.Fatalf("addCron failed: %v", err)
	}

	// Verify the cron job was added
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to read crontab: %v", err)
	}
	if !strings.Contains(string(output), testFuncPath) {
		t.Fatal("Cron job was not added successfully")
	}

	// Now remove the cron job
	err = removeCron(testFuncPath)
	if err != nil {
		t.Fatalf("removeCron failed: %v", err)
	}

	// Verify the cron job was removed
	cmd = exec.Command("crontab", "-l")
	output, err = cmd.CombinedOutput()
	crontabContent := string(output)

	// Check if crontab is empty or doesn't contain the function
	if err == nil && strings.Contains(crontabContent, testFuncPath) {
		t.Errorf("Cron job was not removed. Crontab content:\n%s", crontabContent)
	}
}

func TestRemoveCronMultipleFunctions(t *testing.T) {
	// Skip test if crontab is not available
	if _, err := exec.LookPath("crontab"); err != nil {
		t.Skip("crontab command not available")
	}

	// Setup: Create a temporary directory and function files
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	funcDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatalf("Failed to create test functions directory: %v", err)
	}

	// Create multiple test function files
	functions := []struct {
		name     string
		schedule string
	}{
		{"backup.py", "0 0 * * *"},
		{"sync.js", "0 * * * *"},
		{"cleanup.go", "0 0 * * 0"},
	}

	funcPaths := make([]string, len(functions))
	for i, fn := range functions {
		testFuncPath := filepath.Join(funcDir, fn.name)
		funcPaths[i] = testFuncPath
		if err := os.WriteFile(testFuncPath, []byte("test"), 0755); err != nil {
			t.Fatalf("Failed to create test function file %s: %v", fn.name, err)
		}
	}

	// Setup and cleanup crontab
	cleanup := setupCrontabTest(t)
	defer cleanup()

	// Add all cron jobs
	for i, fn := range functions {
		err := addCron(funcPaths[i], fn.schedule)
		if err != nil {
			t.Fatalf("addCron failed for %s: %v", fn.name, err)
		}
	}

	// Verify all cron jobs were added
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to read crontab: %v", err)
	}
	for _, path := range funcPaths {
		if !strings.Contains(string(output), path) {
			t.Fatalf("Cron job for %s was not added", path)
		}
	}

	// Remove one cron job (the middle one)
	err = removeCron(funcPaths[1])
	if err != nil {
		t.Fatalf("removeCron failed: %v", err)
	}

	// Verify only the second function's cron job was removed
	cmd = exec.Command("crontab", "-l")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to read crontab: %v", err)
	}
	crontabContent := string(output)

	// First and third should still exist
	if !strings.Contains(crontabContent, funcPaths[0]) {
		t.Errorf("Cron job for %s was incorrectly removed", funcPaths[0])
	}
	if !strings.Contains(crontabContent, funcPaths[2]) {
		t.Errorf("Cron job for %s was incorrectly removed", funcPaths[2])
	}

	// Second should be removed
	if strings.Contains(crontabContent, funcPaths[1]) {
		t.Errorf("Cron job for %s was not removed. Crontab content:\n%s", funcPaths[1], crontabContent)
	}
}

func TestRemoveCronNonExistent(t *testing.T) {
	// Skip test if crontab is not available
	if _, err := exec.LookPath("crontab"); err != nil {
		t.Skip("crontab command not available")
	}

	// Setup: Create a temporary directory
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	funcDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatalf("Failed to create test functions directory: %v", err)
	}

	testFuncPath := filepath.Join(funcDir, "nonexistent.py")

	// Setup and cleanup crontab
	cleanup := setupCrontabTest(t)
	defer cleanup()

	// Try to remove a cron job that doesn't exist
	err := removeCron(testFuncPath)
	if err != nil {
		t.Fatalf("removeCron should not fail for non-existent cron job: %v", err)
	}
}

func TestExecutionLogFileNaming(t *testing.T) {
	// This test verifies that execution log files are named correctly
	// by stripping the extension from the function name

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create logs/functions directory
	logsDir := filepath.Join(tmpHome, ".opencloud", "logs", "functions")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs directory: %v", err)
	}

	// Test cases for different function extensions
	testCases := []struct {
		functionName string
		expectedLog  string
	}{
		{"hello.py", "hello.log"},
		{"test.js", "test.log"},
		{"script.go", "script.log"},
		{"function.sh", "function.log"},
	}

	for _, tc := range testCases {
		// Create a test log file as it would be created by RunFunction
		baseName := strings.TrimSuffix(tc.functionName, filepath.Ext(tc.functionName))
		logFileName := baseName + ".log"
		logFilePath := filepath.Join(logsDir, logFileName)

		// Create the log file
		if err := os.WriteFile(logFilePath, []byte("test log content"), 0644); err != nil {
			t.Fatalf("Failed to create test log file for %s: %v", tc.functionName, err)
		}

		// Verify the file exists at the expected path
		if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
			t.Errorf("Expected log file not found: %s", logFilePath)
		}

		// Now simulate deletion using the same logic as DeleteFunction
		fnName := tc.functionName
		baseName = strings.TrimSuffix(fnName, filepath.Ext(fnName))
		deletionPath := filepath.Join(logsDir, baseName+".log")

		if err := os.Remove(deletionPath); err != nil {
			t.Errorf("Failed to remove log file for %s: %v", tc.functionName, err)
		}

		// Verify the file was deleted
		if _, err := os.Stat(deletionPath); !os.IsNotExist(err) {
			t.Errorf("Log file should have been deleted but still exists: %s", deletionPath)
		}
	}
}

func TestFunctionRename(t *testing.T) {
	// This test verifies that renaming a function updates the file, service ledger, and logs

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create functions directory
	funcDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatalf("Failed to create functions directory: %v", err)
	}

	// Create logs directory
	logsDir := filepath.Join(tmpHome, ".opencloud", "logs", "functions")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs directory: %v", err)
	}

	// Create initial function file
	oldFileName := "old_function.py"
	oldFilePath := filepath.Join(funcDir, oldFileName)
	initialCode := "print('old function')"
	if err := os.WriteFile(oldFilePath, []byte(initialCode), 0644); err != nil {
		t.Fatalf("Failed to create initial function file: %v", err)
	}

	// Create a log file for the old function
	oldLogPath := filepath.Join(logsDir, "old_function.log")
	if err := os.WriteFile(oldLogPath, []byte("old log content"), 0644); err != nil {
		t.Fatalf("Failed to create old log file: %v", err)
	}

	// Verify initial state
	if _, err := os.Stat(oldFilePath); os.IsNotExist(err) {
		t.Fatal("Old function file should exist")
	}
	if _, err := os.Stat(oldLogPath); os.IsNotExist(err) {
		t.Fatal("Old log file should exist")
	}

	// Simulate the rename operation (as would happen in UpdateFunction)
	newFileName := "new_function.py"
	newFilePath := filepath.Join(funcDir, newFileName)
	newCode := "print('new function')"

	// Write updated code to old file first
	if err := os.WriteFile(oldFilePath, []byte(newCode), 0644); err != nil {
		t.Fatalf("Failed to update function code: %v", err)
	}

	// Rename the file
	if err := os.Rename(oldFilePath, newFilePath); err != nil {
		t.Fatalf("Failed to rename function file: %v", err)
	}

	// Rename the log file
	newLogPath := filepath.Join(logsDir, "new_function.log")
	if _, err := os.Stat(oldLogPath); err == nil {
		if err := os.Rename(oldLogPath, newLogPath); err != nil {
			t.Fatalf("Failed to rename log file: %v", err)
		}
	}

	// Verify new state
	if _, err := os.Stat(oldFilePath); !os.IsNotExist(err) {
		t.Error("Old function file should not exist after rename")
	}

	if _, err := os.Stat(newFilePath); os.IsNotExist(err) {
		t.Error("New function file should exist after rename")
	} else {
		// Verify the content was updated
		content, err := os.ReadFile(newFilePath)
		if err != nil {
			t.Fatalf("Failed to read new function file: %v", err)
		}
		if string(content) != newCode {
			t.Errorf("New function file has wrong content. Expected: %s, Got: %s", newCode, string(content))
		}
	}

	if _, err := os.Stat(oldLogPath); !os.IsNotExist(err) {
		t.Error("Old log file should not exist after rename")
	}

	if _, err := os.Stat(newLogPath); os.IsNotExist(err) {
		t.Error("New log file should exist after rename")
	}
}

// TestListFunctionsIncludesInvocations verifies that ListFunctions returns invocation counts
// read from the service ledger rather than always returning zero.
func TestListFunctionsIncludesInvocations(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create the functions directory and a test function file
	funcDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatalf("Failed to create functions directory: %v", err)
	}

	fnName := "test_list_invocations.py"
	fnPath := filepath.Join(funcDir, fnName)
	if err := os.WriteFile(fnPath, []byte("print('hello')"), 0644); err != nil {
		t.Fatalf("Failed to create test function file: %v", err)
	}

	// Add an entry to the service ledger with a known invocation count
	// using direct ledger manipulation to set a specific count
	if err := service_ledger.UpdateFunctionEntry(fnName, "python", "", "", "print('hello')"); err != nil {
		t.Fatalf("Failed to create function entry in ledger: %v", err)
	}
	defer service_ledger.DeleteFunctionEntry(fnName)

	// Increment invocations twice so we have a non-zero count
	for i := 0; i < 2; i++ {
		if err := service_ledger.IncrementFunctionInvocations(fnName); err != nil {
			t.Fatalf("Failed to increment invocations: %v", err)
		}
	}

	// Call ListFunctions handler
	req := httptest.NewRequest(http.MethodGet, "/list-functions", nil)
	w := httptest.NewRecorder()
	ListFunctions(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var functions []FunctionItem
	if err := json.NewDecoder(resp.Body).Decode(&functions); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Find our test function and check invocation count
	found := false
	for _, fn := range functions {
		if fn.Name == fnName {
			found = true
			if fn.Invocations != 2 {
				t.Errorf("Expected invocations to be 2 for %s, got %d", fnName, fn.Invocations)
			}
			break
		}
	}
	if !found {
		t.Errorf("Function %s not found in ListFunctions response", fnName)
	}
}

// TestGetFunctionIncludesInvocations verifies that GetFunction returns the invocation count
// from the service ledger rather than always returning zero.
func TestGetFunctionIncludesInvocations(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create the functions directory and a test function file
	funcDir := filepath.Join(tmpHome, ".opencloud", "functions")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatalf("Failed to create functions directory: %v", err)
	}

	fnName := "test_get_invocations.py"
	fnPath := filepath.Join(funcDir, fnName)
	if err := os.WriteFile(fnPath, []byte("print('hello')"), 0644); err != nil {
		t.Fatalf("Failed to create test function file: %v", err)
	}

	// Add an entry to the service ledger
	if err := service_ledger.UpdateFunctionEntry(fnName, "python", "", "", "print('hello')"); err != nil {
		t.Fatalf("Failed to create function entry in ledger: %v", err)
	}
	defer service_ledger.DeleteFunctionEntry(fnName)

	// Increment invocations five times
	for i := 0; i < 5; i++ {
		if err := service_ledger.IncrementFunctionInvocations(fnName); err != nil {
			t.Fatalf("Failed to increment invocations: %v", err)
		}
	}

	// Call GetFunction handler
	req := httptest.NewRequest(http.MethodGet, "/get-function/"+fnName, nil)
	req.RequestURI = "/get-function/" + fnName
	w := httptest.NewRecorder()
	GetFunction(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// The response uses "Invocations" (capital I) - check it
	inv, ok := result["Invocations"]
	if !ok {
		t.Fatal("Response missing 'Invocations' field")
	}
	// JSON numbers decode as float64
	if int(inv.(float64)) != 5 {
		t.Errorf("Expected Invocations to be 5, got %v", inv)
	}
}


// TestGetContainersHandler verifies that GetContainers does not panic and
// returns either 200 (Podman available) or 500 (Podman unavailable)
// in a test environment.  When Podman is unavailable the handler must
// still return a valid HTTP status; when it is available and no containers
// exist the response body must be a JSON array (not null) so that the
// frontend can safely call .filter() / .length without crashing.
func TestGetContainersHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/get-containers", nil)
	w := httptest.NewRecorder()

	// In a test environment Podman is typically not running, so the handler
	// is expected to return an Internal Server Error.  What we verify here is
	// that the handler completes without panicking and returns a recognised HTTP
	// status code.
	GetContainers(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", resp.StatusCode)
	}

	// When the handler returns 200 (Podman reachable, zero containers),
	// the body must be a JSON array — not null — so that the frontend can
	// safely call .filter() / .length without crashing.
	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		trimmed := strings.TrimSpace(string(body))
		if trimmed == "null" {
			t.Error("Response body must be a JSON array, not null, when no containers are present")
		}
	}
}

func TestGetContainersIncludesStoppedContainers(t *testing.T) {
	origConnection := getContainersConnection
	origList := listPodmanContainers
	t.Cleanup(func() {
		getContainersConnection = origConnection
		listPodmanContainers = origList
	})

	getContainersConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	listPodmanContainers = func(ctx context.Context, opts *containers.ListOptions) ([]podmanTypes.ListContainer, error) {
		if opts == nil || !opts.GetAll() {
			t.Fatalf("expected GetContainers to list all containers")
		}
		if !opts.GetSync() {
			t.Fatalf("expected GetContainers to synchronize container state before listing")
		}

		now := time.Now()
		return []podmanTypes.ListContainer{
			{
				ID:      "container-running-1234567890",
				Names:   []string{"/running"},
				Image:   "nginx:latest",
				State:   "running",
				Status:  "Up 5 minutes",
				Created: now,
				Pid:     os.Getpid(),
			},
			{
				ID:      "container-exited-0987654321",
				Names:   []string{"/exited"},
				Image:   "busybox:latest",
				State:   "exited",
				Status:  "Exited (0) 2 minutes ago",
				Created: now.Add(-time.Minute),
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-containers", nil)
	w := httptest.NewRecorder()

	GetContainers(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var got []ContainerInfo
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(got))
	}

	if got[0].State != "running" {
		t.Fatalf("expected first container to be running, got %q", got[0].State)
	}
	if got[0].MemoryUsageBytes <= 0 {
		t.Fatalf("expected running container to include memory usage, got %d", got[0].MemoryUsageBytes)
	}

	if got[1].State != "exited" {
		t.Fatalf("expected second container to be exited, got %q", got[1].State)
	}
	if got[1].Status != "Exited (0) 2 minutes ago" {
		t.Fatalf("expected exited container status to be preserved, got %q", got[1].Status)
	}
	if got[1].MemoryUsageBytes != 0 {
		t.Fatalf("expected exited container memory usage to remain 0, got %d", got[1].MemoryUsageBytes)
	}
}

func TestDeleteContainerInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/delete-container", nil)
	w := httptest.NewRecorder()

	DeleteContainer(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestDeleteContainerInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/delete-container", strings.NewReader("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	DeleteContainer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteContainerMissingContainerID(t *testing.T) {
	body, _ := json.Marshal(DeleteContainerRequest{ContainerID: ""})
	req := httptest.NewRequest(http.MethodPost, "/delete-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	DeleteContainer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteContainerRemovesContainerViaPodman(t *testing.T) {
	origConnection := deleteContainerConnection
	origRemove := removePodmanContainer
	t.Cleanup(func() {
		deleteContainerConnection = origConnection
		removePodmanContainer = origRemove
	})

	deleteContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	removeCalled := false
	removePodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		removeCalled = true
		if nameOrID != "container-123" {
			t.Fatalf("expected container ID container-123, got %q", nameOrID)
		}
		if opts == nil || !opts.GetForce() {
			t.Fatalf("expected DeleteContainer to force removal")
		}
		return nil, nil
	}

	body, _ := json.Marshal(DeleteContainerRequest{ContainerID: "container-123"})
	req := httptest.NewRequest(http.MethodPost, "/delete-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	DeleteContainer(w, req)

	if !removeCalled {
		t.Fatal("expected Podman remove to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "deleted" || resp["containerId"] != "container-123" {
		t.Fatalf("unexpected response body: %#v", resp)
	}
}

func TestDeleteContainerRemoveFailure(t *testing.T) {
	origConnection := deleteContainerConnection
	origRemove := removePodmanContainer
	t.Cleanup(func() {
		deleteContainerConnection = origConnection
		removePodmanContainer = origRemove
	})

	deleteContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	removePodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		return nil, errors.New("remove failed")
	}

	body, _ := json.Marshal(DeleteContainerRequest{ContainerID: "container-123"})
	req := httptest.NewRequest(http.MethodPost, "/delete-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	DeleteContainer(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestContainerActionInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/containers/container-123/stop", nil)
	w := httptest.NewRecorder()

	ContainerAction(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestContainerActionMissingPathSegments(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/containers/container-123", nil)
	w := httptest.NewRecorder()

	ContainerAction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestContainerActionStopsContainerViaPodman(t *testing.T) {
	origConnection := containerActionConnection
	origStop := stopPodmanContainer
	t.Cleanup(func() {
		containerActionConnection = origConnection
		stopPodmanContainer = origStop
	})

	containerActionConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	stopCalled := false
	stopPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error {
		stopCalled = true
		if nameOrID != "container-123" {
			t.Fatalf("expected container ID container-123, got %q", nameOrID)
		}
		if opts != nil {
			t.Fatalf("expected nil stop options, got %#v", opts)
		}
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/containers/container-123/stop", nil)
	w := httptest.NewRecorder()

	ContainerAction(w, req)

	if !stopCalled {
		t.Fatal("expected Podman stop to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "stopped" || resp["containerId"] != "container-123" {
		t.Fatalf("unexpected response body: %#v", resp)
	}
}

func TestContainerActionStartsContainerViaPodman(t *testing.T) {
	origConnection := containerActionConnection
	origStart := startPodmanContainer
	t.Cleanup(func() {
		containerActionConnection = origConnection
		startPodmanContainer = origStart
	})

	containerActionConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	startCalled := false
	startPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.StartOptions) error {
		startCalled = true
		if nameOrID != "container-123" {
			t.Fatalf("expected container ID container-123, got %q", nameOrID)
		}
		if opts != nil {
			t.Fatalf("expected nil start options, got %#v", opts)
		}
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/containers/container-123/start", nil)
	w := httptest.NewRecorder()

	ContainerAction(w, req)

	if !startCalled {
		t.Fatal("expected Podman start to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "started" || resp["containerId"] != "container-123" {
		t.Fatalf("unexpected response body: %#v", resp)
	}
}

func TestContainerActionUnsupportedAction(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/containers/container-123/restart", nil)
	w := httptest.NewRecorder()

	ContainerAction(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestContainerActionStopFailure(t *testing.T) {
	origConnection := containerActionConnection
	origStop := stopPodmanContainer
	t.Cleanup(func() {
		containerActionConnection = origConnection
		stopPodmanContainer = origStop
	})

	containerActionConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	stopPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error {
		return errors.New("stop failed")
	}

	req := httptest.NewRequest(http.MethodPost, "/containers/container-123/stop", nil)
	w := httptest.NewRecorder()

	ContainerAction(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// TestContainerMemoryUsageBytesZeroPID verifies that containerMemoryUsageBytes
// returns 0 when given a zero PID (no running process).
func TestContainerMemoryUsageBytesZeroPID(t *testing.T) {
	if got := containerMemoryUsageBytes(0); got != 0 {
		t.Errorf("Expected 0 for zero PID, got %d", got)
	}
}

// TestContainerMemoryUsageBytesInvalidPID verifies that containerMemoryUsageBytes
// returns 0 for a PID that does not correspond to a live process.
func TestContainerMemoryUsageBytesInvalidPID(t *testing.T) {
	// PID math.MaxUint32 is very unlikely to be a real process.
	if got := containerMemoryUsageBytes(^uint32(0)); got != 0 {
		t.Errorf("Expected 0 for non-existent PID, got %d", got)
	}
}

// TestContainerMemoryUsageBytesCurrentProcess verifies that
// containerMemoryUsageBytes returns a positive, non-zero value for the current
// test process (which is guaranteed to have /proc/{PID}/status on Linux).
func TestContainerMemoryUsageBytesCurrentProcess(t *testing.T) {
	if _, err := os.Stat("/proc/self/status"); os.IsNotExist(err) {
		t.Skip("/proc filesystem not available – skipping Linux-specific test")
	}

	// Obtain the current process PID.
	pid := uint32(os.Getpid())
	mem := containerMemoryUsageBytes(pid)
	if mem <= 0 {
		t.Errorf("Expected positive memory value for current process (pid %d), got %d", pid, mem)
	}
}

// TestPullAndRunHandlerMethodNotAllowed verifies that non-POST requests are rejected with 405.
func TestPullAndRunHandlerMethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/pull-and-run", nil)
		w := httptest.NewRecorder()
		PullAndRun(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Method %s: expected 405, got %d", method, w.Code)
		}
	}
}

// TestPullAndRunHandlerInvalidJSON verifies that malformed JSON returns 400.
func TestPullAndRunHandlerInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/pull-and-run", strings.NewReader("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	PullAndRun(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestPullAndRunHandlerMissingImage verifies that a request without an image returns 400.
func TestPullAndRunHandlerMissingImage(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/pull-and-run", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	PullAndRun(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestPullAndRunHandlerInvalidImageName verifies that a dangerous image name returns 400.
func TestPullAndRunHandlerInvalidImageName(t *testing.T) {
	cases := []string{"../etc/passwd", "/etc/passwd", "my image", "image\\name"}
	for _, img := range cases {
		body := `{"image":"` + img + `"}`
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRun(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Image %q: expected 400, got %d", img, w.Code)
		}
	}
}

// TestPullAndRunHandlerInvalidContainerName verifies that an invalid container name returns 400.
func TestPullAndRunHandlerInvalidContainerName(t *testing.T) {
	cases := []string{"-bad", ".bad", "bad name", "bad/name"}
	for _, name := range cases {
		body, _ := json.Marshal(map[string]string{"image": "nginx:latest", "name": name})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRun(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Container name %q: expected 400, got %d", name, w.Code)
		}
	}
}

// TestPullAndRunHandlerInvalidPort verifies that an invalid port mapping returns 400.
func TestPullAndRunHandlerInvalidPort(t *testing.T) {
	cases := []string{"nocodon", "../80:80", "8080;80"}
	for _, port := range cases {
		body, _ := json.Marshal(map[string]interface{}{"image": "nginx:latest", "ports": []string{port}})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRun(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Port %q: expected 400, got %d", port, w.Code)
		}
	}
}

// TestPullAndRunHandlerInvalidVolume verifies that a volume with path traversal returns 400.
func TestPullAndRunHandlerInvalidVolume(t *testing.T) {
	cases := []string{"../../etc:/data", "/data/../../etc:/data", "/host", "/host:../container"}
	for _, vol := range cases {
		body, _ := json.Marshal(map[string]interface{}{"image": "nginx:latest", "volumes": []string{vol}})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRun(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Volume %q: expected 400, got %d", vol, w.Code)
		}
	}
}

// TestPullAndRunHandlerInvalidRestartPolicy verifies that an unknown restart policy returns 400.
func TestPullAndRunHandlerInvalidRestartPolicy(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"image": "nginx:latest", "restartPolicy": "invalid-policy"})
	req := httptest.NewRequest(http.MethodPost, "/pull-and-run", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	PullAndRun(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid restart policy, got %d", w.Code)
	}
}

// TestPullAndRunHandlerAutoRemoveWithRestartPolicy verifies that combining autoRemove with
// a non-"no" restart policy returns 400 (the two options are mutually exclusive).
func TestPullAndRunHandlerAutoRemoveWithRestartPolicy(t *testing.T) {
	for _, policy := range []string{"always", "on-failure", "unless-stopped"} {
		body, _ := json.Marshal(map[string]interface{}{
			"image":         "nginx:latest",
			"autoRemove":    true,
			"restartPolicy": policy,
		})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRun(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Policy %q with autoRemove: expected 400, got %d", policy, w.Code)
		}
	}
}

// TestPullAndRunHandlerValidRequest verifies that a well-formed request passes all validation
// and either succeeds (nerdctl available) or returns 500 (nerdctl unavailable in test env).
func TestPullAndRunHandlerValidRequest(t *testing.T) {
	body, _ := json.Marshal(PullAndRunRequest{
		Image:         "nginx:latest",
		Name:          "test-container",
		Ports:         []string{"8080:80"},
		Env:           []string{"FOO=bar"},
		Volumes:       []string{"/tmp:/data"},
		RestartPolicy: "no",
		AutoRemove:    false,
		Command:       "",
	})
	req := httptest.NewRequest(http.MethodPost, "/pull-and-run", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	PullAndRun(w, req)
	// In a CI environment nerdctl is typically unavailable, so 500 is expected.
	// In production (nerdctl present) the handler should return 200.
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 200 or 500, got %d", w.Code)
	}
}

// TestPullAndRunStreamHandlerMethodNotAllowed verifies that non-POST requests are rejected with 405.
func TestPullAndRunStreamHandlerMethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/pull-and-run-stream", nil)
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Method %s: expected 405, got %d", method, w.Code)
		}
	}
}

// TestPullAndRunStreamHandlerInvalidJSON verifies that malformed JSON returns 400.
func TestPullAndRunStreamHandlerInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	PullAndRunStream(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestPullAndRunStreamHandlerMissingImage verifies that a request without an image returns 400.
func TestPullAndRunStreamHandlerMissingImage(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	PullAndRunStream(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestPullAndRunStreamHandlerInvalidImageName verifies that a dangerous image name returns 400.
func TestPullAndRunStreamHandlerInvalidImageName(t *testing.T) {
	cases := []string{"../etc/passwd", "/etc/passwd", "my image", "image\\name"}
	for _, img := range cases {
		body := `{"image":"` + img + `"}`
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Image %q: expected 400, got %d", img, w.Code)
		}
	}
}

// TestPullAndRunStreamHandlerInvalidContainerName verifies that an invalid container name returns 400.
func TestPullAndRunStreamHandlerInvalidContainerName(t *testing.T) {
	cases := []string{"-bad", ".bad", "bad name", "bad/name"}
	for _, name := range cases {
		body, _ := json.Marshal(map[string]string{"image": "nginx:latest", "name": name})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Container name %q: expected 400, got %d", name, w.Code)
		}
	}
}

// TestPullAndRunStreamHandlerInvalidPort verifies that an invalid port mapping returns 400.
func TestPullAndRunStreamHandlerInvalidPort(t *testing.T) {
	cases := []string{"nocodon", "../80:80", "8080;80"}
	for _, port := range cases {
		body, _ := json.Marshal(map[string]interface{}{"image": "nginx:latest", "ports": []string{port}})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Port %q: expected 400, got %d", port, w.Code)
		}
	}
}

// TestPullAndRunStreamHandlerInvalidVolume verifies that a volume with path traversal returns 400.
func TestPullAndRunStreamHandlerInvalidVolume(t *testing.T) {
	cases := []string{"../../etc:/data", "/data/../../etc:/data", "/host", "/host:../container"}
	for _, vol := range cases {
		body, _ := json.Marshal(map[string]interface{}{"image": "nginx:latest", "volumes": []string{vol}})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Volume %q: expected 400, got %d", vol, w.Code)
		}
	}
}

// TestPullAndRunStreamHandlerInvalidRestartPolicy verifies that an unknown restart policy returns 400.
func TestPullAndRunStreamHandlerInvalidRestartPolicy(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"image": "nginx:latest", "restartPolicy": "invalid-policy"})
	req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	PullAndRunStream(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid restart policy, got %d", w.Code)
	}
}

// TestPullAndRunStreamHandlerAutoRemoveWithRestartPolicy verifies that combining autoRemove
// with a non-"no" restart policy returns 400.
func TestPullAndRunStreamHandlerAutoRemoveWithRestartPolicy(t *testing.T) {
	for _, policy := range []string{"always", "on-failure", "unless-stopped"} {
		body, _ := json.Marshal(map[string]interface{}{
			"image":         "nginx:latest",
			"autoRemove":    true,
			"restartPolicy": policy,
		})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Policy %q with autoRemove: expected 400, got %d", policy, w.Code)
		}
	}
}

// TestPullAndRunStreamHandlerValidRequest verifies that a well-formed request passes all
// validation and either streams SSE (200) or returns 500 when Podman is unavailable.
func TestPullAndRunStreamHandlerValidRequest(t *testing.T) {
	body, _ := json.Marshal(PullAndRunRequest{
		Image:         "nginx:latest",
		Name:          "test-stream-container",
		Ports:         []string{"8081:80"},
		Env:           []string{"FOO=bar"},
		Volumes:       []string{"/tmp:/data"},
		RestartPolicy: "no",
		AutoRemove:    false,
		Command:       "",
	})
	req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	PullAndRunStream(w, req)
	// In CI Podman is unavailable so 500 is expected; in production 200 with SSE.
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 200 or 500, got %d", w.Code)
	}
	// When the handler succeeds it must set the SSE content-type.
	if w.Code == http.StatusOK {
		ct := w.Header().Get("Content-Type")
		if ct != "text/event-stream" {
			t.Errorf("Expected Content-Type text/event-stream, got %q", ct)
		}
	}
}

// TestValidateContainerName verifies valid and invalid container names.
func TestValidateContainerName(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"my-container", false},
		{"my_container", false},
		{"my.container", false},
		{"container123", false},
		{"A", false},
		{"-container", true},
		{".container", true},
		{"", true},
		{"my container", true},
		{"my/container", true},
		{"my@container", true},
	}
	for _, tt := range tests {
		result := validateContainerName(tt.input)
		if tt.wantErr && result == "" {
			t.Errorf("validateContainerName(%q): expected error, got none", tt.input)
		} else if !tt.wantErr && result != "" {
			t.Errorf("validateContainerName(%q): expected no error, got %q", tt.input, result)
		}
	}
}

// TestValidatePortMapping verifies valid and invalid port mapping strings.
func TestValidatePortMapping(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"8080:80", false},
		{"8080:80/tcp", false},
		{"0.0.0.0:8080:80", false},
		{"80", false},       // dynamic host port (no colon required)
		{"80/tcp", false},   // dynamic host port with protocol
		{"8080", false},     // dynamic host port assignment
		{"nocodon", true},   // non-numeric container port
		{"0:80", true},      // host port must be 1-65535; port 0 is not a valid host port
		{"../80:80", true},  // path traversal
		{"8080;80", true},   // semicolon
		{"8080 80", true},   // space
	}
	for _, tt := range tests {
		result := validatePortMapping(tt.input)
		if tt.wantErr && result == "" {
			t.Errorf("validatePortMapping(%q): expected error, got none", tt.input)
		} else if !tt.wantErr && result != "" {
			t.Errorf("validatePortMapping(%q): expected no error, got %q", tt.input, result)
		}
	}
}

func TestParsePortMapping(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
		hostIP      string
		hostPort    uint16
		container   uint16
		protocol    string
	}{
		{
			name:      "host and container port",
			input:     "8080:80",
			hostPort:  8080,
			container: 80,
			protocol:  "tcp",
		},
		{
			name:      "host ip and udp",
			input:     "127.0.0.1:5353:53/udp",
			hostIP:    "127.0.0.1",
			hostPort:  5353,
			container: 53,
			protocol:  "udp",
		},
		{
			name:        "invalid numeric port",
			input:       "abc:80",
			expectError: true,
		},
		{
			name:      "dynamic host port - container port only",
			input:     "80",
			hostPort:  0,
			container: 80,
			protocol:  "tcp",
		},
		{
			name:      "dynamic host port with protocol",
			input:     "443/tcp",
			hostPort:  0,
			container: 443,
			protocol:  "tcp",
		},
		{
			name:        "invalid dynamic port - non-numeric",
			input:       "abc",
			expectError: true,
		},
		{
			name:        "invalid dynamic port - zero",
			input:       "0",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mapping, err := parsePortMapping(tc.input)
			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected an error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error for %q: %v", tc.input, err)
			}
			if mapping.HostIP != tc.hostIP || mapping.HostPort != tc.hostPort || mapping.ContainerPort != tc.container || mapping.Protocol != tc.protocol {
				t.Fatalf("Unexpected port mapping for %q: %+v", tc.input, mapping)
			}
		})
	}
}

// TestValidateVolumeMount verifies valid and invalid volume mount strings.
func TestValidateVolumeMount(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"/host/data:/container/data", false},
		{"/tmp:/data", false},
		{"data:/container/data", false},
		{"../../etc:/container/data", true}, // path traversal in host
		{"/host/data:../../etc", true},      // path traversal in container
		{"/host/data", true},                // no colon
		{"/host/data:/container/data:Z", false},    // Podman SELinux relabelling (shared)
		{"/host/data:/container/data:U", false},    // Podman user-namespace mapping
		{"/host/data:/container/data:Z,U", false},  // combined Podman options (issue example)
		{"/host/data:/container/data:ro", false},   // read-only option
		{"/host/data:/container/data:badopt", true}, // unknown option
	}
	for _, tt := range tests {
		result := validateVolumeMount(tt.input)
		if tt.wantErr && result == "" {
			t.Errorf("validateVolumeMount(%q): expected error, got none", tt.input)
		} else if !tt.wantErr && result != "" {
			t.Errorf("validateVolumeMount(%q): expected no error, got %q", tt.input, result)
		}
	}
}

// TestParseMountOptions verifies that parseMountOptions accepts known flags and rejects unknown ones.
func TestParseMountOptions(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"Z", false},      // Podman SELinux relabelling (shared)
		{"z", false},      // Podman SELinux relabelling (private)
		{"U", false},      // Podman user-namespace mapping
		{"Z,U", false},    // combined Podman options (used by the issue example)
		{"ro", false},
		{"rw", false},
		{"rbind", false},
		{"Z,U,ro", false}, // combined Podman options with standard flag
		{"badopt", true},
		{"ro,badopt", true}, // valid option combined with unknown option
		{"", false}, // empty string is a no-op
	}
	for _, tt := range tests {
		result := parseMountOptions(tt.input)
		if tt.wantErr && result == "" {
			t.Errorf("parseMountOptions(%q): expected error, got none", tt.input)
		} else if !tt.wantErr && result != "" {
			t.Errorf("parseMountOptions(%q): expected no error, got %q", tt.input, result)
		}
	}
}

// TestExpandTildePath verifies that leading "~" is expanded to the user home directory.
func TestExpandTildePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir:", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~", home},
		{"~/logs", home + "/logs"},
		{"~/a/b/c", home + "/a/b/c"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~hidden", "~hidden"},  // tilde not followed by "/" – leave unchanged
		{"", ""},
	}

	for _, tt := range tests {
		got := expandTildePath(tt.input)
		if got != tt.want {
			t.Errorf("expandTildePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestIsNamedVolumeMount verifies the named volume detection helper.
func TestIsNamedVolumeMount(t *testing.T) {
	tests := []struct {
		source string
		named  bool
	}{
		{"opencloud-my-bucket", true},
		{"myvolume", true},
		{"/host/path", false},
		{"~/some/path", false},
		{"~/.opencloud/blob_storage/bucket", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isNamedVolumeMount(tt.source)
		if got != tt.named {
			t.Errorf("isNamedVolumeMount(%q) = %v, want %v", tt.source, got, tt.named)
		}
	}
}

// TestParseVolumeStrings verifies that parseVolumeStrings correctly separates named volumes
// from bind mounts based on whether the source starts with "/" or "~".
func TestParseVolumeStrings(t *testing.T) {
	vols := []string{
		"opencloud-my-bucket:/app/data",
		"/host/path:/container/path",
		"opencloud-second:/mnt:ro",
		"~/logs:/logs",
	}
	namedVolumes, bindMounts := parseVolumeStrings(vols)

	if len(namedVolumes) != 2 {
		t.Fatalf("expected 2 named volumes, got %d", len(namedVolumes))
	}
	if namedVolumes[0].Name != "opencloud-my-bucket" {
		t.Errorf("expected first named vol %q, got %q", "opencloud-my-bucket", namedVolumes[0].Name)
	}
	if namedVolumes[0].Dest != "/app/data" {
		t.Errorf("expected first named vol dest %q, got %q", "/app/data", namedVolumes[0].Dest)
	}
	if namedVolumes[1].Name != "opencloud-second" {
		t.Errorf("expected second named vol %q, got %q", "opencloud-second", namedVolumes[1].Name)
	}

	if len(bindMounts) != 2 {
		t.Fatalf("expected 2 bind mounts, got %d", len(bindMounts))
	}
	if bindMounts[0].Source != "/host/path" {
		t.Errorf("expected first bind src %q, got %q", "/host/path", bindMounts[0].Source)
	}
}

// TestGetContainerInvalidMethod verifies that non-GET requests are rejected with 405.
func TestGetContainerInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/get-container?id=abc123", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// TestGetContainerMissingID verifies that a missing "id" query parameter returns 400.
func TestGetContainerMissingID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/get-container", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestGetContainerReturnsDetail verifies that GetContainer calls Podman Inspect
// and returns the container details correctly encoded as JSON.
func TestGetContainerReturnsDetail(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	now := time.Now()
	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		if nameOrID != "test-container-id" {
			t.Fatalf("expected container ID test-container-id, got %q", nameOrID)
		}
		return &define.InspectContainerData{
			ID:        "test-container-id",
			Name:      "/my-container",
			ImageName: "nginx:latest",
			Created:   now,
			State: &define.InspectContainerState{
				Status: "running",
				Pid:    0,
			},
			Config: &define.InspectContainerConfig{
				Env: []string{"FOO=bar", "BAZ=qux"},
			},
			// Mounts drives the allowlist (Type=="bind" only).
			Mounts: []define.InspectMount{
				{Type: "bind", Source: "/host/data", Destination: "/container/data", Mode: "rw"},
			},
			// HostConfig.Binds is the primary source for options.
			HostConfig: &define.InspectContainerHostConfig{
				Binds:      []string{"/host/data:/container/data:rw"},
				AutoRemove: false,
				RestartPolicy: &define.InspectRestartPolicy{
					Name: "always",
				},
				PortBindings: map[string][]define.InspectHostPort{
					"80/tcp": {{HostIP: "", HostPort: "8080"}},
				},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=test-container-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var detail ContainerDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if detail.ID != "test-container-id" {
		t.Errorf("expected ID test-container-id, got %q", detail.ID)
	}
	if detail.Name != "my-container" {
		t.Errorf("expected Name my-container, got %q", detail.Name)
	}
	if detail.Image != "nginx:latest" {
		t.Errorf("expected Image nginx:latest, got %q", detail.Image)
	}
	if detail.State != "running" {
		t.Errorf("expected State running, got %q", detail.State)
	}
	if detail.RestartPolicy != "always" {
		t.Errorf("expected RestartPolicy always, got %q", detail.RestartPolicy)
	}
	if len(detail.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(detail.Env))
	}
	if len(detail.Binds) != 1 || detail.Binds[0] != "/host/data:/container/data:rw" {
		t.Errorf("unexpected Binds: %v", detail.Binds)
	}
	if len(detail.Ports) != 1 {
		t.Errorf("expected 1 port mapping, got %d", len(detail.Ports))
	}
}

// TestGetContainerExcludesAnonymousVolumes verifies that anonymous and named
// volumes (Podman internal storage paths) are excluded from the Binds field so
// they are not passed back to UpdateContainer where they would cause a
// "no such file or directory" error after the old container is removed.
// This exercises both the Mounts path and the HostConfig.Binds path.
func TestGetContainerExcludesAnonymousVolumes(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "test-id",
			Name:      "/test",
			ImageName: "nginx:latest",
			Created:   time.Now(),
			Mounts: []define.InspectMount{
				// User-specified bind mount — must be included.
				{Type: "bind", Source: "/home/user/data", Destination: "/usr/share/nginx/html", Mode: "rw"},
				// Anonymous volume created by Podman for an image VOLUME
				// declaration — must be excluded (source is an internal path).
				{Type: "volume", Source: "/home/ubuntu/a1b2c3d4e5f6", Destination: "/var/cache/nginx", Mode: "rw"},
			},
			// HostConfig.Binds may also contain the anonymous volume path
			// as Podman used to include it in older behaviour — it must still
			// be excluded because the source directory is deleted on container
			// removal and would cause a statfs error on recreate.
			HostConfig: &define.InspectContainerHostConfig{
				Binds: []string{
					"/home/user/data:/usr/share/nginx/html",
					"/home/ubuntu/a1b2c3d4e5f6:/var/cache/nginx",
				},
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=test-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var detail ContainerDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(detail.Binds) != 1 {
		t.Fatalf("expected exactly 1 bind (anonymous volume excluded), got %d: %v", len(detail.Binds), detail.Binds)
	}
	if detail.Binds[0] != "/home/user/data:/usr/share/nginx/html" {
		t.Errorf("unexpected bind value: %q", detail.Binds[0])
	}
}

// TestGetContainerUsesVolumeLabelForVolumes verifies that when the
// opencloud/volumes label is present, GetContainer uses it as the primary source
// for volume bind strings.
func TestGetContainerUsesVolumeLabelForVolumes(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "test-id",
			Name:      "/test",
			ImageName: "nginx:latest",
			Created:   time.Now(),
			Config: &define.InspectContainerConfig{
				Labels: map[string]string{
					// opencloud/volumes stores the original user-specified bind strings.
					"opencloud/volumes": "/home/user/data:/usr/share/nginx/html:ro",
				},
			},
			// Mounts drives the allowlist (Type == "bind" only).
			Mounts: []define.InspectMount{
				{Type: "bind", Source: "/home/user/data", Destination: "/usr/share/nginx/html", Mode: "ro"},
			},
			// HostConfig.Binds may differ from the label.
			HostConfig: &define.InspectContainerHostConfig{
				Binds:         []string{"/home/user/data:/usr/share/nginx/html:rw"},
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=test-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var detail ContainerDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(detail.Binds) != 1 {
		t.Fatalf("expected 1 bind, got %d: %v", len(detail.Binds), detail.Binds)
	}
	// The label takes priority over HostConfig.Binds.
	if detail.Binds[0] != "/home/user/data:/usr/share/nginx/html:ro" {
		t.Errorf("volume label not recovered correctly; got %q", detail.Binds[0])
	}
}

// TestGetContainerVolumeLabelTakesPriorityOverHostConfigBinds verifies that when
// the opencloud/volumes label and HostConfig.Binds disagree on options for the same
// destination, the label wins (because it stores the original user intent).
func TestGetContainerVolumeLabelTakesPriorityOverHostConfigBinds(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "test-id",
			Name:      "/test",
			ImageName: "nginx:latest",
			Created:   time.Now(),
			Config: &define.InspectContainerConfig{
				Labels: map[string]string{
					// Label has ro; HostConfig.Binds has rw — label must win.
					"opencloud/volumes": "/data:/app:ro",
				},
			},
			Mounts: []define.InspectMount{
				{Type: "bind", Source: "/data", Destination: "/app", Mode: "ro"},
			},
			HostConfig: &define.InspectContainerHostConfig{
				Binds:         []string{"/data:/app:rw"},
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=test-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var detail ContainerDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(detail.Binds) != 1 {
		t.Fatalf("expected 1 bind, got %d: %v", len(detail.Binds), detail.Binds)
	}
	if detail.Binds[0] != "/data:/app:ro" {
		t.Errorf("label did not take priority over HostConfig.Binds; got %q", detail.Binds[0])
	}
}

// TestGetContainerIncludesNamedVolumesFromLabel verifies that named volumes (e.g.
// Podman named volumes created for blob storage bucket mounts like
// "opencloud-mybucket:/app/data") stored in the opencloud/volumes label are
// included in detail.Binds. Named volumes appear in data.Mounts with
// Type == "volume" (not "bind"), so they are absent from bindDests. The
// bindDests guard must NOT be applied to label entries — all entries in
// opencloud/volumes are user-specified and genuine.
func TestGetContainerIncludesNamedVolumesFromLabel(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "test-id",
			Name:      "/test",
			ImageName: "nginx:latest",
			Created:   time.Now(),
			Config: &define.InspectContainerConfig{
				Labels: map[string]string{
					// opencloud/volumes stores the original user-specified volume string,
					// including named volumes created for blob storage bucket mounts.
					"opencloud/volumes": "opencloud-mybucket:/app/data",
				},
			},
			// The named volume appears in Mounts with Type "volume", not "bind".
			// This means its destination ("/app/data") is NOT in bindDests.
			// GetContainer must still include it via the label path.
			Mounts: []define.InspectMount{
				{Type: "volume", Name: "opencloud-mybucket", Source: "/var/lib/containers/storage/volumes/opencloud-mybucket/_data", Destination: "/app/data", Mode: ""},
			},
			HostConfig: &define.InspectContainerHostConfig{
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=test-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var detail ContainerDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(detail.Binds) != 1 {
		t.Fatalf("expected 1 bind for named volume, got %d: %v", len(detail.Binds), detail.Binds)
	}
	if detail.Binds[0] != "opencloud-mybucket:/app/data" {
		t.Errorf("named volume from label not recovered; got %q, want %q", detail.Binds[0], "opencloud-mybucket:/app/data")
	}
}

// TestGetContainerIncludesMixedVolumesFromLabel verifies that when the
// opencloud/volumes label contains both a named volume and a bind mount entry,
// both are returned in detail.Binds.
func TestGetContainerIncludesMixedVolumesFromLabel(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "test-id",
			Name:      "/test",
			ImageName: "nginx:latest",
			Created:   time.Now(),
			Config: &define.InspectContainerConfig{
				Labels: map[string]string{
					"opencloud/volumes": "opencloud-mybucket:/app/data\n/host/logs:/container/logs",
				},
			},
			Mounts: []define.InspectMount{
				{Type: "volume", Name: "opencloud-mybucket", Source: "/var/lib/containers/storage/volumes/opencloud-mybucket/_data", Destination: "/app/data", Mode: ""},
				{Type: "bind", Source: "/host/logs", Destination: "/container/logs", Mode: "rw"},
			},
			HostConfig: &define.InspectContainerHostConfig{
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=test-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var detail ContainerDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(detail.Binds) != 2 {
		t.Fatalf("expected 2 binds (named + bind), got %d: %v", len(detail.Binds), detail.Binds)
	}
	if detail.Binds[0] != "opencloud-mybucket:/app/data" {
		t.Errorf("first bind: got %q, want %q", detail.Binds[0], "opencloud-mybucket:/app/data")
	}
	if detail.Binds[1] != "/host/logs:/container/logs" {
		t.Errorf("second bind: got %q, want %q", detail.Binds[1], "/host/logs:/container/logs")
	}
}

// TestGetContainerUsesPortLabelForPorts verifies that when the opencloud/ports
// label is present, GetContainer uses it as the primary source for port strings.
// This is critical for dynamic host port mappings (e.g. "80") which must round-trip
// back to the edit form without being overwritten by the runtime-assigned host port.
func TestGetContainerUsesPortLabelForPorts(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "test-id",
			Name:      "/test",
			ImageName: "nginx:latest",
			Created:   time.Now(),
			Config: &define.InspectContainerConfig{
				// opencloud/ports stores the original user-specified port strings.
				// "80" represents a dynamic host port mapping.
				Labels: map[string]string{
					"opencloud/ports": "80",
				},
			},
			HostConfig: &define.InspectContainerHostConfig{
				// Podman assigns ephemeral port 32768 at runtime — the label value
				// must take priority so the edit form shows the original intent.
				PortBindings: map[string][]define.InspectHostPort{
					"80/tcp": {{HostIP: "", HostPort: "32768"}},
				},
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=test-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var detail ContainerDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(detail.Ports) != 1 {
		t.Fatalf("expected 1 port, got %d: %v", len(detail.Ports), detail.Ports)
	}
	// The label "80" must be returned, not the runtime-assigned "32768:80/tcp".
	if detail.Ports[0] != "80" {
		t.Errorf("port label not recovered correctly; got %q, want %q", detail.Ports[0], "80")
	}
}

// TestGetContainerPortLabelFallbackToPortBindings verifies that when the
// opencloud/ports label is absent, GetContainer falls back to HostConfig.PortBindings.
func TestGetContainerPortLabelFallbackToPortBindings(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "test-id",
			Name:      "/test",
			ImageName: "nginx:latest",
			Created:   time.Now(),
			Config: &define.InspectContainerConfig{
				Labels: map[string]string{},
			},
			HostConfig: &define.InspectContainerHostConfig{
				PortBindings: map[string][]define.InspectHostPort{
					"80/tcp": {{HostIP: "", HostPort: "8080"}},
				},
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=test-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var detail ContainerDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(detail.Ports) != 1 {
		t.Fatalf("expected 1 port, got %d: %v", len(detail.Ports), detail.Ports)
	}
	if detail.Ports[0] != "8080:80/tcp" {
		t.Errorf("unexpected port; got %q, want %q", detail.Ports[0], "8080:80/tcp")
	}
}

// TestUpdateContainerStoresVolumesLabel verifies that UpdateContainer writes the
// opencloud/volumes label so that GetContainer can later recover volume strings accurately.
func TestUpdateContainerStoresVolumesLabel(t *testing.T) {
	origConnection := updateContainerConnection
	origInspect := updateContainerInspect
	origStop := updateContainerStop
	origRemove := updateContainerRemove
	origEnsureImage := updateContainerEnsureImage
	origCreate := updateContainerCreateWithSpec
	origStart := updateContainerStart
	t.Cleanup(func() {
		updateContainerConnection = origConnection
		updateContainerInspect = origInspect
		updateContainerStop = origStop
		updateContainerRemove = origRemove
		updateContainerEnsureImage = origEnsureImage
		updateContainerCreateWithSpec = origCreate
		updateContainerStart = origStart
	})

	updateContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	updateContainerInspect = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "old-id",
			ImageName: "nginx:latest",
			State:     &define.InspectContainerState{Status: "exited"},
		}, nil
	}
	updateContainerStop = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error { return nil }
	updateContainerRemove = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		return nil, nil
	}
	updateContainerEnsureImage = func(ctx context.Context, ref string) (string, error) { return ref, nil }

	var capturedLabels map[string]string
	updateContainerCreateWithSpec = func(ctx context.Context, s *specgen.SpecGenerator, opts *containers.CreateOptions) (podmanTypes.ContainerCreateResponse, error) {
		capturedLabels = s.Labels
		return podmanTypes.ContainerCreateResponse{ID: "new-id"}, nil
	}
	updateContainerStart = func(ctx context.Context, nameOrID string, opts *containers.StartOptions) error { return nil }

	body, _ := json.Marshal(UpdateContainerRequest{
		ContainerID: "old-id",
		Image:       "nginx:latest",
		Name:        "test-container",
		Volumes:     []string{"/host/data:/container/data:ro"},
	})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	UpdateContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	if capturedLabels == nil {
		t.Fatal("expected spec labels to be set")
	}
	got, ok := capturedLabels["opencloud/volumes"]
	if !ok {
		t.Fatal("expected opencloud/volumes label to be set")
	}
	if got != "/host/data:/container/data:ro" {
		t.Errorf("unexpected opencloud/volumes label value: %q", got)
	}
}

// TestGetContainerMultipleVolumesFromLabel verifies that when the opencloud/volumes
// label contains multiple volume entries (newline-separated), all are parsed correctly.
func TestGetContainerMultipleVolumesFromLabel(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "test-id",
			Name:      "/test",
			ImageName: "nginx:latest",
			Created:   time.Now(),
			Config: &define.InspectContainerConfig{
				Labels: map[string]string{
					// Two volumes, newline-separated.
					"opencloud/volumes": "/data/a:/app/a:ro\n/data/b:/app/b",
				},
			},
			Mounts: []define.InspectMount{
				{Type: "bind", Source: "/data/a", Destination: "/app/a", Mode: "ro"},
				{Type: "bind", Source: "/data/b", Destination: "/app/b", Mode: "rw"},
			},
			HostConfig: &define.InspectContainerHostConfig{
				Binds:         []string{"/data/a:/app/a:rw", "/data/b:/app/b:rw"},
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=test-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var detail ContainerDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(detail.Binds) != 2 {
		t.Fatalf("expected 2 binds, got %d: %v", len(detail.Binds), detail.Binds)
	}
	bindsMap := make(map[string]bool)
	for _, b := range detail.Binds {
		bindsMap[b] = true
	}
	if !bindsMap["/data/a:/app/a:ro"] {
		t.Errorf("expected /data/a:/app/a:ro in binds, got %v", detail.Binds)
	}
	if !bindsMap["/data/b:/app/b"] {
		t.Errorf("expected /data/b:/app/b in binds, got %v", detail.Binds)
	}
}

// TestGetContainerInspectFailure verifies that Podman Inspect errors result in 500.
func TestGetContainerInspectFailure(t *testing.T) {
	origConnection := getContainerConnection
	origInspect := inspectPodmanContainer
	t.Cleanup(func() {
		getContainerConnection = origConnection
		inspectPodmanContainer = origInspect
	})

	getContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	inspectPodmanContainer = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return nil, errors.New("container not found")
	}

	req := httptest.NewRequest(http.MethodGet, "/get-container?id=missing-id", nil)
	w := httptest.NewRecorder()

	GetContainer(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// TestGetContainerLogsInvalidMethod verifies that non-GET requests are rejected with 405.
func TestGetContainerLogsInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/container-logs?id=abc123", nil)
	w := httptest.NewRecorder()

	GetContainerLogs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// TestGetContainerLogsMissingID verifies that a missing "id" query parameter returns 400.
func TestGetContainerLogsMissingID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/container-logs", nil)
	w := httptest.NewRecorder()

	GetContainerLogs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestGetContainerLogsInvalidTail verifies that an out-of-range tail parameter returns 400.
func TestGetContainerLogsInvalidTail(t *testing.T) {
	tests := []string{"0", "-1", "1001", "abc"}
	for _, tail := range tests {
		req := httptest.NewRequest(http.MethodGet, "/container-logs?id=abc123&tail="+tail, nil)
		w := httptest.NewRecorder()
		GetContainerLogs(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("tail=%q: expected 400, got %d", tail, w.Code)
		}
	}
}

// TestGetContainerLogsReturnLines verifies that GetContainerLogs merges
// stdout and stderr lines from Podman into a newline-separated body.
func TestGetContainerLogsReturnLines(t *testing.T) {
	origConnection := containerLogsConnection
	origLogs := podmanContainerLogs
	t.Cleanup(func() {
		containerLogsConnection = origConnection
		podmanContainerLogs = origLogs
	})

	containerLogsConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	podmanContainerLogs = func(ctx context.Context, nameOrID string, opts *containers.LogOptions, stdoutChan, stderrChan chan string) error {
		if nameOrID != "my-container" {
			t.Fatalf("expected container ID my-container, got %q", nameOrID)
		}
		stdoutChan <- "stdout line 1\n"
		stdoutChan <- "stdout line 2\n"
		stderrChan <- "stderr line 1\n"
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/container-logs?id=my-container&tail=50", nil)
	w := httptest.NewRecorder()

	GetContainerLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "stdout line 1") {
		t.Errorf("expected stdout line 1 in body, got: %q", body)
	}
	if !strings.Contains(body, "stdout line 2") {
		t.Errorf("expected stdout line 2 in body, got: %q", body)
	}
	if !strings.Contains(body, "stderr line 1") {
		t.Errorf("expected stderr line 1 in body, got: %q", body)
	}
}

// TestGetContainerLogsAllTail verifies that "tail=all" is accepted.
func TestGetContainerLogsAllTail(t *testing.T) {
	origConnection := containerLogsConnection
	origLogs := podmanContainerLogs
	t.Cleanup(func() {
		containerLogsConnection = origConnection
		podmanContainerLogs = origLogs
	})

	containerLogsConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	podmanContainerLogs = func(ctx context.Context, nameOrID string, opts *containers.LogOptions, stdoutChan, stderrChan chan string) error {
		stdoutChan <- "all logs\n"
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/container-logs?id=ctr&tail=all", nil)
	w := httptest.NewRecorder()

	GetContainerLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestGetContainerLogsFailure verifies that Podman Logs errors result in 500.
func TestGetContainerLogsFailure(t *testing.T) {
	origConnection := containerLogsConnection
	origLogs := podmanContainerLogs
	t.Cleanup(func() {
		containerLogsConnection = origConnection
		podmanContainerLogs = origLogs
	})

	containerLogsConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	podmanContainerLogs = func(ctx context.Context, nameOrID string, opts *containers.LogOptions, stdoutChan, stderrChan chan string) error {
		return errors.New("container not running")
	}

	req := httptest.NewRequest(http.MethodGet, "/container-logs?id=ctr", nil)
	w := httptest.NewRecorder()

	GetContainerLogs(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// TestUpdateContainerInvalidMethod verifies that non-POST requests are rejected with 405.
func TestUpdateContainerInvalidMethod(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/update-container", nil)
		w := httptest.NewRecorder()
		UpdateContainer(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Method %s: expected 405, got %d", method, w.Code)
		}
	}
}

// TestUpdateContainerInvalidJSON verifies that malformed JSON returns 400.
func TestUpdateContainerInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	UpdateContainer(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestUpdateContainerMissingContainerID verifies that a missing containerId returns 400.
func TestUpdateContainerMissingContainerID(t *testing.T) {
	body, _ := json.Marshal(UpdateContainerRequest{ContainerID: "", Image: "nginx:latest"})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	UpdateContainer(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestUpdateContainerMissingImage verifies that a missing image returns 400.
func TestUpdateContainerMissingImage(t *testing.T) {
	body, _ := json.Marshal(UpdateContainerRequest{ContainerID: "abc123", Image: ""})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	UpdateContainer(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestUpdateContainerInvalidImageName verifies that a dangerous image name returns 400.
func TestUpdateContainerInvalidImageName(t *testing.T) {
	cases := []string{"../etc/passwd", "/etc/passwd", "my image", "image\\name"}
	for _, img := range cases {
		body, _ := json.Marshal(UpdateContainerRequest{ContainerID: "abc123", Image: img})
		req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		UpdateContainer(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Image %q: expected 400, got %d", img, w.Code)
		}
	}
}

// TestUpdateContainerInvalidContainerName verifies that an invalid container name returns 400.
func TestUpdateContainerInvalidContainerName(t *testing.T) {
	cases := []string{"-bad", ".bad", "bad name", "bad/name"}
	for _, name := range cases {
		body, _ := json.Marshal(UpdateContainerRequest{ContainerID: "abc123", Image: "nginx:latest", Name: name})
		req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		UpdateContainer(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Container name %q: expected 400, got %d", name, w.Code)
		}
	}
}

// TestUpdateContainerInvalidPort verifies that an invalid port mapping returns 400.
func TestUpdateContainerInvalidPort(t *testing.T) {
	cases := []string{"nocodon", "../80:80", "8080;80"}
	for _, port := range cases {
		body, _ := json.Marshal(UpdateContainerRequest{ContainerID: "abc123", Image: "nginx:latest", Ports: []string{port}})
		req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		UpdateContainer(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Port %q: expected 400, got %d", port, w.Code)
		}
	}
}

// TestUpdateContainerInvalidVolume verifies that a volume with path traversal returns 400.
func TestUpdateContainerInvalidVolume(t *testing.T) {
	cases := []string{"../../etc:/data", "/data/../../etc:/data", "/host", "/host:../container"}
	for _, vol := range cases {
		body, _ := json.Marshal(UpdateContainerRequest{ContainerID: "abc123", Image: "nginx:latest", Volumes: []string{vol}})
		req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		UpdateContainer(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Volume %q: expected 400, got %d", vol, w.Code)
		}
	}
}

// TestUpdateContainerInvalidRestartPolicy verifies that an unknown restart policy returns 400.
func TestUpdateContainerInvalidRestartPolicy(t *testing.T) {
	body, _ := json.Marshal(UpdateContainerRequest{ContainerID: "abc123", Image: "nginx:latest", RestartPolicy: "invalid-policy"})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	UpdateContainer(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestUpdateContainerAutoRemoveWithRestartPolicy verifies that combining autoRemove
// with a non-"no" restart policy returns 400.
func TestUpdateContainerAutoRemoveWithRestartPolicy(t *testing.T) {
	for _, policy := range []string{"always", "on-failure", "unless-stopped"} {
		body, _ := json.Marshal(UpdateContainerRequest{
			ContainerID:   "abc123",
			Image:         "nginx:latest",
			AutoRemove:    true,
			RestartPolicy: policy,
		})
		req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		UpdateContainer(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Policy %q with autoRemove: expected 400, got %d", policy, w.Code)
		}
	}
}

// TestUpdateContainerStopsRemovesAndRecreates verifies that UpdateContainer calls Podman to
// stop, remove, recreate, and start the container with the new configuration.
func TestUpdateContainerStopsRemovesAndRecreates(t *testing.T) {
	origConnection := updateContainerConnection
	origInspect := updateContainerInspect
	origStop := updateContainerStop
	origRemove := updateContainerRemove
	origEnsureImage := updateContainerEnsureImage
	origCreate := updateContainerCreateWithSpec
	origStart := updateContainerStart
	t.Cleanup(func() {
		updateContainerConnection = origConnection
		updateContainerInspect = origInspect
		updateContainerStop = origStop
		updateContainerRemove = origRemove
		updateContainerEnsureImage = origEnsureImage
		updateContainerCreateWithSpec = origCreate
		updateContainerStart = origStart
	})

	updateContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}

	inspectCalled := false
	updateContainerInspect = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		inspectCalled = true
		return &define.InspectContainerData{
			ID:        "old-container-id",
			ImageName: "nginx:latest",
			State: &define.InspectContainerState{
				Status: "running",
			},
		}, nil
	}

	stopCalled := false
	updateContainerStop = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error {
		stopCalled = true
		if nameOrID != "old-container-id" {
			t.Errorf("expected stop called with old-container-id, got %q", nameOrID)
		}
		return nil
	}

	removeCalled := 0
	updateContainerRemove = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		removeCalled++
		return nil, nil
	}

	updateContainerEnsureImage = func(ctx context.Context, ref string) (string, error) {
		return ref, nil
	}

	createCalled := false
	updateContainerCreateWithSpec = func(ctx context.Context, s *specgen.SpecGenerator, opts *containers.CreateOptions) (podmanTypes.ContainerCreateResponse, error) {
		createCalled = true
		return podmanTypes.ContainerCreateResponse{ID: "new-container-id"}, nil
	}

	startCalled := false
	updateContainerStart = func(ctx context.Context, nameOrID string, opts *containers.StartOptions) error {
		startCalled = true
		if nameOrID != "new-container-id" {
			t.Errorf("expected start called with new-container-id, got %q", nameOrID)
		}
		return nil
	}

	body, _ := json.Marshal(UpdateContainerRequest{
		ContainerID:   "old-container-id",
		Image:         "nginx:latest",
		Name:          "updated-container",
		Ports:         []string{"8080:80"},
		Env:           []string{"FOO=bar"},
		RestartPolicy: "no",
	})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	UpdateContainer(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	if !inspectCalled {
		t.Error("expected Podman Inspect to be called")
	}
	if !stopCalled {
		t.Error("expected Podman Stop to be called for running container")
	}
	if removeCalled == 0 {
		t.Error("expected Podman Remove to be called")
	}
	if !createCalled {
		t.Error("expected Podman CreateWithSpec to be called")
	}
	if !startCalled {
		t.Error("expected Podman Start to be called")
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status success, got %q", resp["status"])
	}
	if resp["containerId"] != "new-container-id" {
		t.Errorf("expected containerId new-container-id, got %q", resp["containerId"])
	}
}

// TestUpdateContainerStopFailureIsNonFatal verifies that a stop error does not abort the update.
// The handler should log the failure and proceed to force-remove and recreate the container.
func TestUpdateContainerStopFailureIsNonFatal(t *testing.T) {
	origConnection := updateContainerConnection
	origInspect := updateContainerInspect
	origStop := updateContainerStop
	origRemove := updateContainerRemove
	origEnsureImage := updateContainerEnsureImage
	origCreate := updateContainerCreateWithSpec
	origStart := updateContainerStart
	t.Cleanup(func() {
		updateContainerConnection = origConnection
		updateContainerInspect = origInspect
		updateContainerStop = origStop
		updateContainerRemove = origRemove
		updateContainerEnsureImage = origEnsureImage
		updateContainerCreateWithSpec = origCreate
		updateContainerStart = origStart
	})

	updateContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	updateContainerInspect = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "running-container-id",
			ImageName: "nginx:latest",
			State: &define.InspectContainerState{
				Status: "running",
			},
		}, nil
	}

	// Stop returns an error – this must not abort the update.
	stopCalled := false
	updateContainerStop = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error {
		stopCalled = true
		return fmt.Errorf("stop timed out")
	}

	removeCalled := false
	updateContainerRemove = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		removeCalled = true
		return nil, nil
	}
	updateContainerEnsureImage = func(ctx context.Context, ref string) (string, error) {
		return ref, nil
	}
	updateContainerCreateWithSpec = func(ctx context.Context, s *specgen.SpecGenerator, opts *containers.CreateOptions) (podmanTypes.ContainerCreateResponse, error) {
		return podmanTypes.ContainerCreateResponse{ID: "new-container-id"}, nil
	}
	updateContainerStart = func(ctx context.Context, nameOrID string, opts *containers.StartOptions) error {
		return nil
	}

	body, _ := json.Marshal(UpdateContainerRequest{
		ContainerID: "running-container-id",
		Image:       "nginx:latest",
		Name:        "my-container",
	})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	UpdateContainer(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even when stop fails, got %d; body: %s", w.Code, w.Body.String())
	}
	if !stopCalled {
		t.Error("expected Stop to be attempted")
	}
	if !removeCalled {
		t.Error("expected Remove to be called even after stop failure")
	}
}

// TestUpdateContainerSkipsStopWhenNotRunning verifies that a stopped container is not stopped again.
func TestUpdateContainerSkipsStopWhenNotRunning(t *testing.T) {
	origConnection := updateContainerConnection
	origInspect := updateContainerInspect
	origStop := updateContainerStop
	origRemove := updateContainerRemove
	origEnsureImage := updateContainerEnsureImage
	origCreate := updateContainerCreateWithSpec
	origStart := updateContainerStart
	t.Cleanup(func() {
		updateContainerConnection = origConnection
		updateContainerInspect = origInspect
		updateContainerStop = origStop
		updateContainerRemove = origRemove
		updateContainerEnsureImage = origEnsureImage
		updateContainerCreateWithSpec = origCreate
		updateContainerStart = origStart
	})

	updateContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	updateContainerInspect = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "stopped-container",
			ImageName: "nginx:latest",
			State: &define.InspectContainerState{
				Status: "exited",
			},
		}, nil
	}
	stopCalled := false
	updateContainerStop = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error {
		stopCalled = true
		return nil
	}
	updateContainerRemove = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		return nil, nil
	}
	updateContainerEnsureImage = func(ctx context.Context, ref string) (string, error) {
		return ref, nil
	}
	updateContainerCreateWithSpec = func(ctx context.Context, s *specgen.SpecGenerator, opts *containers.CreateOptions) (podmanTypes.ContainerCreateResponse, error) {
		return podmanTypes.ContainerCreateResponse{ID: "new-id"}, nil
	}
	updateContainerStart = func(ctx context.Context, nameOrID string, opts *containers.StartOptions) error {
		return nil
	}

	body, _ := json.Marshal(UpdateContainerRequest{ContainerID: "stopped-container", Image: "nginx:latest"})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	UpdateContainer(w, req)

	if stopCalled {
		t.Error("expected Podman Stop NOT to be called for a stopped container")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestUpdateContainerRollsBackWhenImageResolveFails verifies that if pulling/resolving
// the new image fails after the old container has been removed, the original container
// is recreated (best-effort rollback) from the captured configuration.
func TestUpdateContainerRollsBackWhenImageResolveFails(t *testing.T) {
	origConnection := updateContainerConnection
	origInspect := updateContainerInspect
	origStop := updateContainerStop
	origRemove := updateContainerRemove
	origEnsureImage := updateContainerEnsureImage
	origCreate := updateContainerCreateWithSpec
	origStart := updateContainerStart
	t.Cleanup(func() {
		updateContainerConnection = origConnection
		updateContainerInspect = origInspect
		updateContainerStop = origStop
		updateContainerRemove = origRemove
		updateContainerEnsureImage = origEnsureImage
		updateContainerCreateWithSpec = origCreate
		updateContainerStart = origStart
	})

	updateContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	updateContainerInspect = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "old-container-id",
			Name:      "/my-container",
			ImageName: "nginx:1.25",
			State:     &define.InspectContainerState{Status: "running"},
			Config: &define.InspectContainerConfig{
				Env:    []string{"FOO=bar"},
				Labels: map[string]string{"opencloud/name": "my-container"},
			},
			HostConfig: &define.InspectContainerHostConfig{
				RestartPolicy: &define.InspectRestartPolicy{Name: "always"},
			},
		}, nil
	}
	updateContainerStop = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error { return nil }
	updateContainerRemove = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		return nil, nil
	}
	// New image resolution fails; old image (rollback) succeeds.
	updateContainerEnsureImage = func(ctx context.Context, ref string) (string, error) {
		if ref == "nginx:bad-tag" {
			return "", fmt.Errorf("image not found: %s", ref)
		}
		return ref, nil
	}

	var rollbackCreateSpec *specgen.SpecGenerator
	updateContainerCreateWithSpec = func(ctx context.Context, s *specgen.SpecGenerator, opts *containers.CreateOptions) (podmanTypes.ContainerCreateResponse, error) {
		rollbackCreateSpec = s
		return podmanTypes.ContainerCreateResponse{ID: "restored-container-id"}, nil
	}
	rollbackStartCalled := false
	updateContainerStart = func(ctx context.Context, nameOrID string, opts *containers.StartOptions) error {
		rollbackStartCalled = true
		return nil
	}

	body, _ := json.Marshal(UpdateContainerRequest{
		ContainerID: "old-container-id",
		Image:       "nginx:bad-tag",
		Name:        "my-container",
	})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	UpdateContainer(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on image resolve failure, got %d", w.Code)
	}
	if rollbackCreateSpec == nil {
		t.Fatal("expected rollback create to be called after image resolve failure")
	}
	if rollbackCreateSpec.Image != "nginx:1.25" {
		t.Errorf("expected rollback to use old image nginx:1.25, got %q", rollbackCreateSpec.Image)
	}
	if rollbackCreateSpec.Name != "my-container" {
		t.Errorf("expected rollback container name my-container, got %q", rollbackCreateSpec.Name)
	}
	if !rollbackStartCalled {
		t.Error("expected rollback container to be started (original was running)")
	}
}

// TestUpdateContainerRollsBackWhenCreateFails verifies that if creating the new container
// fails after the old container has been removed, the original container is recreated
// (best-effort rollback).
func TestUpdateContainerRollsBackWhenCreateFails(t *testing.T) {
	origConnection := updateContainerConnection
	origInspect := updateContainerInspect
	origStop := updateContainerStop
	origRemove := updateContainerRemove
	origEnsureImage := updateContainerEnsureImage
	origCreate := updateContainerCreateWithSpec
	origStart := updateContainerStart
	t.Cleanup(func() {
		updateContainerConnection = origConnection
		updateContainerInspect = origInspect
		updateContainerStop = origStop
		updateContainerRemove = origRemove
		updateContainerEnsureImage = origEnsureImage
		updateContainerCreateWithSpec = origCreate
		updateContainerStart = origStart
	})

	updateContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	updateContainerInspect = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "old-container-id",
			Name:      "/my-container",
			ImageName: "nginx:1.25",
			State:     &define.InspectContainerState{Status: "exited"},
			Config: &define.InspectContainerConfig{
				Labels: map[string]string{"opencloud/name": "my-container"},
			},
		}, nil
	}
	updateContainerStop = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error { return nil }
	updateContainerRemove = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		return nil, nil
	}
	updateContainerEnsureImage = func(ctx context.Context, ref string) (string, error) { return ref, nil }

	// First call (new container create) fails; second call (rollback) succeeds.
	createCallCount := 0
	var rollbackCreateSpec *specgen.SpecGenerator
	updateContainerCreateWithSpec = func(ctx context.Context, s *specgen.SpecGenerator, opts *containers.CreateOptions) (podmanTypes.ContainerCreateResponse, error) {
		createCallCount++
		if createCallCount == 1 {
			return podmanTypes.ContainerCreateResponse{}, fmt.Errorf("port already in use")
		}
		rollbackCreateSpec = s
		return podmanTypes.ContainerCreateResponse{ID: "restored-container-id"}, nil
	}
	updateContainerStart = func(ctx context.Context, nameOrID string, opts *containers.StartOptions) error { return nil }

	body, _ := json.Marshal(UpdateContainerRequest{
		ContainerID: "old-container-id",
		Image:       "nginx:latest",
		Name:        "my-container",
	})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	UpdateContainer(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on create failure, got %d", w.Code)
	}
	if rollbackCreateSpec == nil {
		t.Fatal("expected rollback create to be called after container create failure")
	}
	if rollbackCreateSpec.Image != "nginx:1.25" {
		t.Errorf("expected rollback to use old image nginx:1.25, got %q", rollbackCreateSpec.Image)
	}
}

// TestUpdateContainerRollsBackWhenStartFails verifies that if starting the new container
// fails, the new container is cleaned up and the original container is recreated
// (best-effort rollback).
func TestUpdateContainerRollsBackWhenStartFails(t *testing.T) {
	origConnection := updateContainerConnection
	origInspect := updateContainerInspect
	origStop := updateContainerStop
	origRemove := updateContainerRemove
	origEnsureImage := updateContainerEnsureImage
	origCreate := updateContainerCreateWithSpec
	origStart := updateContainerStart
	t.Cleanup(func() {
		updateContainerConnection = origConnection
		updateContainerInspect = origInspect
		updateContainerStop = origStop
		updateContainerRemove = origRemove
		updateContainerEnsureImage = origEnsureImage
		updateContainerCreateWithSpec = origCreate
		updateContainerStart = origStart
	})

	updateContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	updateContainerInspect = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "old-container-id",
			Name:      "/my-container",
			ImageName: "nginx:1.25",
			State:     &define.InspectContainerState{Status: "running"},
			Config: &define.InspectContainerConfig{
				Env:    []string{"FOO=bar"},
				Labels: map[string]string{"opencloud/name": "my-container"},
			},
			HostConfig: &define.InspectContainerHostConfig{
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}
	updateContainerStop = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error { return nil }

	removedIDs := []string{}
	updateContainerRemove = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		removedIDs = append(removedIDs, nameOrID)
		return nil, nil
	}
	updateContainerEnsureImage = func(ctx context.Context, ref string) (string, error) { return ref, nil }

	// New container is created but fails to start.
	createCallCount := 0
	var rollbackCreateSpec *specgen.SpecGenerator
	updateContainerCreateWithSpec = func(ctx context.Context, s *specgen.SpecGenerator, opts *containers.CreateOptions) (podmanTypes.ContainerCreateResponse, error) {
		createCallCount++
		if createCallCount == 1 {
			return podmanTypes.ContainerCreateResponse{ID: "new-container-id"}, nil
		}
		rollbackCreateSpec = s
		return podmanTypes.ContainerCreateResponse{ID: "restored-container-id"}, nil
	}

	startCallCount := 0
	rollbackStartID := ""
	updateContainerStart = func(ctx context.Context, nameOrID string, opts *containers.StartOptions) error {
		startCallCount++
		if startCallCount == 1 {
			return fmt.Errorf("container failed to start: OCI error")
		}
		rollbackStartID = nameOrID
		return nil
	}

	body, _ := json.Marshal(UpdateContainerRequest{
		ContainerID: "old-container-id",
		Image:       "nginx:latest",
		Name:        "my-container",
	})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	UpdateContainer(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on start failure, got %d", w.Code)
	}

	// The failed new container must be removed.
	foundNewRemoved := false
	for _, id := range removedIDs {
		if id == "new-container-id" {
			foundNewRemoved = true
		}
	}
	if !foundNewRemoved {
		t.Errorf("expected new-container-id to be removed on start failure; removedIDs=%v", removedIDs)
	}

	if rollbackCreateSpec == nil {
		t.Fatal("expected rollback create to be called after start failure")
	}
	if rollbackCreateSpec.Image != "nginx:1.25" {
		t.Errorf("expected rollback image nginx:1.25, got %q", rollbackCreateSpec.Image)
	}
	if rollbackStartID != "restored-container-id" {
		t.Errorf("expected rollback container restored-container-id to be started, got %q", rollbackStartID)
	}
}

// TestUpdateContainerRollsBackUsingHostConfigFallback verifies that when the
// opencloud/volumes and opencloud/ports labels are absent, the rollback correctly
// falls back to HostConfig.Binds and HostConfig.PortBindings to reconstruct the
// original container spec.
func TestUpdateContainerRollsBackUsingHostConfigFallback(t *testing.T) {
	origConnection := updateContainerConnection
	origInspect := updateContainerInspect
	origStop := updateContainerStop
	origRemove := updateContainerRemove
	origEnsureImage := updateContainerEnsureImage
	origCreate := updateContainerCreateWithSpec
	origStart := updateContainerStart
	t.Cleanup(func() {
		updateContainerConnection = origConnection
		updateContainerInspect = origInspect
		updateContainerStop = origStop
		updateContainerRemove = origRemove
		updateContainerEnsureImage = origEnsureImage
		updateContainerCreateWithSpec = origCreate
		updateContainerStart = origStart
	})

	updateContainerConnection = func(ctx context.Context) (context.Context, error) {
		return ctx, nil
	}
	// No opencloud/volumes or opencloud/ports labels – forces HostConfig fallback.
	updateContainerInspect = func(ctx context.Context, nameOrID string, opts *containers.InspectOptions) (*define.InspectContainerData, error) {
		return &define.InspectContainerData{
			ID:        "old-container-id",
			Name:      "/my-container",
			ImageName: "nginx:1.25",
			State:     &define.InspectContainerState{Status: "running"},
			Config: &define.InspectContainerConfig{
				Labels: map[string]string{"opencloud/name": "my-container"},
			},
			HostConfig: &define.InspectContainerHostConfig{
				Binds: []string{"/host/data:/container/data:ro"},
				PortBindings: map[string][]define.InspectHostPort{
					"80/tcp": {{HostIP: "", HostPort: "8080"}},
				},
				RestartPolicy: &define.InspectRestartPolicy{Name: "no"},
			},
		}, nil
	}
	updateContainerStop = func(ctx context.Context, nameOrID string, opts *containers.StopOptions) error { return nil }
	updateContainerRemove = func(ctx context.Context, nameOrID string, opts *containers.RemoveOptions) ([]*reports.RmReport, error) {
		return nil, nil
	}
	// New image resolution fails to force the rollback path.
	updateContainerEnsureImage = func(ctx context.Context, ref string) (string, error) {
		if ref == "nginx:bad-tag" {
			return "", fmt.Errorf("image not found: %s", ref)
		}
		return ref, nil
	}

	var rollbackCreateSpec *specgen.SpecGenerator
	updateContainerCreateWithSpec = func(ctx context.Context, s *specgen.SpecGenerator, opts *containers.CreateOptions) (podmanTypes.ContainerCreateResponse, error) {
		rollbackCreateSpec = s
		return podmanTypes.ContainerCreateResponse{ID: "restored-container-id"}, nil
	}
	updateContainerStart = func(ctx context.Context, nameOrID string, opts *containers.StartOptions) error { return nil }

	body, _ := json.Marshal(UpdateContainerRequest{
		ContainerID: "old-container-id",
		Image:       "nginx:bad-tag",
		Name:        "my-container",
	})
	req := httptest.NewRequest(http.MethodPost, "/update-container", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	UpdateContainer(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on image resolve failure, got %d", w.Code)
	}
	if rollbackCreateSpec == nil {
		t.Fatal("expected rollback create to be called")
	}
	// Verify port mapping was recovered from HostConfig.PortBindings fallback.
	if len(rollbackCreateSpec.PortMappings) != 1 {
		t.Fatalf("expected 1 port mapping in rollback spec, got %d", len(rollbackCreateSpec.PortMappings))
	}
	if rollbackCreateSpec.PortMappings[0].HostPort != 8080 || rollbackCreateSpec.PortMappings[0].ContainerPort != 80 {
		t.Errorf("unexpected rollback port mapping: %+v", rollbackCreateSpec.PortMappings[0])
	}
	// Verify volume was recovered from HostConfig.Binds fallback.
	if len(rollbackCreateSpec.Mounts) != 1 {
		t.Fatalf("expected 1 mount in rollback spec, got %d; volumes=%v", len(rollbackCreateSpec.Mounts), rollbackCreateSpec.Volumes)
	}
}

// TestShellSplit verifies that shellSplit correctly tokenises shell-like strings.
func TestShellSplit(t *testing.T) {
	tests := []struct {
		input   string
		want    []string
		wantErr bool
	}{
		{`nginx:latest`, []string{"nginx:latest"}, false},
		{`-p 8080:80 nginx:latest`, []string{"-p", "8080:80", "nginx:latest"}, false},
		{`-e "FOO=hello world" nginx:latest`, []string{"-e", "FOO=hello world", "nginx:latest"}, false},
		{`-e 'FOO=hello world' nginx:latest`, []string{"-e", "FOO=hello world", "nginx:latest"}, false},
		{`-v /host/path:/container/path:Z,U nginx:latest`, []string{"-v", "/host/path:/container/path:Z,U", "nginx:latest"}, false},
		{`--name my-app nginx:latest /bin/sh`, []string{"--name", "my-app", "nginx:latest", "/bin/sh"}, false},
		{``, nil, false},
		{`'unterminated`, nil, true},
		{`"unterminated`, nil, true},
		{`trailing\`, nil, true},
	}

	for _, tt := range tests {
		got, err := shellSplit(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("shellSplit(%q): expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("shellSplit(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("shellSplit(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("shellSplit(%q) token[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

// TestParseRawContainerArgs exercises the custom-command parser with various flag combinations.
func TestParseRawContainerArgs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PullAndRunRequest
		wantErr bool
	}{
		{
			name:  "image only",
			input: "nginx:latest",
			want:  PullAndRunRequest{Image: "nginx:latest"},
		},
		{
			name:  "port flags space-separated",
			input: "-p 8080:80 -p 443:443 nginx:latest",
			want:  PullAndRunRequest{Image: "nginx:latest", Ports: []string{"8080:80", "443:443"}},
		},
		{
			name:  "port flags equals-separated",
			input: "--publish=8080:80 nginx:latest",
			want:  PullAndRunRequest{Image: "nginx:latest", Ports: []string{"8080:80"}},
		},
		{
			name:  "env flags",
			input: `-e FOO=bar -e "BAZ=hello world" nginx:latest`,
			want:  PullAndRunRequest{Image: "nginx:latest", Env: []string{"FOO=bar", "BAZ=hello world"}},
		},
		{
			name:  "volume with Z,U options",
			input: "-v ~/logs:/var/log:Z,U nginx:latest",
			want:  PullAndRunRequest{Image: "nginx:latest", Volumes: []string{"~/logs:/var/log:Z,U"}},
		},
		{
			name:  "container name via --name",
			input: "--name my-app nginx:latest",
			want:  PullAndRunRequest{Image: "nginx:latest", Name: "my-app"},
		},
		{
			name:  "restart policy",
			input: "--restart always nginx:latest",
			want:  PullAndRunRequest{Image: "nginx:latest", RestartPolicy: "always"},
		},
		{
			name:  "auto-remove flag",
			input: "--rm nginx:latest",
			want:  PullAndRunRequest{Image: "nginx:latest", AutoRemove: true},
		},
		{
			name:  "detach flag is ignored (always detached via bindings)",
			input: "-d nginx:latest",
			want:  PullAndRunRequest{Image: "nginx:latest"},
		},
		{
			name:  "command override after image",
			input: "nginx:latest /bin/sh -c echo",
			want:  PullAndRunRequest{Image: "nginx:latest", Command: "/bin/sh -c echo"},
		},
		{
			name:  "issue example: rabbitmq",
			input: `-p 15672:15672 -p 5672:5672 -e RABBITMQ_LOGS=/var/log/rabbitmq/rabbit.log -v ~/rabbitlogs/:/var/log/rabbitmq:Z,U docker.io/library/rabbitmq:management`,
			want: PullAndRunRequest{
				Image:   "docker.io/library/rabbitmq:management",
				Ports:   []string{"15672:15672", "5672:5672"},
				Env:     []string{"RABBITMQ_LOGS=/var/log/rabbitmq/rabbit.log"},
				Volumes: []string{"~/rabbitlogs/:/var/log/rabbitmq:Z,U"},
			},
		},
		{
			name:    "missing image returns error",
			input:   "-p 8080:80",
			wantErr: true,
		},
		{
			name:    "empty string returns error",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only flags and no image returns error",
			input:   "--rm --restart always",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRawContainerArgs(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseRawContainerArgs(%q): expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseRawContainerArgs(%q): unexpected error: %v", tt.input, err)
				return
			}
			if got.Image != tt.want.Image {
				t.Errorf("Image: got %q, want %q", got.Image, tt.want.Image)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name: got %q, want %q", got.Name, tt.want.Name)
			}
			if got.RestartPolicy != tt.want.RestartPolicy {
				t.Errorf("RestartPolicy: got %q, want %q", got.RestartPolicy, tt.want.RestartPolicy)
			}
			if got.AutoRemove != tt.want.AutoRemove {
				t.Errorf("AutoRemove: got %v, want %v", got.AutoRemove, tt.want.AutoRemove)
			}
			if got.Command != tt.want.Command {
				t.Errorf("Command: got %q, want %q", got.Command, tt.want.Command)
			}
			checkSlice := func(field string, a, b []string) {
				t.Helper()
				if len(a) != len(b) {
					t.Errorf("%s: got %v, want %v", field, a, b)
					return
				}
				for i := range a {
					if a[i] != b[i] {
						t.Errorf("%s[%d]: got %q, want %q", field, i, a[i], b[i])
					}
				}
			}
			checkSlice("Ports", got.Ports, tt.want.Ports)
			checkSlice("Env", got.Env, tt.want.Env)
			checkSlice("Volumes", got.Volumes, tt.want.Volumes)
		})
	}
}

// TestPullAndRunStreamHandlerFullCustomCommand verifies that the fullCustomCommand field
// is parsed and validated correctly by PullAndRunStream.
func TestPullAndRunStreamHandlerFullCustomCommand(t *testing.T) {
	// Valid custom command with a well-formed image.
	t.Run("valid custom command", func(t *testing.T) {
		body, _ := json.Marshal(PullAndRunRequest{
			FullCustomCommand: "-p 8080:80 -e FOO=bar nginx:latest",
		})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Errorf("Expected 200 or 500, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Invalid custom command (no image) must return 400 before SSE headers are set.
	t.Run("custom command without image", func(t *testing.T) {
		body, _ := json.Marshal(PullAndRunRequest{
			FullCustomCommand: "-p 8080:80 --rm",
		})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Malformed custom command (unterminated quote) must return 400.
	t.Run("malformed custom command", func(t *testing.T) {
		body, _ := json.Marshal(PullAndRunRequest{
			FullCustomCommand: `-e "unterminated nginx:latest`,
		})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Custom command with an invalid image name must return 400.
	t.Run("custom command with invalid image name", func(t *testing.T) {
		body, _ := json.Marshal(PullAndRunRequest{
			FullCustomCommand: "../etc/passwd",
		})
		req := httptest.NewRequest(http.MethodPost, "/pull-and-run-stream", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		PullAndRunStream(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestApplyFullCustomCommand verifies that applyFullCustomCommand populates
// and trims request fields correctly, and is a no-op when the field is empty.
func TestApplyFullCustomCommand(t *testing.T) {
	t.Run("no-op on empty FullCustomCommand", func(t *testing.T) {
		req := PullAndRunRequest{Image: "nginx:latest", Name: "original"}
		if err := applyFullCustomCommand(&req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Image != "nginx:latest" || req.Name != "original" {
			t.Errorf("fields changed unexpectedly: %+v", req)
		}
	})

	t.Run("parses and trims whitespace in image", func(t *testing.T) {
		req := PullAndRunRequest{
			// The parsed image might have leading/trailing whitespace if the command does.
			FullCustomCommand: "  nginx:latest  ",
		}
		// shellSplit trims tokens, so the image should not have spaces.
		if err := applyFullCustomCommand(&req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Image != "nginx:latest" {
			t.Errorf("Image: got %q, want \"nginx:latest\"", req.Image)
		}
	})

	t.Run("returns error for invalid command", func(t *testing.T) {
		req := PullAndRunRequest{FullCustomCommand: `"unterminated`}
		if err := applyFullCustomCommand(&req); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("overwrites pre-existing fields", func(t *testing.T) {
		req := PullAndRunRequest{
			Image:             "old-image:latest",
			Name:              "old-name",
			FullCustomCommand: "--name new-name -p 9090:9090 new-image:latest",
		}
		if err := applyFullCustomCommand(&req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Image != "new-image:latest" {
			t.Errorf("Image: got %q", req.Image)
		}
		if req.Name != "new-name" {
			t.Errorf("Name: got %q", req.Name)
		}
		if len(req.Ports) != 1 || req.Ports[0] != "9090:9090" {
			t.Errorf("Ports: got %v", req.Ports)
		}
	})
}
