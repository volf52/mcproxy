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
	// DebugLevelEnvVar is the environment variable to control debug level
	DebugLevelEnvVar = "MCPROXY_DEBUG_LEVEL"
	// ShowSecretsEnvVar is the environment variable to show unredacted secrets
	ShowSecretsEnvVar = "MCPROXY_SHOW_SECRETS"
)

// Debug levels for controlling secret disclosure
const (
	DebugLevelOff        = "off"
	DebugLevelSanitized  = "sanitized"  // Default - always redact secrets
	DebugLevelPartial    = "partial"    // Show partial secrets (e.g., "sec***123")
	DebugLevelUnredacted = "unredacted" // Show full secrets (explicit opt-in)
)

// SecretMatcher contains patterns for identifying secrets in text
var (
	// templatePattern matches {{ secret_name }} templates (including empty)
	templatePattern = regexp.MustCompile(`\{\{\s*[^}]*\s*\}\}`)
)

// SecretLogger provides secure logging with secret redaction
type SecretLogger struct {
	debugMode  bool
	debugLevel string
	secrets    map[string]string
}

var globalLogger *SecretLogger

// Init initializes the global secure logger
func Init(secrets map[string]string) {
	debugMode := strings.ToLower(os.Getenv(DebugEnvVar)) == "true"

	// Get debug level from environment
	debugLevel := strings.ToLower(os.Getenv(DebugLevelEnvVar))
	if debugLevel == "" {
		debugLevel = DebugLevelSanitized // Default to sanitized for security
	}

	// Check for MCPROXY_SHOW_SECRETS=true as alternative for unredacted mode
	if strings.ToLower(os.Getenv(ShowSecretsEnvVar)) == "true" {
		debugLevel = DebugLevelUnredacted
	}

	// Validate debug level
	validLevels := map[string]bool{
		DebugLevelOff:        true,
		DebugLevelSanitized:  true,
		DebugLevelPartial:    true,
		DebugLevelUnredacted: true,
	}
	if !validLevels[debugLevel] {
		debugLevel = DebugLevelSanitized // Fallback to sanitized for security
	}

	globalLogger = &SecretLogger{
		debugMode:  debugMode,
		debugLevel: debugLevel,
		secrets:    secrets,
	}
}

// IsDebugMode returns whether debug logging is enabled
func IsDebugMode() bool {
	if globalLogger == nil {
		return false
	}
	return globalLogger.debugMode
}

// getDebugLevel returns the current debug level
func getDebugLevel() string {
	if globalLogger == nil {
		return DebugLevelSanitized
	}
	return globalLogger.debugLevel
}

// IsUnredactedMode returns whether unredacted secrets should be shown
func IsUnredactedMode() bool {
	return getDebugLevel() == DebugLevelUnredacted
}

// SanitizeForLogging replaces sensitive information with placeholders
func SanitizeForLogging(text string) string {
	return SanitizeForLoggingWithLevel(text, getDebugLevel())
}

// SanitizeForLoggingWithLevel replaces sensitive information with placeholders based on debug level
func SanitizeForLoggingWithLevel(text string, level string) string {
	if globalLogger == nil {
		return text
	}

	// In unredacted mode, show full content (for admin use only)
	if level == DebugLevelUnredacted {
		return text
	}

	// Replace secret template patterns first
	sanitized := templatePattern.ReplaceAllString(text, SecretPlaceholder)

	// Handle actual secret values based on debug level
	for _, secretValue := range globalLogger.secrets {
		if strings.Contains(sanitized, secretValue) && secretValue != "" {
			if len(secretValue) > 3 {
				if level == DebugLevelPartial {
					// Show partial secrets
					partial := createPartialSecret(secretValue)
					sanitized = strings.ReplaceAll(sanitized, secretValue, partial)
				} else {
					// Full sanitization (default)
					sanitized = strings.ReplaceAll(sanitized, secretValue, SecretPlaceholder)
				}
			}
		}
	}

	return sanitized
}

