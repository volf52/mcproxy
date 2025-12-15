package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// SecureCommandValidator handles secure validation of commands
type SecureCommandValidator struct {
	allowedPaths     []string
	allowedPattern   *regexp.Regexp
	blockListPattern *regexp.Regexp
	strictMode       bool
}

// NewSecureCommandValidator creates a new secure command validator
func NewSecureCommandValidator() *SecureCommandValidator {
	// Patterns that should never appear in commands (prevent shell injection)
	// Note: Single and double quotes are allowed as they're needed for quoted arguments
	blockList := regexp.MustCompile(`[;&|<>$` + "`" + `\\]`)

	// Allowed pattern for executable names (alphanumeric, hyphens, underscores, dots, forward slashes)
	allowedPattern := regexp.MustCompile(`^[a-zA-Z0-9\-_./]+$`)

	return &SecureCommandValidator{
		allowedPattern:   allowedPattern,
		blockListPattern: blockList,
	}
}

// WithAllowedPaths adds allowed base paths for executables
func (scv *SecureCommandValidator) WithAllowedPaths(paths []string) *SecureCommandValidator {
	scv.allowedPaths = paths
	return scv
}

// WithStrictMode enables strict validation including executable permission checks
func (scv *SecureCommandValidator) WithStrictMode(strict bool) *SecureCommandValidator {
	scv.strictMode = strict
	return scv
}

// ValidateAndParseCommand validates and securely parses a command string
func (scv *SecureCommandValidator) ValidateAndParseCommand(commandStr string) ([]string, error) {
	// Check for control characters (including newlines and carriage returns)
	// Tabs are allowed only within the command, not as trailing whitespace
	for i, r := range commandStr {
		if r < 32 {
			if r == '\t' {
				// Check if tab is at the end or followed by only whitespace
				rest := strings.TrimLeft(commandStr[i:], "\t ")
				if rest == "" {
					return nil, fmt.Errorf("command contains invalid trailing whitespace")
				}
			} else {
				return nil, fmt.Errorf("command contains control characters")
			}
		}
	}

	// First, parse the command to understand its structure
	parts, err := scv.parseCommandString(commandStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %w", err)
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Now check each part for dangerous patterns
	for i, part := range parts {
		// For the executable, be very strict
		if i == 0 {
			if !scv.allowedPattern.MatchString(part) {
				return nil, fmt.Errorf("executable contains invalid characters")
			}
			// Check for blocklisted patterns in executable
			if scv.blockListPattern.MatchString(part) {
				return nil, fmt.Errorf("executable contains potentially dangerous characters")
			}
		} else {
			// For arguments, allow quotes but check for other dangerous patterns
			if strings.Contains(part, ";") || strings.Contains(part, "&") ||
				strings.Contains(part, "|") || strings.Contains(part, "`") ||
				strings.ContainsAny(part, "<>") {
				return nil, fmt.Errorf("argument contains potentially dangerous characters")
			}
		}

		if err := scv.validateCommandPart(part, i == 0); err != nil {
			return nil, fmt.Errorf("invalid command part %d: %w", i, err)
		}
	}

	// Additional validation for the executable path
	if err := scv.validateExecutablePath(parts[0]); err != nil {
		return nil, fmt.Errorf("invalid executable path: %w", err)
	}

	return parts, nil
}

// parseCommandString parses a command string safely, respecting quoted arguments
func (scv *SecureCommandValidator) parseCommandString(commandStr string) ([]string, error) {
	var parts []string
	var current strings.Builder
	inQuotes := false
	escapeNext := false

	for _, r := range commandStr {
		switch {
		case escapeNext:
			// Add the escaped character literally
			current.WriteRune(r)
			escapeNext = false

		case r == '\\':
			// Escape next character
			escapeNext = true

		case r == '"':
			// Toggle quote state
			inQuotes = !inQuotes

		case unicode.IsSpace(r) && !inQuotes:
			// End of argument
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}

		default:
			// Regular character
			current.WriteRune(r)
		}
	}

	// Check for unterminated quotes
	if inQuotes {
		return nil, fmt.Errorf("unterminated quoted string")
	}

	// Add the last part if any
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts, nil
}

