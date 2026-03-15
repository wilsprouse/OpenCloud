package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
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
		filepath.Join(home, ".opencloud", "user"),
	}

	// Create all directories with appropriate permissions
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Initialize the credentials file with default admin credentials if it
	// does not already exist.
	if err := initializeCredentials(home); err != nil {
		return fmt.Errorf("failed to initialize credentials: %w", err)
	}

	return nil
}

// initializeCredentials creates ~/.opencloud/user/credentials with a default
// admin account when the file does not yet exist.  Each line in the file has
// the form "username:bcrypt_hash".
func initializeCredentials(home string) error {
	credPath := filepath.Join(home, ".opencloud", "user", "credentials")

	// If the credentials file already exists, leave it unchanged.
	if _, err := os.Stat(credPath); err == nil {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash default password: %w", err)
	}

	content := fmt.Sprintf("admin:%s\n", hash)
	if err := os.WriteFile(credPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}
