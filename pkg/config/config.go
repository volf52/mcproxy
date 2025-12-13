package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mcproxy/pkg/logging"
)

// ServerConfig contains server-specific configuration
type ServerConfig struct {
	ReadTimeout     int `json:"readTimeout,omitempty" description:"Maximum duration for reading the entire request, including the body (seconds)"`
	WriteTimeout    int `json:"writeTimeout,omitempty" description:"Maximum duration before timing out writes of the response (seconds)"`
	IdleTimeout     int `json:"idleTimeout,omitempty" description:"Maximum amount of time to wait for the next request when keep-alives are enabled (seconds)"`
	ShutdownTimeout int `json:"shutdownTimeout,omitempty" description:"Maximum time to wait for graceful shutdown (seconds)"`
}

// Config represents the main configuration structure
type Config struct {
	Endpoints         map[string]Endpoint `json:"endpoints"`
	LogFile           string              `json:"logFile,omitempty"`
	GlobalTimeout     time.Duration       `json:"globalTimeout,omitempty"`     // Global timeout for all endpoints (default: 60s)
	GlobalMaxBodySize int64               `json:"globalMaxBodySize,omitempty"` // Global max body size in bytes (default: 10MB)
	Server            ServerConfig        `json:"server,omitempty"`            // Server configuration for timeouts and shutdown
}

// EndpointType represents the type of endpoint (HTTP or stdio)
type EndpointType string

const (
	EndpointTypeHTTP  EndpointType = "http"
	EndpointTypeStdio EndpointType = "stdio"
)

// Endpoint represents a single proxy endpoint configuration
type Endpoint struct {
	Type        EndpointType      `json:"type,omitempty" description:"Endpoint type: 'http' for HTTP upstream or 'stdio' for MCP stdio process (default: 'http')"`
	Url         string            `json:"url,omitempty" description:"The upstream server url (required for http type)"`
	Command     []string          `json:"command,omitempty" description:"Command and arguments to execute for stdio endpoints"`
	Env         map[string]string `json:"env,omitempty" description:"Environment variables to set for the stdio process"`
	Args        []string          `json:"args,omitempty" description:"Arguments to pass to the MCP stdio endpoint during initialization"`
	Headers     map[string]string `json:"headers,omitempty" description:"Custom headers to add to requests to the upstream server."`
	Timeout     *time.Duration    `json:"timeout,omitempty" description:"Per-endpoint timeout in seconds (overrides global timeout)."`
	MaxBodySize *int64            `json:"maxBodySize,omitempty" description:"Maximum request body size in bytes (overrides global limit)."`
	_           struct{}          `additionalProperties:"false"`                            // Tags of unnamed field are applied to parent schema.
	_           struct{}          `title:"MCProxy Config" description:"Config for MCProxy"` // Multiple unnamed fields can be used.
}

// Secrets represents the secrets structure for template substitution
type Secrets map[string]string

// LoadConfig loads the main configuration from JSON/JSONC file
func LoadConfig() (*Config, error) {
	configPath := os.Getenv("MCPROXY_CONFIG")
	if configPath == "" {
		// Check for JSONC file first in .mcproxy directory
		mcproxyDir := ".mcproxy"
		if fileExists(filepath.Join(mcproxyDir, "config.jsonc")) {
			configPath = filepath.Join(mcproxyDir, "config.jsonc")
		} else {
			configPath = filepath.Join(mcproxyDir, "config.json")
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	if err := UnmarshalWithAutoDetection(data, &config, configPath); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Validate endpoints
	if err := validateEndpoints(config.Endpoints); err != nil {
		return nil, fmt.Errorf("invalid endpoints in config: %w", err)
	}

	return &config, nil
}

// LoadSecrets loads secrets from JSON/JSONC file
func LoadSecrets() (Secrets, error) {
	secretsPath := os.Getenv("MCPROXY_SECRETS")
	if secretsPath == "" {
		// Check .mcproxy directory first
		mcproxyDir := ".mcproxy"
		if fileExists(filepath.Join(mcproxyDir, "secrets.jsonc")) {
			secretsPath = filepath.Join(mcproxyDir, "secrets.jsonc")
		} else if fileExists(filepath.Join(mcproxyDir, "secrets.json")) {
			secretsPath = filepath.Join(mcproxyDir, "secrets.json")
		} else {
			// Fallback to home directory
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get user home directory: %w", err)
			}

			// Check for JSONC file first
			jsoncPath := filepath.Join(home, "secrets.jsonc")
			if fileExists(jsoncPath) {
				secretsPath = jsoncPath
			} else {
				secretsPath = filepath.Join(home, "secrets.json")
			}
		}
	}

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets file %s: %w", secretsPath, err)
	}

	var secrets Secrets
	if err := UnmarshalWithAutoDetection(data, &secrets, secretsPath); err != nil {
		return nil, fmt.Errorf("failed to parse secrets file %s: %w", secretsPath, err)
	}

	return secrets, nil
}