// SanitizeHeadersForLogging replaces header values that might contain secrets
func SanitizeHeadersForLogging(headers map[string]string) map[string]string {
	return SanitizeHeadersForLoggingWithLevel(headers, getDebugLevel())
}

// SanitizeHeadersForLoggingWithLevel replaces header values that might contain secrets based on debug level
func SanitizeHeadersForLoggingWithLevel(headers map[string]string, level string) map[string]string {
	if globalLogger == nil {
		return headers
	}

	// In unredacted mode, show full headers (for admin use only)
	if level == DebugLevelUnredacted {
		return headers
	}

	sanitized := make(map[string]string)
	for key, value := range headers {
		// Check if header value contains secret templates using regex pattern
		if templatePattern.MatchString(value) {
			if level == DebugLevelPartial {
				sanitized[key] = "[TEMPLATE]"
			} else {
				sanitized[key] = SecretPlaceholder
			}
		} else if containsKnownSecret(value) || (looksLikeSecretValue(value) && isSensitiveHeaderName(key)) {
			// Mark as secret if it contains known secret values OR looks like a secret and header name is sensitive
			if containsKnownSecret(value) || isSensitiveHeaderName(key) {
				// Known secret found OR sensitive header - replace entire value
				if level == DebugLevelPartial {
					if containsKnownSecret(value) {
						sanitized[key] = replaceKnownSecretsWithPartial(value)
					} else {
						sanitized[key] = createPartialSecret(value)
					}
				} else {
					sanitized[key] = SecretPlaceholder
				}
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

// containsKnownSecret checks if text contains any known secret values
func containsKnownSecret(text string) bool {
	for _, secretValue := range globalLogger.secrets {
		if strings.Contains(text, secretValue) && secretValue != "" {
			return true
		}
	}
	return false
}

// replaceKnownSecrets replaces known secret values with [SECRET]
func replaceKnownSecrets(text string) string {
	result := text
	for _, secretValue := range globalLogger.secrets {
		if strings.Contains(result, secretValue) && secretValue != "" {
			result = strings.ReplaceAll(result, secretValue, SecretPlaceholder)
		}
	}
	return result
}

// replaceKnownSecretsWithPartial replaces known secret values with partial versions
func replaceKnownSecretsWithPartial(text string) string {
	result := text
	for _, secretValue := range globalLogger.secrets {
		if strings.Contains(result, secretValue) && secretValue != "" && len(secretValue) > 3 {
			partial := createPartialSecret(secretValue)
			result = strings.ReplaceAll(result, secretValue, partial)
		}
	}
	return result
}

// createPartialSecret creates a partially masked version of a secret
func createPartialSecret(secret string) string {
	if len(secret) <= 4 {
		return strings.Repeat("*", len(secret))
	}
	if len(secret) <= 8 {
		return secret[:1] + "***" + secret[len(secret)-1:]
	}
	return secret[:2] + "***" + secret[len(secret)-2:]
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

// Debugf logs a debug message with respect to debug level settings
func Debugf(format string, args ...interface{}) {
	if !IsDebugMode() {
		return
	}

	level := getDebugLevel()
	sanitizedFormat := SanitizeForLoggingWithLevel(format, level)
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeForLoggingWithLevel(str, level)
		} else {
			sanitizedArgs[i] = arg
		}
	}

	log.Printf("[DEBUG] "+sanitizedFormat, sanitizedArgs...)
}

// DebugfSanitized logs a debug message with forced sanitization (only in debug mode)
func DebugfSanitized(format string, args ...interface{}) {
	if !IsDebugMode() {
		return
	}

	// Always sanitize regardless of debug level
	sanitizedFormat := SanitizeForLoggingWithLevel(format, DebugLevelSanitized)
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeForLoggingWithLevel(str, DebugLevelSanitized)
		} else {
			sanitizedArgs[i] = arg
		}
	}

	log.Printf("[DEBUG] "+sanitizedFormat, sanitizedArgs...)
}

// DebugfPartial logs a debug message with partial secret disclosure (only in debug mode)
func DebugfPartial(format string, args ...interface{}) {
	if !IsDebugMode() {
		return
	}

	// Show partial secrets
	sanitizedFormat := SanitizeForLoggingWithLevel(format, DebugLevelPartial)
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeForLoggingWithLevel(str, DebugLevelPartial)
		} else {
			sanitizedArgs[i] = arg
		}
	}

	log.Printf("[DEBUG] "+sanitizedFormat, sanitizedArgs...)
}

