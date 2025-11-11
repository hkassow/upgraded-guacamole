package lib

import (
	"fmt"
	"os"
	"strings"
)

// LoadSecret reads a secret from a Docker secret file or env variable.
// secretEnvVar: the name of the environment variable that can contain the value or file path
func LoadSecret(secretEnvVar string) (string, error) {
	// Check if a file is specified
	if filePath := os.Getenv(secretEnvVar + "_FILE"); filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read secret file %s: %w", filePath, err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	// Fallback to environment variable
	if value := os.Getenv(secretEnvVar); value != "" {
		return value, nil
	}

	return "", fmt.Errorf("secret %s not found", secretEnvVar)
}

