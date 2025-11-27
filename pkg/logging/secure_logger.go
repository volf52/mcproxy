package logging

import (
	"log"
	"os"
	"regexp"
	"strings"
)

const (
	// SecretPlaceholder is used to replace secret values in logs
	SecretPlaceholder = "[SECRET]"
	// DebugEnvVar is the environment variable to enable debug logging
	DebugEnvVar = "MCPROXY_DEBUG"
)

// SecretMatcher contains patterns for identifying secrets in text
var (
	// templatePattern matches {{ secret_name }} templates (including empty)
	templatePattern = regexp.MustCompile(`\{\{\s*[^}]*\s*\}\}`)
)

// SecretLogger provides secure logging with secret redaction
type SecretLogger struct {
	debugMode bool
	secrets   map[string]string
}

var globalLogger *SecretLogger

// Init initializes the global secure logger
func Init(secrets map[string]string) {
	debugMode := strings.ToLower(os.Getenv(DebugEnvVar)) == "true"
	globalLogger = &SecretLogger{
		debugMode: debugMode,
		secrets:   secrets,
	}
}

// IsDebugMode returns whether debug logging is enabled
func IsDebugMode() bool {
	if globalLogger == nil {
		return false
	}
	return globalLogger.debugMode
}

// SanitizeForLogging replaces sensitive information with placeholders
func SanitizeForLogging(text string) string {
	if globalLogger == nil || globalLogger.debugMode {
		return text // In debug mode, show the actual content
	}

	// Replace secret template patterns
	sanitized := templatePattern.ReplaceAllString(text, SecretPlaceholder)

	// Replace actual secret values if we have access to the secrets map
	for _, secretValue := range globalLogger.secrets {
		if strings.Contains(sanitized, secretValue) && secretValue != "" {
			// Only replace if the secret is reasonably long to avoid false positives
			if len(secretValue) > 3 {
				sanitized = strings.ReplaceAll(sanitized, secretValue, SecretPlaceholder)
			}
		}
	}

	return sanitized
}

// SanitizeHeadersForLogging replaces header values that might contain secrets
func SanitizeHeadersForLogging(headers map[string]string) map[string]string {
	if globalLogger == nil || globalLogger.debugMode {
		return headers // In debug mode, show actual headers
	}

	sanitized := make(map[string]string)
	for key, value := range headers {
		// Check if header value contains secret templates
		if strings.Contains(value, "{{") {
			sanitized[key] = SecretPlaceholder
		} else if looksLikeSecretValue(value) {
			// Only mark as secret if the header name suggests it might be sensitive
			if isSensitiveHeaderName(key) {
				sanitized[key] = SecretPlaceholder
			} else {
				sanitized[key] = value
			}
		} else {
			sanitized[key] = value
		}
	}

	return sanitized
}

// isSensitiveHeaderName determines if a header name typically contains sensitive information
func isSensitiveHeaderName(headerName string) bool {
	headerNameLower := strings.ToLower(headerName)

	sensitiveHeaders := []string{
		"authorization",
		"api-key",
		"apikey",
		"x-api-key",
		"x-auth-token",
		"auth-token",
		"token",
		"bearer",
		"secret",
		"password",
		"x-secret",
		"x-auth",
		"x-token",
		"session",
		"cookie",
		"x-session",
		"jwt",
		"oauth",
		"signature",
		"credentials",
	}

	for _, sensitive := range sensitiveHeaders {
		if strings.Contains(headerNameLower, sensitive) {
			return true
		}
	}

	return false
}

// looksLikeSecretValue heuristically determines if a value might be a secret
func looksLikeSecretValue(value string) bool {
	// Skip obvious non-secret values first
	if len(value) < 16 {
		return false
	}

	// Values with spaces, tabs, or newlines are probably not secrets
	if strings.Contains(value, " ") || strings.Contains(value, "\t") || strings.Contains(value, "\n") {
		return false
	}

	// Common indicators of secret values (long values with separators)
	if len(value) > 20 &&
		(strings.Contains(value, "-") || strings.Contains(value, "_") || strings.Contains(value, ".")) {
		return true
	}

	// Values that look like API keys, tokens, etc. (long continuous strings)
	if len(value) >= 16 {
		return true
	}

	return false
}

