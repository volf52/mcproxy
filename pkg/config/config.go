package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mcproxy/pkg/logging"
)

// Config represents the main configuration structure
type Config struct {
	Endpoints         map[string]Endpoint `json:"endpoints"`
	LogFile           string              `json:"logFile,omitempty"`
	GlobalTimeout     time.Duration       `json:"globalTimeout,omitempty"`     // Global timeout for all endpoints (default: 60s)
	GlobalMaxBodySize int64               `json:"globalMaxBodySize,omitempty"` // Global max body size in bytes (default: 10MB)
}

// Endpoint represents a single proxy endpoint configuration
type Endpoint struct {
	Url         string            `json:"url" description:"The upstream server url"`
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

// validateEndpoints validates the endpoints configuration
func validateEndpoints(endpoints map[string]Endpoint) error {
	if len(endpoints) == 0 {
		return fmt.Errorf("at least one endpoint must be defined")
	}

	for name, endpoint := range endpoints {
		if name == "" {
			return fmt.Errorf("endpoint name cannot be empty")
		}
		if endpoint.Url == "" {
			return fmt.Errorf("upstream URL cannot be empty for endpoint '%s'", name)
		}

		// Validate timeout if specified
		if endpoint.Timeout != nil {
			if *endpoint.Timeout <= 0 {
				return fmt.Errorf("timeout must be positive for endpoint '%s'", name)
			}
			if *endpoint.Timeout > 300*time.Second {
				return fmt.Errorf("timeout too large (max 300s) for endpoint '%s'", name)
			}
		}

		// Validate max body size if specified
		if endpoint.MaxBodySize != nil {
			if *endpoint.MaxBodySize <= 0 {
				return fmt.Errorf("maxBodySize must be positive for endpoint '%s'", name)
			}
			if *endpoint.MaxBodySize > 100*1024*1024 { // 100MB
				return fmt.Errorf("maxBodySize too large (max 100MB) for endpoint '%s'", name)
			}
		}
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