// validationError represents a single validation error
type validationError struct {
	field   string
	message string
}

// validationErrors is a collection of validation errors
type validationErrors []validationError

// Error implements the error interface
func (ve validationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	if len(ve) == 1 {
		return ve[0].message
	}

	var messages []string
	for _, err := range ve {
		messages = append(messages, err.message)
	}
	return fmt.Sprintf("validation failed with %d errors:\n  %s", len(messages), strings.Join(messages, "\n  "))
}

// add adds a new validation error
func (ve *validationErrors) add(field, message string) {
	*ve = append(*ve, validationError{field: field, message: message})
}

// isValidURL checks if a URL is valid and uses HTTP/HTTPS scheme
func isValidURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme, got '%s'", parsed.Scheme)
	}

	// Check host
	if parsed.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	// Reject fragments
	if parsed.Fragment != "" {
		return fmt.Errorf("URL fragments are not allowed")
	}

	return nil
}

// isValidHeaderKey checks if a header key is valid according to RFC 7230
func isValidHeaderKey(key string) error {
	if key == "" {
		return fmt.Errorf("header key cannot be empty")
	}

	// Header keys should not contain control characters, spaces, or tabs
	for _, r := range key {
		if r < 33 || r > 126 {
			return fmt.Errorf("header key contains invalid character: %q", r)
		}
		if r == ':' {
			return fmt.Errorf("header key cannot contain colon character")
		}
	}

	return nil
}

// isValidHeaderValue checks if a header value is valid according to RFC 7230
func isValidHeaderValue(value string) error {
	// Header values should not contain control characters except for tab
	for _, r := range value {
		// Check for line breaks first
		if r == '\r' || r == '\n' {
			return fmt.Errorf("header value cannot contain line breaks")
		}
		// Check for other control characters
		if r < ' ' && r != '\t' {
			return fmt.Errorf("header value contains invalid control character")
		}
	}

	return nil
}