// validateCommandPart validates a single part of the command
func (scv *SecureCommandValidator) validateCommandPart(part string, isExecutable bool) error {
	if part == "" {
		return fmt.Errorf("empty command part")
	}

	// Check for null bytes
	if strings.Contains(part, "\x00") {
		return fmt.Errorf("command part contains null byte")
	}

	// Check for path traversal attempts
	if strings.Contains(part, "../") || strings.Contains(part, "..\\") {
		return fmt.Errorf("command part contains path traversal attempt")
	}

	// For non-executable parts (arguments), be more permissive
	if !isExecutable {
		// Arguments can contain more characters but still no control chars
		for _, r := range part {
			if unicode.IsControl(r) && r != '\t' {
				return fmt.Errorf("argument contains control character")
			}
		}
		return nil
	}

	// For executable, validate against allowed pattern
	if !scv.allowedPattern.MatchString(part) {
		return fmt.Errorf("executable contains invalid characters")
	}

	return nil
}

// validateExecutablePath performs additional validation on the executable path
func (scv *SecureCommandValidator) validateExecutablePath(execPath string) error {
	// If path is not absolute, check if it's a simple command name or relative path
	if !filepath.IsAbs(execPath) {
		// Allow simple command names (will be looked up in PATH)
		if !strings.Contains(execPath, "/") {
			// Don't require PATH lookup during validation as the executable might not be installed
			// We'll let exec.Command handle the lookup at runtime
			return nil
		}

		// For relative paths (starting with ./ or ../), allow them but check for path traversal
		if strings.HasPrefix(execPath, "./") || strings.HasPrefix(execPath, "../") {
			// Check for obvious path traversal attempts
			if strings.Contains(execPath, "../") && !strings.HasPrefix(execPath, "../") {
				// Relative path contains traversal but doesn't start with it
				return fmt.Errorf("relative path contains unsafe traversal")
			}
			return nil
		}

		// Other relative paths (just a directory component without explicit ./) are not allowed
		return fmt.Errorf("relative paths must start with ./ or ../")
	}

	// For absolute paths, check if it's in allowed directories
	if len(scv.allowedPaths) > 0 {
		absPath := execPath
		if !filepath.IsAbs(execPath) {
			var err error
			absPath, err = filepath.Abs(execPath)
			if err != nil {
				return fmt.Errorf("failed to get absolute path: %w", err)
			}
		}

		allowed := false
		for _, allowedPath := range scv.allowedPaths {
			absAllowed, err := filepath.Abs(allowedPath)
			if err != nil {
				continue
			}
			if strings.HasPrefix(absPath, absAllowed+string(filepath.Separator)) || absPath == absAllowed {
				allowed = true
				break
			}
		}

		if !allowed {
			return fmt.Errorf("executable path not in allowed directories")
		}
	}

	// Only check file existence if it's an absolute path
	// Skip the check for files that might not exist during testing/configuration
	if filepath.IsAbs(execPath) {
		if info, err := os.Stat(execPath); err == nil {
			// File exists, check if it's a regular file
			if !info.Mode().IsRegular() {
				return fmt.Errorf("executable is not a regular file")
			}
			// In strict mode, check executable permissions
			if scv.strictMode && info.Mode().Perm()&0111 == 0 {
				return fmt.Errorf("executable file lacks execute permissions")
			}
		}
		// If file doesn't exist, we allow it (it might be created later or installed elsewhere)
	}

	return nil
}

// IsSafeCommand checks if a command string appears safe without full validation
func (scv *SecureCommandValidator) IsSafeCommand(commandStr string) bool {
	// Quick check for obviously dangerous patterns
	dangerousPatterns := []string{
		";",
		"&",
		"|",
		"`",
		"$(",
		"${",
		"<",
		">",
		">>",
		"<<",
	}

	lowerCmd := strings.ToLower(commandStr)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerCmd, pattern) {
			return false
		}
	}

	// Check for common attack patterns
	attackPatterns := []string{
		"/etc/passwd",
		"/etc/shadow",
		"rm -rf",
		"chmod 777",
		"nc -l",
		"python -c",
		"perl -e",
		"bash -c",
		"sh -c",
		"eval",
		"exec",
	}

	for _, pattern := range attackPatterns {
		if strings.Contains(lowerCmd, pattern) {
			return false
		}
	}

	return true
}
