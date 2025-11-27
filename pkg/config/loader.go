package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"mcproxy/pkg/logging"
)

// ConfigFileInfo holds information about loaded configuration files
type ConfigFileInfo struct {
	Path     string
	Loaded   bool
	Endpoints int
	Secrets   int
}

// HierarchicalLoadResult contains the merged configuration and file loading information
type HierarchicalLoadResult struct {
	Config       *Config
	Secrets      Secrets
	GlobalFiles  ConfigFileInfo
	ProjectFiles ConfigFileInfo
}

// getGlobalConfigPath returns the path to the global config file
func getGlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logging.Printf("Warning: Failed to get user home directory: %v", err)
		return ""
	}
	return filepath.Join(home, "config.json")
}

// getGlobalSecretsPath returns the path to the global secrets file
func getGlobalSecretsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logging.Printf("Warning: Failed to get user home directory: %v", err)
		return ""
	}
	return filepath.Join(home, "secrets.json")
}

// getProjectConfigPath returns the path to the project config file
func getProjectConfigPath() string {
	configPath := os.Getenv("MCPROXY_CONFIG")
	if configPath == "" {
		configPath = "./config.json"
	}
	return configPath
}

// getProjectSecretsPath returns the path to the project secrets file
func getProjectSecretsPath() string {
	secretsPath := os.Getenv("MCPROXY_SECRETS")
	if secretsPath == "" {
		secretsPath = "./secrets.json"
	}
	return secretsPath
}

// fileExists checks if a file exists and is not a directory
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// loadConfigFromFile loads configuration from a file, returns nil if file doesn't exist
func loadConfigFromFile(path string) (*Config, error) {
	if !fileExists(path) {
		return nil, nil // File not found is not an error for optional files
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	logging.Debugf("Loaded config from %s with %d endpoints", path, len(config.Endpoints))
	return &config, nil
}

// loadSecretsFromFile loads secrets from a file, returns nil if file doesn't exist
func loadSecretsFromFile(path string) (Secrets, error) {
	if !fileExists(path) {
		return nil, nil // File not found is not an error for optional files
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets file %s: %w", path, err)
	}

	var secrets Secrets
	if err := json.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf("failed to parse secrets file %s: %w", path, err)
	}

	logging.Debugf("Loaded secrets from %s with %d entries", path, len(secrets))
	return secrets, nil
}

// mergeConfigs merges two configurations, with project config overriding global config
func mergeConfigs(global, project *Config) *Config {
	merged := &Config{
		Endpoints: make(map[string]Endpoint),
	}

	// Copy global endpoints first
	if global != nil {
		for name, endpoint := range global.Endpoints {
			merged.Endpoints[name] = endpoint
		}
		if global.LogFile != "" {
			merged.LogFile = global.LogFile
		}
	}

	// Override with project endpoints
	if project != nil {
		for name, endpoint := range project.Endpoints {
			merged.Endpoints[name] = endpoint
		}
		if project.LogFile != "" {
			merged.LogFile = project.LogFile
		}
	}

	return merged
}

// mergeSecrets merges two secrets maps, with project secrets overriding global secrets
func mergeSecrets(global, project Secrets) Secrets {
	merged := make(Secrets)

	// Copy global secrets first
	if global != nil {
		for key, value := range global {
			merged[key] = value
		}
	}

	// Override with project secrets
	if project != nil {
		for key, value := range project {
			merged[key] = value
		}
	}

	return merged
}

// LoadConfigHierarchical loads configuration from both global and project sources
func LoadConfigHierarchical() (*HierarchicalLoadResult, error) {
	result := &HierarchicalLoadResult{}

	// Get file paths
	globalConfigPath := getGlobalConfigPath()
	globalSecretsPath := getGlobalSecretsPath()
	projectConfigPath := getProjectConfigPath()
	projectSecretsPath := getProjectSecretsPath()

	// Initialize file info
	result.GlobalFiles = ConfigFileInfo{Path: globalConfigPath}
	result.ProjectFiles = ConfigFileInfo{Path: projectConfigPath}

	logging.Debugf("Looking for configuration files...")
	logging.Debugf("Global config: %s", globalConfigPath)
	logging.Debugf("Global secrets: %s", globalSecretsPath)
	logging.Debugf("Project config: %s", projectConfigPath)
	logging.Debugf("Project secrets: %s", projectSecretsPath)

	// Load global files
	var globalConfig, projectConfig *Config
	var globalSecrets, projectSecrets Secrets

	if globalConfigPath != "" {
		globalConfig, _ = loadConfigFromFile(globalConfigPath)
		if globalConfig != nil {
			result.GlobalFiles.Loaded = true
			result.GlobalFiles.Endpoints = len(globalConfig.Endpoints)
			logging.Printf("Loaded global config from %s (%d endpoints)", globalConfigPath, len(globalConfig.Endpoints))
		}
	}

	if globalSecretsPath != "" {
		globalSecrets, _ = loadSecretsFromFile(globalSecretsPath)
		if globalSecrets != nil {
			result.GlobalFiles.Secrets = len(globalSecrets)
			logging.Printf("Loaded global secrets from %s (%d entries)", globalSecretsPath, len(globalSecrets))
		}
	}

	// Load project files
	if projectConfigPath != "" {
		projectConfig, _ = loadConfigFromFile(projectConfigPath)
		if projectConfig != nil {
			result.ProjectFiles.Loaded = true
			result.ProjectFiles.Endpoints = len(projectConfig.Endpoints)
			logging.Printf("Loaded project config from %s (%d endpoints)", projectConfigPath, len(projectConfig.Endpoints))
		}
	}

	if projectSecretsPath != "" {
		projectSecrets, _ = loadSecretsFromFile(projectSecretsPath)
		if projectSecrets != nil {
			result.ProjectFiles.Secrets = len(projectSecrets)
			logging.Printf("Loaded project secrets from %s (%d entries)", projectSecretsPath, len(projectSecrets))
		}
	}

	// Merge configurations
	result.Config = mergeConfigs(globalConfig, projectConfig)
	result.Secrets = mergeSecrets(globalSecrets, projectSecrets)

	// Log merging information
	if globalConfig != nil && projectConfig != nil {
		logging.Printf("Merged global and project configurations")
		logging.Debugf("Total endpoints after merge: %d", len(result.Config.Endpoints))
	}

	if globalSecrets != nil && projectSecrets != nil {
		logging.Printf("Merged global and project secrets")
		logging.Debugf("Total secrets after merge: %d", len(result.Secrets))
	}

	return result, nil
}

// validateMergedConfig validates the final merged configuration
func validateMergedConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Allow empty endpoints - this will be handled in main with graceful exit
	for name, endpoint := range config.Endpoints {
		if name == "" {
			return fmt.Errorf("endpoint name cannot be empty")
		}
		if endpoint.UpstreamURL == "" {
			return fmt.Errorf("upstream URL cannot be empty for endpoint '%s'", name)
		}
	}

	return nil
}