// DebugfUnredacted logs a debug message without any sanitization (admin use only)
func DebugfUnredacted(format string, args ...interface{}) {
	if !IsDebugMode() || !IsUnredactedMode() {
		return
	}

	// Show full content - no sanitization (admin use only)
	log.Printf("[DEBUG-UNREDACTED] "+format, args...)
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
	if !IsDebugMode() {
		return
	}

	level := getDebugLevel()
	sanitizedMsg := SanitizeForLoggingWithLevel(msg, level)
	log.Printf("[DEBUG] %s", sanitizedMsg)
}

// DebugSanitized logs a debug message with forced sanitization (only in debug mode)
func DebugSanitized(msg string) {
	if !IsDebugMode() {
		return
	}

	// Always sanitize regardless of debug level
	sanitizedMsg := SanitizeForLoggingWithLevel(msg, DebugLevelSanitized)
	log.Printf("[DEBUG] %s", sanitizedMsg)
}

// LogTemplateResolution logs the secret template resolution process
func LogTemplateResolution(endpointName string, originalHeaders, resolvedHeaders map[string]string, missingVars []string) {
	if !IsDebugMode() {
		// In production mode, just log that we processed the endpoint
		if len(missingVars) > 0 {
			log.Printf("Warning: Skipping endpoint '%s' due to missing secret variables: %v", endpointName, missingVars)
		}
		return
	}

	level := getDebugLevel()
	log.Printf("[DEBUG] Template resolution for endpoint '%s':", endpointName)

	for headerName, originalValue := range originalHeaders {
		resolvedValue := resolvedHeaders[headerName]
		if originalValue != resolvedValue {
			sanitizedOriginal := SanitizeForLoggingWithLevel(originalValue, level)
			sanitizedResolved := SanitizeForLoggingWithLevel(resolvedValue, level)
			log.Printf("[DEBUG]   Header '%s': %s -> %s", headerName, sanitizedOriginal, sanitizedResolved)
		}
	}

	if len(missingVars) > 0 {
		log.Printf("[DEBUG]   Missing variables: %v", missingVars)
	}
}

// LogEndpointRegistration logs endpoint registration without exposing secrets
func LogEndpointRegistration(name, upstreamURL, routePrefix string, headers map[string]string) {
	if routePrefix == "" {
		routePrefix = "/mcp" // Default prefix
	}
	fullPath := routePrefix + "/" + name

	sanitizedHeaders := SanitizeHeadersForLogging(headers)

	switch getDebugLevel() {
	case DebugLevelUnredacted:
		// Show full information (admin use only)
		log.Printf("Registered endpoint: %s -> %s with headers: %v", fullPath, upstreamURL, headers)
	case DebugLevelPartial:
		// Show partial headers
		partialHeaders := SanitizeHeadersForLoggingWithLevel(headers, DebugLevelPartial)
		if len(partialHeaders) > 0 {
			log.Printf("Registered endpoint: %s -> %s with headers: %v", fullPath, upstreamURL, partialHeaders)
		} else {
			log.Printf("Registered endpoint: %s -> %s", fullPath, upstreamURL)
		}
	default: // DebugLevelSanitized
		// Show minimal information (most secure)
		if len(sanitizedHeaders) > 0 {
			log.Printf("Registered endpoint: %s -> %s with %d configured headers", fullPath, upstreamURL, len(sanitizedHeaders))
		} else {
			log.Printf("Registered endpoint: %s -> %s", fullPath, upstreamURL)
		}
	}
}
