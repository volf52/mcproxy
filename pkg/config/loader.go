package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"mcproxy/pkg/logging"

	"github.com/marcozac/go-jsonc"
)

// ConfigFileInfo holds information about loaded configuration files
type ConfigFileInfo struct {
	Path      string
	Loaded    bool
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
// Checks for .jsonc first, falls back to .json
func getGlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logging.Printf("Warning: Failed to get user home directory: %v", err)
		return ""
	}

	if fileExists(filepath.Join(home, "config.jsonc")) {
		return filepath.Join(home, "config.jsonc")
	}
	// Return default JSON path even if it doesn't exist
	return filepath.Join(home, "config.json")
}

// getGlobalSecretsPath returns the path to the global secrets file
// Checks for .jsonc first, falls back to .json
func getGlobalSecretsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logging.Printf("Warning: Failed to get user home directory: %v", err)
		return ""
	}

	if fileExists(filepath.Join(home, "secrets.jsonc")) {
		return filepath.Join(home, "secrets.jsonc")
	}
	// Return default JSON path even if it doesn't exist
	return filepath.Join(home, "secrets.json")
}

// getProjectConfigPath returns the path to the project config file
// Checks XDG config directory first, then .mcproxy directory, with .jsonc preference
func getProjectConfigPath() string {
	configPath := os.Getenv("MCPROXY_CONFIG")
	if configPath != "" {
		return configPath
	}

	// Check XDG directory first
	if xdgPath := getXDGConfigPath(); xdgPath != "" {
		return xdgPath
	}

	// Check .mcproxy directory
	mcproxyDir := ".mcproxy"
	// Check if directory exists (not a file)
	if _, err := os.Stat(mcproxyDir); os.IsNotExist(err) {
		// Directory doesn't exist, return default path
		return filepath.Join(mcproxyDir, "config.json")
	}
	if fileExists(filepath.Join(mcproxyDir, "config.jsonc")) {
		return filepath.Join(mcproxyDir, "config.jsonc")
	}
	// Return default JSON path even if it doesn't exist
	return filepath.Join(mcproxyDir, "config.json")
}

// getProjectSecretsPath returns the path to the project secrets file
// Checks .mcproxy directory only, with .jsonc preference
func getProjectSecretsPath() string {
	secretsPath := os.Getenv("MCPROXY_SECRETS")
	if secretsPath != "" {
		return secretsPath
	}

	// Check .mcproxy directory
	mcproxyDir := ".mcproxy"
	// Check if directory exists (not a file)
	if _, err := os.Stat(mcproxyDir); os.IsNotExist(err) {
		// Directory doesn't exist, return default path
		return filepath.Join(mcproxyDir, "secrets.json")
	}
	if fileExists(filepath.Join(mcproxyDir, "secrets.jsonc")) {
		return filepath.Join(mcproxyDir, "secrets.jsonc")
	}
	// Return default JSON path even if it doesn't exist
	return filepath.Join(mcproxyDir, "secrets.json")
}

// fileExists checks if a file exists and is not a directory
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// IsJSONCFile checks if a file has .jsonc extension
func IsJSONCFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".jsonc")
}

// findConfigFile looks for config files in a directory, preferring .jsonc over .json
func findConfigFile(baseDir, filename string) string {
	if baseDir == "" || filename == "" {
		return ""
	}

	jsoncPath := filepath.Join(baseDir, filename+".jsonc")
	if fileExists(jsoncPath) {
		return jsoncPath
	}

	jsonPath := filepath.Join(baseDir, filename+".json")
	if fileExists(jsonPath) {
		return jsonPath
	}

	return ""
}

// getXDGConfigPath returns the XDG-compliant config path
// Checks ~/.config/mcproxy/ for config files
func getXDGConfigPath() string {
	userConfigDir, err := os.UserConfigDir()
	if err == nil {
		mcproxyConfigDir := filepath.Join(userConfigDir, "mcproxy")
		return findConfigFile(mcproxyConfigDir, "config")
	}
	logging.Debugf("Warning: Failed to get user config directory: %v", err)
	return ""
}

// UnmarshalWithAutoDetection automatically detects JSON or JSONC format and unmarshals accordingly
func UnmarshalWithAutoDetection(data []byte, v any, path string) error {
	if IsJSONCFile(path) {
		// Use JSONC parser for .jsonc files
		return jsonc.Unmarshal(data, v)
	} else {
		// Use standard JSON parser for .json files and unknown extensions
		return json.Unmarshal(data, v)
	}
}

// loadConfigFromFile loads configuration from a file, returns nil if file doesn't exist
func loadConfigFromFile(path string) (*Config, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File not found is not an error for optional files
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config Config
	if err := UnmarshalWithAutoDetection(data, &config, path); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	logging.Debugf("Loaded config from %s with %d endpoints", path, len(config.Endpoints))
	return &config, nil
}

