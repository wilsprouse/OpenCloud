package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// InitializeOpenCloudDirectories creates the necessary .opencloud directory structure
// in the user's home directory if it doesn't already exist
func InitializeOpenCloudDirectories() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Define all required directories
	directories := []string{
		filepath.Join(home, ".opencloud"),
		filepath.Join(home, ".opencloud", "functions"),
		filepath.Join(home, ".opencloud", "pipelines"),
		filepath.Join(home, ".opencloud", "blob_storage"),
		filepath.Join(home, ".opencloud", "logs"),
		filepath.Join(home, ".opencloud", "logs", "functions"),
		filepath.Join(home, ".opencloud", "logs", "pipelines"),
	}

	// Create all directories with appropriate permissions
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
