package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeOpenCloudDirectories(t *testing.T) {
	// Setup: Create a temporary home directory
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Call InitializeOpenCloudDirectories
	err := InitializeOpenCloudDirectories()
	if err != nil {
		t.Fatalf("InitializeOpenCloudDirectories failed: %v", err)
	}

	// Verify all expected directories were created
	expectedDirs := []string{
		filepath.Join(tmpHome, ".opencloud"),
		filepath.Join(tmpHome, ".opencloud", "functions"),
		filepath.Join(tmpHome, ".opencloud", "pipelines"),
		filepath.Join(tmpHome, ".opencloud", "blob_storage"),
		filepath.Join(tmpHome, ".opencloud", "logs"),
		filepath.Join(tmpHome, ".opencloud", "logs", "functions"),
		filepath.Join(tmpHome, ".opencloud", "logs", "pipelines"),
	}

	for _, dir := range expectedDirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("Directory %s was not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s exists but is not a directory", dir)
		}
	}
}

func TestInitializeOpenCloudDirectoriesIdempotent(t *testing.T) {
	// Setup: Create a temporary home directory
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Call InitializeOpenCloudDirectories multiple times
	for i := 0; i < 3; i++ {
		err := InitializeOpenCloudDirectories()
		if err != nil {
			t.Fatalf("InitializeOpenCloudDirectories failed on iteration %d: %v", i, err)
		}
	}

	// Verify directories still exist after multiple calls
	opencloudDir := filepath.Join(tmpHome, ".opencloud")
	info, err := os.Stat(opencloudDir)
	if err != nil {
		t.Fatalf("Directory %s does not exist after multiple initializations: %v", opencloudDir, err)
	}
	if !info.IsDir() {
		t.Errorf("%s exists but is not a directory", opencloudDir)
	}
}

func TestInitializeOpenCloudDirectoriesWithExistingFiles(t *testing.T) {
	// Setup: Create a temporary home directory with some existing content
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create .opencloud directory and add a test file
	opencloudDir := filepath.Join(tmpHome, ".opencloud")
	if err := os.MkdirAll(opencloudDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	testFile := filepath.Join(opencloudDir, "test.txt")
	testContent := []byte("test content")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Call InitializeOpenCloudDirectories
	err := InitializeOpenCloudDirectories()
	if err != nil {
		t.Fatalf("InitializeOpenCloudDirectories failed: %v", err)
	}

	// Verify existing file was not affected
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Test file was removed or cannot be read: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("Test file content was modified. Expected %q, got %q", testContent, content)
	}

	// Verify all subdirectories were created
	functionsDir := filepath.Join(opencloudDir, "functions")
	if _, err := os.Stat(functionsDir); os.IsNotExist(err) {
		t.Errorf("Subdirectory %s was not created", functionsDir)
	}
}
