package utils

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// SaveBase64ToTempFile decodes a base64 string and saves it to a temporary file.
// prefix is used for the filename prefix.
// Returns the absolute path to the temp file or an error.
func SaveBase64ToTempFile(data string, prefix string) (string, error) {
	// Check if data has a header (e.g., "data:image/png;base64,")
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 data: %w", err)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-*.tmp", prefix))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(decoded); err != nil {
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

// IsLocalFile checks if a string is a valid local file path.
func IsLocalFile(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
