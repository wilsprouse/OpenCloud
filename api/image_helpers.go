package api

import (
	"regexp"
	"strings"
	"time"
)

const BuildTimeout = 5 * time.Minute

// Pre-compiled regex patterns for image name validation
var (
	// imageNamePatternLower matches lowercase-only image names (after normalization)
	imageNamePatternLower = regexp.MustCompile(`^[a-z0-9]+(([._-]|__)[a-z0-9]+)*(:[a-z0-9]+(([._-]|__)[a-z0-9]+)*)*(/[a-z0-9]+(([._-]|__)[a-z0-9]+)*(:[a-z0-9]+(([._-]|__)[a-z0-9]+)*)*)*(@sha256:[a-f0-9]{64})?$`)
	// imageNamePatternMixed matches mixed-case image names
	imageNamePatternMixed = regexp.MustCompile(`^[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*(:[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*)*(/[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*(:[a-zA-Z0-9]+(([._-]|__)[a-zA-Z0-9]+)*)*)*$`)
)

// normalizeImageRef adds docker.io registry prefix if no registry is specified
func normalizeImageRef(imageRef string) string {
	if strings.Contains(imageRef, "/") {
		return imageRef
	}
	parts := strings.Split(imageRef, ":")
	if strings.Contains(parts[0], ".") {
		return imageRef
	}
	return "docker.io/library/" + imageRef
}

// NormalizeImageRef is the exported wrapper for normalizeImageRef.
func NormalizeImageRef(imageRef string) string {
	return normalizeImageRef(imageRef)
}

// validateImageName checks an image name for dangerous or invalid patterns.
// Returns an error string if invalid, or empty string if valid.
func validateImageName(name string) string {
	// Reject backslashes
	if strings.Contains(name, `\`) {
		return "image name must not contain backslashes"
	}
	// Reject names starting with a slash (absolute paths)
	if strings.HasPrefix(name, "/") {
		return "image name must not start with a slash"
	}
	// Reject path traversal attempts
	if strings.Contains(name, "..") {
		return "image name must not contain path traversal sequences"
	}
	// Reject double slashes
	if strings.Contains(name, "//") {
		return "image name must not contain consecutive slashes"
	}
	// Validate against the allowed character pattern
	if !imageNamePatternMixed.MatchString(name) {
		return "image name contains invalid characters"
	}
	return ""
}

// ValidateImageName is the exported wrapper for validateImageName.
func ValidateImageName(name string) string {
	return validateImageName(name)
}