// loadSecretsFromFile loads secrets from a file, returns nil if file doesn't exist
func loadSecretsFromFile(path string) (Secrets, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File not found is not an error for optional files
		}
		return nil, fmt.Errorf("failed to read secrets file %s: %w", path, err)
	}

	var secrets Secrets
	if err := UnmarshalWithAutoDetection(data, &secrets, path); err != nil {
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

	// Copy global config first
	if global != nil {
		for name, endpoint := range global.Endpoints {
			merged.Endpoints[name] = endpoint
		}
		if global.LogFile != "" {
			merged.LogFile = global.LogFile
		}
		// Copy global server config
		merged.Server = global.Server
		merged.GlobalTimeout = global.GlobalTimeout
		merged.GlobalMaxBodySize = global.GlobalMaxBodySize
	}

	// Override with project config
	if project != nil {
		maps.Copy(merged.Endpoints, project.Endpoints)
		if project.LogFile != "" {
			merged.LogFile = project.LogFile
		}
		// Override server config with project settings
		if project.Server.ReadTimeout != 0 {
			merged.Server.ReadTimeout = project.Server.ReadTimeout
		}
		if project.Server.WriteTimeout != 0 {
			merged.Server.WriteTimeout = project.Server.WriteTimeout
		}
		if project.Server.IdleTimeout != 0 {
			merged.Server.IdleTimeout = project.Server.IdleTimeout
		}
		if project.Server.ShutdownTimeout != 0 {
			merged.Server.ShutdownTimeout = project.Server.ShutdownTimeout
		}
		if project.GlobalTimeout != 0 {
			merged.GlobalTimeout = project.GlobalTimeout
		}
		if project.GlobalMaxBodySize != 0 {
			merged.GlobalMaxBodySize = project.GlobalMaxBodySize
		}
	}

	return merged
}

// mergeSecrets merges two secrets maps, with project secrets overriding global secrets
func mergeSecrets(global, project Secrets) Secrets {
	merged := make(Secrets)

	// Copy global secrets first
	maps.Copy(merged, global)

	// Override with project secrets
	maps.Copy(merged, project)

	return merged
}

// loadConfigFileExclusive loads a config file that was explicitly specified via environment variable.
// Unlike loadConfigFromFile, this function returns errors for all failure cases except empty path.
// This ensures that when users explicitly specify a config file, any errors are reported.
func loadConfigFileExclusive(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}

	// Read the file directly to handle empty files properly
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to load exclusive config from %s: %w", path, err)
	}

	// Check if file is empty
	if len(data) == 0 {
		return nil, fmt.Errorf("empty or invalid config file: %s", path)
	}

	// Try to parse the config
	var config Config
	if err := UnmarshalWithAutoDetection(data, &config, path); err != nil {
		return nil, fmt.Errorf("failed to load exclusive config from %s: %w", path, err)
	}

	logging.Debugf("Loaded exclusive config from %s with %d endpoints", path, len(config.Endpoints))
	return &config, nil
}

// loadSecretsFileExclusive loads a secrets file that was explicitly specified via environment variable.
// Unlike loadSecretsFromFile, this function returns errors for all failure cases except empty path.
// This ensures that when users explicitly specify a secrets file, any errors are reported.
func loadSecretsFileExclusive(path string) (Secrets, error) {
	if path == "" {
		return nil, fmt.Errorf("secrets path cannot be empty")
	}

	// Read the file directly to handle empty files properly
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("secrets file does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to load exclusive secrets from %s: %w", path, err)
	}

	// Check if file is empty
	if len(data) == 0 {
		return nil, fmt.Errorf("empty or invalid secrets file: %s", path)
	}

	// Try to parse the secrets
	var secrets Secrets
	if err := UnmarshalWithAutoDetection(data, &secrets, path); err != nil {
		return nil, fmt.Errorf("failed to load exclusive secrets from %s: %w", path, err)
	}

	logging.Debugf("Loaded exclusive secrets from %s with %d entries", path, len(secrets))
	return secrets, nil
}

