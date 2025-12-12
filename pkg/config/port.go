package config

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizePort validates and normalizes a port string to the format ":port".
// It accepts both "8099" and ":8099" formats and returns ":8099" in both cases.
// Empty string returns the default port ":8099".
// Whitespace is trimmed from input. Always normalizes to canonical format.
func NormalizePort(portStr string) (string, error) {
	// Trim whitespace from input
	trimmed := strings.TrimSpace(portStr)

	if trimmed == "" {
		return ":8099", nil
	}

	// Remove leading colon if present
	portNum := strings.TrimPrefix(trimmed, ":")

	// Validate the port number
	if err := validatePortNumber(portNum); err != nil {
		// Show the trimmed input in error for clarity
		if trimmed != portStr {
			return "", fmt.Errorf("invalid port format '%s' (trimmed from '%q'): %w", trimmed, portStr, err)
		}
		return "", fmt.Errorf("invalid port format '%s': %w", trimmed, err)
	}

	// Convert to int and back to string to normalize (e.g., "0080" -> "80")
	port, _ := strconv.Atoi(portNum)
	return ":" + strconv.Itoa(port), nil
}

// validatePortNumber validates that a port string represents a valid port number (1-65535)
func validatePortNumber(portStr string) error {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("port must be a number, got '%s'", portStr)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}

	return nil
}
