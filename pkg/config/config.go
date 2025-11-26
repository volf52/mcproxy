package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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