// Printf logs a formatted message with secret sanitization
func Printf(format string, args ...interface{}) {
	if globalLogger == nil {
		// Fallback to standard log if not initialized
		log.Printf(format, args...)
		return
	}

	// Sanitize the format string and arguments
	sanitizedFormat := SanitizeForLogging(format)

	// Sanitize string arguments
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeForLogging(str)
		} else {
			sanitizedArgs[i] = arg
		}
	}

	log.Printf(sanitizedFormat, sanitizedArgs...)
}

// Debugf logs a debug message (only in debug mode)
func Debugf(format string, args ...interface{}) {
	if !IsDebugMode() {
		return
	}

	log.Printf("[DEBUG] "+format, args...)
}

// DebugfSanitized logs a debug message with sanitization (only in debug mode)
func DebugfSanitized(format string, args ...interface{}) {
	if !IsDebugMode() {
		return
	}

	// Even in debug mode, we might want to show sanitized versions sometimes
	sanitizedFormat := SanitizeForLogging(format)
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeForLogging(str)
		} else {
			sanitizedArgs[i] = arg
		}
	}

	log.Printf("[DEBUG] "+sanitizedFormat, sanitizedArgs...)
}

// Println logs a message with secret sanitization
func Println(args ...interface{}) {
	if globalLogger == nil {
		// Fallback to standard log if not initialized
		log.Println(args...)
		return
	}

	// Sanitize string arguments
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeForLogging(str)
		} else {
			sanitizedArgs[i] = arg
		}
	}

	log.Println(sanitizedArgs...)
}

// Info logs an info message
func Info(msg string) {
	log.Printf("[INFO] %s", SanitizeForLogging(msg))
}

// Warning logs a warning message
func Warning(msg string) {
	log.Printf("[WARNING] %s", SanitizeForLogging(msg))
}

// Error logs an error message
func Error(msg string) {
	log.Printf("[ERROR] %s", SanitizeForLogging(msg))
}

// Debug logs a debug message (only in debug mode)
func Debug(msg string) {
	if IsDebugMode() {
		log.Printf("[DEBUG] %s", msg)
	}
}

// DebugSanitized logs a debug message with sanitization (only in debug mode)
func DebugSanitized(msg string) {
	if IsDebugMode() {
		log.Printf("[DEBUG] %s", SanitizeForLogging(msg))
	}
}

// LogTemplateResolution logs the secret template resolution process
func LogTemplateResolution(endpointName string, originalHeaders, resolvedHeaders map[string]string, missingVars []string) {
	if IsDebugMode() {
		log.Printf("[DEBUG] Template resolution for endpoint '%s':", endpointName)
		for headerName, originalValue := range originalHeaders {
			resolvedValue := resolvedHeaders[headerName]
			if originalValue != resolvedValue {
				log.Printf("[DEBUG]   Header '%s': %s -> %s", headerName, originalValue, resolvedValue)
			}
		}
		if len(missingVars) > 0 {
			log.Printf("[DEBUG]   Missing variables: %v", missingVars)
		}
	} else {
		// In production mode, just log that we processed the endpoint
		if len(missingVars) > 0 {
			log.Printf("Warning: Skipping endpoint '%s' due to missing secret variables: %v", endpointName, missingVars)
		}
	}
}

// LogEndpointRegistration logs endpoint registration without exposing secrets
func LogEndpointRegistration(name, upstreamURL string, headers map[string]string) {
	sanitizedHeaders := SanitizeHeadersForLogging(headers)

	if IsDebugMode() {
		// In debug mode, show actual headers
		log.Printf("Registered endpoint: /%s -> %s with headers: %v", name, upstreamURL, headers)
	} else {
		// In production mode, show sanitized headers
		if len(sanitizedHeaders) > 0 {
			log.Printf("Registered endpoint: /%s -> %s with %d configured headers", name, upstreamURL, len(sanitizedHeaders))
		} else {
			log.Printf("Registered endpoint: /%s -> %s", name, upstreamURL)
		}
	}
}