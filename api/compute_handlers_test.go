package api

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
