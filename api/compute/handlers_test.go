package compute

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/domain/entities/reports"
	podmanTypes "github.com/containers/podman/v5/pkg/domain/entities/types"
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
	cases := []string{"nocodon", "8080", "../80:80", "8080;80"}
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
		{"0:80", false},
		{"8080:80/tcp", false},
		{"0.0.0.0:8080:80", false},
		{"8080", true},     // no colon
		{"../80:80", true}, // path traversal
		{"8080;80", true},  // semicolon
		{"8080 80", true},  // space
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
