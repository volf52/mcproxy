package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Config represents the main configuration structure
type Config struct {
	Endpoints map[string]Endpoint `json:"endpoints"`
	LogFile   string             `json:"logFile,omitempty"`
}

// Endpoint represents a single proxy endpoint configuration
type Endpoint struct {
	UpstreamURL string            `json:"upstreamUrl"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// Secrets represents the secrets structure for template substitution
type Secrets map[string]string

// LoadConfig loads the main configuration from JSON file
func LoadConfig() (*Config, error) {
	configPath := os.Getenv("MCPROXY_CONFIG")
	if configPath == "" {
		configPath = "./config.json"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Validate endpoints
	if err := validateEndpoints(config.Endpoints); err != nil {
		return nil, fmt.Errorf("invalid endpoints in config: %w", err)
	}

	return &config, nil
}

// LoadSecrets loads secrets from JSON file
func LoadSecrets() (Secrets, error) {
	secretsPath := os.Getenv("MCPROXY_SECRETS")
	if secretsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		secretsPath = filepath.Join(home, "secrets.json")
	}

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets file %s: %w", secretsPath, err)
	}

	var secrets Secrets
	if err := json.Unmarshal(data, &secrets); err != nil {
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
		if endpoint.UpstreamURL == "" {
			return fmt.Errorf("upstream URL cannot be empty for endpoint '%s'", name)
		}
	}

	return nil
}

// ProcessSecretTemplates processes all endpoint headers with secret templating
// Returns endpoints with resolved secrets and a list of skipped endpoints
func ProcessSecretTemplates(config *Config, secrets Secrets) (map[string]Endpoint, []string) {
	processed := make(map[string]Endpoint)
	var skipped []string

	for name, endpoint := range config.Endpoints {
		processedEndpoint, missingVars := processEndpointTemplating(endpoint, secrets)

		if len(missingVars) > 0 {
			log.Printf("Warning: Skipping endpoint '%s' due to missing secret variables: %v", name, missingVars)
			skipped = append(skipped, name)
		} else {
			processed[name] = processedEndpoint
		}
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
			log.Printf("Error processing header '%s': %v", headerName, err)
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