// LoadConfigHierarchical loads configuration from both global and project sources
func LoadConfigHierarchical() (*HierarchicalLoadResult, error) {
	result := &HierarchicalLoadResult{}

	// Check env vars directly to determine mode
	envConfigPath := os.Getenv("MCPROXY_CONFIG")
	envSecretsPath := os.Getenv("MCPROXY_SECRETS")

	var globalConfig, projectConfig *Config
	var globalSecrets, projectSecrets Secrets

	// Check if we should use exclusive mode
	// First, check if all specified files exist
	configExists := true
	secretsExists := true

	if envConfigPath != "" {
		if _, err := os.Stat(envConfigPath); err != nil {
			configExists = false
		}
	}

	if envSecretsPath != "" {
		if _, err := os.Stat(envSecretsPath); err != nil {
			secretsExists = false
		}
	}

	// If any environment variable was set and files don't exist, check what to do
	if envConfigPath != "" || envSecretsPath != "" {
		// If one file exists and the other doesn't, return an error
		if (envConfigPath != "" && envSecretsPath != "") && (configExists != secretsExists) {
			if envConfigPath != "" && !configExists {
				return nil, fmt.Errorf("failed to load exclusive secrets: config file exists but secrets file %s does not exist", envSecretsPath)
			}
			if envSecretsPath != "" && !secretsExists {
				return nil, fmt.Errorf("failed to load exclusive secrets: secrets file exists but config file %s does not exist", envConfigPath)
			}
		}

		// If both files don't exist, fall back to hierarchical mode
		if !configExists && !secretsExists {
			// Don't use exclusive mode, will fall back to hierarchical loading
		} else if configExists && secretsExists {
			// Both files exist, use exclusive mode
			if envConfigPath != "" {
				config, loadErr := loadConfigFileExclusive(envConfigPath)
				if loadErr != nil {
					return nil, fmt.Errorf("failed to load exclusive config: %w", loadErr)
				}
				projectConfig = config
				result.ProjectFiles.Path = envConfigPath
				result.ProjectFiles.Loaded = true
				result.ProjectFiles.Endpoints = len(projectConfig.Endpoints)
				logging.Printf("Loaded exclusive config from %s (%d endpoints)", envConfigPath, len(projectConfig.Endpoints))
			}

			if envSecretsPath != "" {
				secrets, loadErr := loadSecretsFileExclusive(envSecretsPath)
				if loadErr != nil {
					return nil, fmt.Errorf("failed to load exclusive secrets: %w", loadErr)
				}
				projectSecrets = secrets
				result.ProjectFiles.Secrets = len(projectSecrets)
				logging.Printf("Loaded exclusive secrets from %s (%d entries)", envSecretsPath, len(projectSecrets))
			}

			// Skip hierarchical loading since we're in exclusive mode
			// Variables are already loaded, skip to merging
		} else {
			// Hierarchical mode: load global and project configs
			globalConfigPath := getGlobalConfigPath()
			projectConfigPath := getProjectConfigPath()

			// Initialize file info
			result.GlobalFiles = ConfigFileInfo{Path: globalConfigPath}
			result.ProjectFiles = ConfigFileInfo{Path: projectConfigPath}

			logging.Debugf("Looking for configuration files...")
			logging.Debugf("Global config: %s", globalConfigPath)
			logging.Debugf("Project config: %s", projectConfigPath)

			// Load global config
			globalConfig, _ = loadConfigFromFile(globalConfigPath)
			if globalConfig != nil {
				result.GlobalFiles.Loaded = true
				result.GlobalFiles.Endpoints = len(globalConfig.Endpoints)
				logging.Printf("Loaded global config from %s (%d endpoints)", globalConfigPath, len(globalConfig.Endpoints))
			}

			// Load project config
			loadedProjectConfig, _ := loadConfigFromFile(projectConfigPath)
			if loadedProjectConfig != nil {
				projectConfig = loadedProjectConfig
				result.ProjectFiles.Loaded = true
				result.ProjectFiles.Endpoints = len(projectConfig.Endpoints)
				logging.Printf("Loaded project config from %s (%d endpoints)", projectConfigPath, len(projectConfig.Endpoints))
			}

			// Hierarchical mode: load global and project secrets
			globalSecretsPath := getGlobalSecretsPath()
			projectSecretsPath := getProjectSecretsPath()

			logging.Debugf("Global secrets: %s", globalSecretsPath)
			logging.Debugf("Project secrets: %s", projectSecretsPath)

			// Load global secrets
			globalSecrets, _ = loadSecretsFromFile(globalSecretsPath)
			if globalSecrets != nil {
				result.GlobalFiles.Secrets = len(globalSecrets)
				logging.Printf("Loaded global secrets from %s (%d entries)", globalSecretsPath, len(globalSecrets))
			}

			// Load project secrets
			loadedProjectSecrets, _ := loadSecretsFromFile(projectSecretsPath)
			if loadedProjectSecrets != nil {
				projectSecrets = loadedProjectSecrets
				result.ProjectFiles.Secrets = len(projectSecrets)
				logging.Printf("Loaded project secrets from %s (%d entries)", projectSecretsPath, len(projectSecrets))
			}
		}
	}

	// Merge configurations
	result.Config = mergeConfigs(globalConfig, projectConfig)
	result.Secrets = mergeSecrets(globalSecrets, projectSecrets)

	// Log merging information
	if globalConfig != nil && projectConfig != nil {
		logging.Printf("Merged global and project configurations")
		logging.Debugf("Total endpoints after merge: %d", len(result.Config.Endpoints))
	} else if envConfigPath != "" && projectConfig != nil {
		logging.Printf("Using exclusive configuration from %s", envConfigPath)
	}

	if globalSecrets != nil && projectSecrets != nil {
		logging.Printf("Merged global and project secrets")
		logging.Debugf("Total secrets after merge: %d", len(result.Secrets))
	} else if envSecretsPath != "" && projectSecrets != nil {
		logging.Printf("Using exclusive secrets from %s", envSecretsPath)
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
		if endpoint.Url == "" {
			return fmt.Errorf("upstream URL cannot be empty for endpoint '%s'", name)
		}
	}

	return nil
}
