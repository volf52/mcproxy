package config

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizePort validates and normalizes a port string to the format ":port".
// It accepts both "8099" and ":8099" formats and returns ":8099" in both cases.
// Empty string returns the default port ":8099".
func NormalizePort(portStr string) (string, error) {
	if portStr == "" {
		return ":8099", nil
	}

	if strings.HasPrefix(portStr, ":") {
		portNum := strings.TrimPrefix(portStr, ":")
		if err := validatePortNumber(portNum); err != nil {
			return "", fmt.Errorf("invalid port format '%s': %w", portStr, err)
		}
		return portStr, nil
	}

	if err := validatePortNumber(portStr); err != nil {
		return "", fmt.Errorf("invalid port format '%s': %w", portStr, err)
	}

	// Convert to int and back to string to normalize (e.g., "0080" -> "80")
	port, _ := strconv.Atoi(portStr)
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