// validateEndpoints validates the endpoints configuration with comprehensive checks
func validateEndpoints(endpoints map[string]Endpoint) error {
	var errors validationErrors

	if len(endpoints) == 0 {
		return fmt.Errorf("at least one endpoint must be defined")
	}

	// Track case-insensitive endpoint names to detect collisions
	lowercaseNames := make(map[string]string)

	for name, endpoint := range endpoints {
		if name == "" {
			errors.add("name", "endpoint name cannot be empty")
			continue
		}

		// Check for case-insensitive name collisions
		lowerName := strings.ToLower(name)
		if existing, exists := lowercaseNames[lowerName]; exists {
			errors.add("name", fmt.Sprintf("endpoint name '%s' conflicts with endpoint '%s' (names are case-insensitive)", name, existing))
			continue
		}
		lowercaseNames[lowerName] = name

		// Set default type if not specified
		endpointType := endpoint.Type
		if endpointType == "" {
			endpointType = EndpointTypeHTTP
		}

		// Validate based on endpoint type
		switch endpointType {
		case EndpointTypeHTTP:
			// HTTP endpoints require a URL
			if endpoint.Url == "" {
				errors.add(fmt.Sprintf("endpoint[%s].url", name), "url is required for HTTP endpoints")
			} else if err := isValidURL(endpoint.Url); err != nil {
				errors.add(fmt.Sprintf("endpoint[%s].url", name), err.Error())
			}

			// HTTP endpoints should not have command or env
			if len(endpoint.Command) > 0 {
				errors.add(fmt.Sprintf("endpoint[%s].command", name), "command is not allowed for HTTP endpoints")
			}
			if len(endpoint.Env) > 0 {
				errors.add(fmt.Sprintf("endpoint[%s].env", name), "env is not allowed for HTTP endpoints")
			}
			if len(endpoint.Args) > 0 {
				errors.add(fmt.Sprintf("endpoint[%s].args", name), "args is not allowed for HTTP endpoints")
			}

		case EndpointTypeStdio:
			// stdio endpoints require a command
			if len(endpoint.Command) == 0 {
				errors.add(fmt.Sprintf("endpoint[%s].command", name), "command is required for stdio endpoints")
			} else {
				// Validate command components
				for i, cmdPart := range endpoint.Command {
					if cmdPart == "" {
						errors.add(fmt.Sprintf("endpoint[%s].command[%d]", name, i), "command component cannot be empty")
					}
					if strings.ContainsAny(cmdPart, "\r\n\t") {
						errors.add(fmt.Sprintf("endpoint[%s].command[%d]", name, i), "command component contains invalid characters")
					}
				}
			}

			// stdio endpoints should not have a URL
			if endpoint.Url != "" {
				errors.add(fmt.Sprintf("endpoint[%s].url", name), "url is not allowed for stdio endpoints")
			}

			// Validate environment variables
			for key, value := range endpoint.Env {
				if key == "" {
					errors.add(fmt.Sprintf("endpoint[%s].env", name), "environment variable key cannot be empty")
				}
				if strings.ContainsAny(key, "\x00\r\n") {
					errors.add(fmt.Sprintf("endpoint[%s].env[%s]", name, key), "environment variable key contains invalid characters")
				}
				if strings.ContainsAny(value, "\x00\r\n") {
					errors.add(fmt.Sprintf("endpoint[%s].env[%s]", name, key), "environment variable value contains invalid characters")
				}
			}

			// Validate args
			for i, arg := range endpoint.Args {
				if arg == "" {
					errors.add(fmt.Sprintf("endpoint[%s].args[%d]", name, i), "arg cannot be empty")
				}
			}

		default:
			errors.add(fmt.Sprintf("endpoint[%s].type", name), fmt.Sprintf("invalid endpoint type '%s', must be 'http' or 'stdio'", endpointType))
		}

		// Validate headers (common to both types)
		for key, value := range endpoint.Headers {
			if err := isValidHeaderKey(key); err != nil {
				errors.add(fmt.Sprintf("endpoint[%s].headers[%s]", name, key), err.Error())
			}
			if err := isValidHeaderValue(value); err != nil {
				errors.add(fmt.Sprintf("endpoint[%s].headers[%s]", name, key), err.Error())
			}
		}

		// Validate timeout if specified
		if endpoint.Timeout != nil {
			if *endpoint.Timeout <= 0 {
				errors.add(fmt.Sprintf("endpoint[%s].timeout", name), "timeout must be positive")
			}
			if *endpoint.Timeout > 300*time.Second {
				errors.add(fmt.Sprintf("endpoint[%s].timeout", name), "timeout too large (max 300s)")
			}
		}

		// Validate max body size if specified
		if endpoint.MaxBodySize != nil {
			if *endpoint.MaxBodySize <= 0 {
				errors.add(fmt.Sprintf("endpoint[%s].maxBodySize", name), "maxBodySize must be positive")
			}
			if *endpoint.MaxBodySize > 100*1024*1024 { // 100MB
				errors.add(fmt.Sprintf("endpoint[%s].maxBodySize", name), "maxBodySize too large (max 100MB)")
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// setConfigDefaults sets default values for configuration
func setConfigDefaults(config *Config) {
	// Set global defaults
	if config.GlobalTimeout == 0 {
		config.GlobalTimeout = 60 * time.Second
	}
	if config.GlobalMaxBodySize == 0 {
		config.GlobalMaxBodySize = 10 * 1024 * 1024 // 10MB
	}

	// Set server defaults
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 30 // 30 seconds
	}
	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 30 // 30 seconds
	}
	if config.Server.IdleTimeout == 0 {
		config.Server.IdleTimeout = 120 // 120 seconds
	}
	if config.Server.ShutdownTimeout == 0 {
		config.Server.ShutdownTimeout = 30 // 30 seconds
	}
}

// LoadConfigWithDefaults loads configuration with backward compatibility
// Uses hierarchical loading if available, falls back to original behavior
func LoadConfigWithDefaults() (*Config, Secrets, error) {
	var config *Config
	var secrets Secrets

	// Try hierarchical loading first
	result, err := LoadConfigHierarchical()
	if err != nil {
		// Fall back to original behavior if hierarchical loading fails
		logging.Printf("Warning: Hierarchical loading failed, falling back to single file loading: %v", err)
		config, err = LoadConfig()
		if err != nil {
			return nil, nil, err
		}
		secrets, err = LoadSecrets()
		if err != nil {
			return nil, nil, err
		}
	} else {
		// Use hierarchical loading result
		config = result.Config
		secrets = result.Secrets

		// Validate merged configuration
		if err := validateMergedConfig(config); err != nil {
			return nil, nil, fmt.Errorf("invalid merged configuration: %w", err)
		}
	}

	// Apply default values
	setConfigDefaults(config)

	// Override with environment variables if present
	overrideWithEnvVars(config)

	return config, secrets, nil
}

// ProcessSecretTemplates processes all endpoint headers with secret templating
// Returns endpoints with resolved secrets and a list of skipped endpoints
func ProcessSecretTemplates(config *Config, secrets Secrets) (map[string]Endpoint, []string) {
	processed := make(map[string]Endpoint)
	var skipped []string

	for name, endpoint := range config.Endpoints {
		processedEndpoint, missingVars := processEndpointTemplating(endpoint, secrets)

		if len(missingVars) > 0 {
			logging.Printf("Warning: Skipping endpoint '%s' due to missing secret variables: %v", name, missingVars)
			skipped = append(skipped, name)
		} else {
			processed[name] = processedEndpoint
		}

		// Log template resolution details for debugging
		logging.LogTemplateResolution(name, endpoint.Headers, processedEndpoint.Headers, missingVars)
	}

	return processed, skipped
}

// processEndpointTemplating processes secret templates for a single endpoint
func processEndpointTemplating(endpoint Endpoint, secrets Secrets) (Endpoint, []string) {
	var allMissingVars []string
	processed := endpoint
	processed.Headers = make(map[string]string)

	// Process headers
	for headerName, headerValue := range endpoint.Headers {
		resolvedValue, missingVars, err := substituteTemplate(headerValue, secrets)
		if err != nil {
			logging.Printf("Error processing header '%s': %v", headerName, err)
			logging.Debugf("Template error details: header='%s', value='%s', error=%v", headerName, headerValue, err)
			// Use original value if template processing fails
			resolvedValue = headerValue
		}

		allMissingVars = append(allMissingVars, missingVars...)
		processed.Headers[headerName] = resolvedValue
	}

	// Process environment variables for stdio endpoints
	if len(endpoint.Env) > 0 {
		processed.Env = make(map[string]string)
		for envName, envValue := range endpoint.Env {
			resolvedValue, missingVars, err := substituteTemplate(envValue, secrets)
			if err != nil {
				logging.Printf("Error processing environment variable '%s': %v", envName, err)
				logging.Debugf("Template error details: env='%s', value='%s', error=%v", envName, envValue, err)
				// Use original value if template processing fails
				resolvedValue = envValue
			}

			allMissingVars = append(allMissingVars, missingVars...)
			processed.Env[envName] = resolvedValue
		}
	}

	return processed, allMissingVars
}

// substituteTemplate replaces {{ var_name }} placeholders with secret values
// Returns the resolved string, a slice of missing variable names, and any error
func substituteTemplate(text string, secrets Secrets) (string, []string, error) {
	var missingVars []string

	for {
		start := strings.Index(text, "{{")
		if start == -1 {
			break // no more templates
		}

		end := strings.Index(text, "}}")
		if end == -1 {
			return text, missingVars, fmt.Errorf("unclosed template placeholder in: %s", text)
		}

		if end <= start+1 {
			return text, missingVars, fmt.Errorf("invalid template placeholder in: %s", text)
		}

		varName := strings.TrimSpace(text[start+2 : end])
		if varName == "" {
			return text, missingVars, fmt.Errorf("empty template variable in: %s", text)
		}

		if secret, exists := secrets[varName]; exists {
			text = text[:start] + secret + text[end+2:]
		} else {
			missingVars = append(missingVars, varName)
			text = text[:start] + text[end+2:] // remove the placeholder
		}
	}

	return text, missingVars, nil
}

// overrideWithEnvVars overrides configuration with environment variables
func overrideWithEnvVars(config *Config) {
	// Override server timeouts if environment variables are set
	if readTimeout := os.Getenv("MCPROXY_READ_TIMEOUT"); readTimeout != "" {
		if val, err := strconv.Atoi(readTimeout); err == nil && val > 0 {
			config.Server.ReadTimeout = val
			logging.Debugf("Overriding read timeout with environment variable: %d seconds", val)
		}
	}
	if writeTimeout := os.Getenv("MCPROXY_WRITE_TIMEOUT"); writeTimeout != "" {
		if val, err := strconv.Atoi(writeTimeout); err == nil && val > 0 {
			config.Server.WriteTimeout = val
			logging.Debugf("Overriding write timeout with environment variable: %d seconds", val)
		}
	}
	if idleTimeout := os.Getenv("MCPROXY_IDLE_TIMEOUT"); idleTimeout != "" {
		if val, err := strconv.Atoi(idleTimeout); err == nil && val > 0 {
			config.Server.IdleTimeout = val
			logging.Debugf("Overriding idle timeout with environment variable: %d seconds", val)
		}
	}
	if shutdownTimeout := os.Getenv("MCPROXY_SHUTDOWN_TIMEOUT"); shutdownTimeout != "" {
		if val, err := strconv.Atoi(shutdownTimeout); err == nil && val > 0 {
			config.Server.ShutdownTimeout = val
			logging.Debugf("Overriding shutdown timeout with environment variable: %d seconds", val)
		}
	}
}
