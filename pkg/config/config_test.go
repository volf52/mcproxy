package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := map[string]interface{}{
		"endpoints": map[string]interface{}{
			"test": map[string]interface{}{
				"upstreamUrl": "https://example.com",
				"headers": map[string]interface{}{
					"Authorization": "Bearer {{token}}",
				},
			},
		},
		"logFile": "/tmp/test.log",
	}

	data, _ := json.Marshal(configContent)
	os.WriteFile(configPath, data, 0644)

	// Set environment variable
	oldConfig := os.Getenv("MCPROXY_CONFIG")
	os.Setenv("MCPROXY_CONFIG", configPath)
	defer os.Setenv("MCPROXY_CONFIG", oldConfig)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}

	if config.Endpoints["test"].UpstreamURL != "https://example.com" {
		t.Errorf("Expected upstream URL 'https://example.com', got '%s'", config.Endpoints["test"].UpstreamURL)
	}

	if config.LogFile != "/tmp/test.log" {
		t.Errorf("Expected log file '/tmp/test.log', got '%s'", config.LogFile)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	oldConfig := os.Getenv("MCPROXY_CONFIG")
	os.Setenv("MCPROXY_CONFIG", "/nonexistent/config.json")
	defer os.Setenv("MCPROXY_CONFIG", oldConfig)

	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected error for missing config file")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte("invalid json"), 0644)

	oldConfig := os.Getenv("MCPROXY_CONFIG")
	os.Setenv("MCPROXY_CONFIG", configPath)
	defer os.Setenv("MCPROXY_CONFIG", oldConfig)

	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestLoadConfigEmptyEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := map[string]interface{}{
		"endpoints": map[string]interface{}{},
	}

	data, _ := json.Marshal(configContent)
	os.WriteFile(configPath, data, 0644)

	oldConfig := os.Getenv("MCPROXY_CONFIG")
	os.Setenv("MCPROXY_CONFIG", configPath)
	defer os.Setenv("MCPROXY_CONFIG", oldConfig)

	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected error for empty endpoints")
	}
}

func TestLoadSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	secretsPath := filepath.Join(tmpDir, "secrets.json")
	secretsContent := map[string]string{
		"api_token": "secret123",
		"db_password": "pass456",
	}

	data, _ := json.Marshal(secretsContent)
	os.WriteFile(secretsPath, data, 0644)

	oldSecrets := os.Getenv("MCPROXY_SECRETS")
	os.Setenv("MCPROXY_SECRETS", secretsPath)
	defer os.Setenv("MCPROXY_SECRETS", oldSecrets)

	secrets, err := LoadSecrets()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(secrets) != 2 {
		t.Errorf("Expected 2 secrets, got %d", len(secrets))
	}

	if secrets["api_token"] != "secret123" {
		t.Errorf("Expected api_token 'secret123', got '%s'", secrets["api_token"])
	}

	if secrets["db_password"] != "pass456" {
		t.Errorf("Expected db_password 'pass456', got '%s'", secrets["db_password"])
	}
}

func TestLoadSecretsMissingFile(t *testing.T) {
	oldSecrets := os.Getenv("MCPROXY_SECRETS")
	os.Setenv("MCPROXY_SECRETS", "/nonexistent/secrets.json")
	defer os.Setenv("MCPROXY_SECRETS", oldSecrets)

	_, err := LoadSecrets()
	if err == nil {
		t.Error("Expected error for missing secrets file")
	}
}

func TestLoadSecretsInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	secretsPath := filepath.Join(tmpDir, "secrets.json")
	os.WriteFile(secretsPath, []byte("invalid json"), 0644)

	oldSecrets := os.Getenv("MCPROXY_SECRETS")
	os.Setenv("MCPROXY_SECRETS", secretsPath)
	defer os.Setenv("MCPROXY_SECRETS", oldSecrets)

	_, err := LoadSecrets()
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestSubstituteTemplate(t *testing.T) {
	secrets := Secrets{
		"api_token": "secret123",
		"db_pass":   "password456",
	}

	tests := []struct {
		name           string
		input          string
		expected       string
		expectedMissing []string
		expectError    bool
	}{
		{
			name:            "single template",
			input:           "Bearer {{api_token}}",
			expected:        "Bearer secret123",
			expectedMissing: []string{},
			expectError:     false,
		},
		{
			name:            "multiple templates",
			input:           "Token: {{api_token}}, Pass: {{db_pass}}",
			expected:        "Token: secret123, Pass: password456",
			expectedMissing: []string{},
			expectError:     false,
		},
		{
			name:            "missing template variable",
			input:           "Bearer {{missing_token}}",
			expected:        "Bearer ",
			expectedMissing: []string{"missing_token"},
			expectError:     false,
		},
		{
			name:            "no templates",
			input:           "static-value",
			expected:        "static-value",
			expectedMissing: []string{},
			expectError:     false,
		},
		{
			name:            "empty string",
			input:           "",
			expected:        "",
			expectedMissing: []string{},
			expectError:     false,
		},
		{
			name:            "mixed missing and present",
			input:           "{{api_token}}-{{missing}}",
			expected:        "secret123-",
			expectedMissing: []string{"missing"},
			expectError:     false,
		},
		{
			name:            "unclosed template",
			input:           "Bearer {{api_token",
			expected:        "",
			expectedMissing: nil,
			expectError:     true,
		},
		{
			name:            "empty template variable",
			input:           "Bearer {{}}",
			expected:        "",
			expectedMissing: nil,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, missing, err := substituteTemplate(tt.input, secrets)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Expected result '%s', got '%s'", tt.expected, result)
			}

			if !equalSlices(missing, tt.expectedMissing) {
				t.Errorf("Expected missing variables %v, got %v", tt.expectedMissing, missing)
			}
		})
	}
}

func TestProcessSecretTemplates(t *testing.T) {
	config := &Config{
		Endpoints: map[string]Endpoint{
			"valid": {
				UpstreamURL: "https://api.example.com",
				Headers: map[string]string{
					"Authorization": "Bearer {{api_token}}",
					"Content-Type":  "application/json",
				},
			},
			"missing": {
				UpstreamURL: "https://api.example.com",
				Headers: map[string]string{
					"Authorization": "Bearer {{missing_token}}",
				},
			},
			"mixed": {
				UpstreamURL: "https://api.example.com",
				Headers: map[string]string{
					"Good": "Bearer {{api_token}}",
					"Bad":  "Key {{missing_key}}",
				},
			},
		},
	}

	secrets := Secrets{
		"api_token": "secret123",
	}

	processed, skipped := ProcessSecretTemplates(config, secrets)

	// Check processed endpoints
	if len(processed) != 1 {
		t.Errorf("Expected 1 processed endpoint, got %d", len(processed))
	}

	if _, exists := processed["valid"]; !exists {
		t.Error("Expected 'valid' endpoint to be processed")
	}

	// Check skipped endpoints
	if len(skipped) != 2 {
		t.Errorf("Expected 2 skipped endpoints, got %d", len(skipped))
	}

	expectedSkipped := []string{"missing", "mixed"}
	if !equalSlices(skipped, expectedSkipped) {
		t.Errorf("Expected skipped endpoints %v, got %v", expectedSkipped, skipped)
	}

	// Check the processed endpoint has correct headers
	validEndpoint := processed["valid"]
	if validEndpoint.Headers["Authorization"] != "Bearer secret123" {
		t.Errorf("Expected resolved Authorization header, got '%s'", validEndpoint.Headers["Authorization"])
	}
}

// Helper function to compare string slices
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Hierarchical loading tests

func TestGetGlobalConfigPath(t *testing.T) {
	path := getGlobalConfigPath()
	if path == "" {
		t.Error("Expected non-empty global config path")
	}
	if !filepath.IsAbs(path) {
		t.Error("Expected absolute path for global config")
	}
}

func TestGetGlobalSecretsPath(t *testing.T) {
	path := getGlobalSecretsPath()
	if path == "" {
		t.Error("Expected non-empty global secrets path")
	}
	if !filepath.IsAbs(path) {
		t.Error("Expected absolute path for global secrets")
	}
}

func TestGetProjectConfigPath(t *testing.T) {
	// Test default path
	oldConfig := os.Getenv("MCPROXY_CONFIG")
	defer os.Setenv("MCPROXY_CONFIG", oldConfig)

	os.Unsetenv("MCPROXY_CONFIG")
	path := getProjectConfigPath()
	expected := "./config.json"
	if path != expected {
		t.Errorf("Expected project config path '%s', got '%s'", expected, path)
	}

	// Test environment override
	os.Setenv("MCPROXY_CONFIG", "/custom/config.json")
	path = getProjectConfigPath()
	expected = "/custom/config.json"
	if path != expected {
		t.Errorf("Expected project config path '%s', got '%s'", expected, path)
	}
}

func TestGetProjectSecretsPath(t *testing.T) {
	// Test default path
	oldSecrets := os.Getenv("MCPROXY_SECRETS")
	defer os.Setenv("MCPROXY_SECRETS", oldSecrets)

	os.Unsetenv("MCPROXY_SECRETS")
	path := getProjectSecretsPath()
	expected := "./secrets.json"
	if path != expected {
		t.Errorf("Expected project secrets path '%s', got '%s'", expected, path)
	}

	// Test environment override
	os.Setenv("MCPROXY_SECRETS", "/custom/secrets.json")
	path = getProjectSecretsPath()
	expected = "/custom/secrets.json"
	if path != expected {
		t.Errorf("Expected project secrets path '%s', got '%s'", expected, path)
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Test existing file
	testFile := filepath.Join(tmpDir, "test.json")
	os.WriteFile(testFile, []byte("{}"), 0644)
	if !fileExists(testFile) {
		t.Error("Expected file to exist")
	}

	// Test non-existing file
	if fileExists(filepath.Join(tmpDir, "nonexistent.json")) {
		t.Error("Expected file to not exist")
	}

	// Test empty path
	if fileExists("") {
		t.Error("Expected empty path to return false")
	}

	// Test directory path
	if fileExists(tmpDir) {
		t.Error("Expected directory to return false")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test loading existing file
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := map[string]interface{}{
		"endpoints": map[string]interface{}{
			"test": map[string]interface{}{
				"upstreamUrl": "https://example.com",
			},
		},
	}
	data, _ := json.Marshal(configContent)
	os.WriteFile(configPath, data, 0644)

	config, err := loadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if config == nil {
		t.Fatal("Expected config, got nil")
	}
	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}

	// Test non-existing file
	config, err = loadConfigFromFile(filepath.Join(tmpDir, "nonexistent.json"))
	if err != nil {
		t.Errorf("Expected no error for missing file, got %v", err)
	}
	if config != nil {
		t.Error("Expected nil config for missing file")
	}
}

func TestLoadSecretsFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test loading existing file
	secretsPath := filepath.Join(tmpDir, "secrets.json")
	secretsContent := map[string]string{
		"token": "secret123",
	}
	data, _ := json.Marshal(secretsContent)
	os.WriteFile(secretsPath, data, 0644)

	secrets, err := loadSecretsFromFile(secretsPath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if secrets == nil {
		t.Fatal("Expected secrets, got nil")
	}
	if len(secrets) != 1 {
		t.Errorf("Expected 1 secret, got %d", len(secrets))
	}

	// Test non-existing file
	secrets, err = loadSecretsFromFile(filepath.Join(tmpDir, "nonexistent.json"))
	if err != nil {
		t.Errorf("Expected no error for missing file, got %v", err)
	}
	if secrets != nil {
		t.Error("Expected nil secrets for missing file")
	}
}

func TestMergeConfigs(t *testing.T) {
	globalConfig := &Config{
		Endpoints: map[string]Endpoint{
			"global-only": {
				UpstreamURL: "https://global.com",
				Headers:     map[string]string{"Global": "true"},
			},
			"shared": {
				UpstreamURL: "https://global-shared.com",
				Headers:     map[string]string{"Source": "global"},
			},
		},
		LogFile: "/var/log/global.log",
	}

	projectConfig := &Config{
		Endpoints: map[string]Endpoint{
			"project-only": {
				UpstreamURL: "https://project.com",
				Headers:     map[string]string{"Project": "true"},
			},
			"shared": {
				UpstreamURL: "https://project-shared.com",
				Headers:     map[string]string{"Source": "project"},
			},
		},
		LogFile: "/var/log/project.log",
	}

	merged := mergeConfigs(globalConfig, projectConfig)

	// Check that all endpoints are present
	if len(merged.Endpoints) != 3 {
		t.Errorf("Expected 3 merged endpoints, got %d", len(merged.Endpoints))
	}

	// Check global-only endpoint
	if merged.Endpoints["global-only"].UpstreamURL != "https://global.com" {
		t.Error("Global-only endpoint not preserved correctly")
	}

	// Check project-only endpoint
	if merged.Endpoints["project-only"].UpstreamURL != "https://project.com" {
		t.Error("Project-only endpoint not added correctly")
	}

	// Check that project overrides global for shared endpoint
	if merged.Endpoints["shared"].UpstreamURL != "https://project-shared.com" {
		t.Error("Project config should override global config for shared endpoint")
	}
	if merged.Endpoints["shared"].Headers["Source"] != "project" {
		t.Error("Project headers should override global headers")
	}

	// Check that project log file overrides global
	if merged.LogFile != "/var/log/project.log" {
		t.Error("Project log file should override global log file")
	}
}

func TestMergeSecrets(t *testing.T) {
	globalSecrets := Secrets{
		"global-only": "global-value",
		"shared":      "global-shared",
	}

	projectSecrets := Secrets{
		"project-only": "project-value",
		"shared":       "project-shared",
	}

	merged := mergeSecrets(globalSecrets, projectSecrets)

	// Check that all secrets are present
	if len(merged) != 3 {
		t.Errorf("Expected 3 merged secrets, got %d", len(merged))
	}

	// Check global-only secret
	if merged["global-only"] != "global-value" {
		t.Error("Global-only secret not preserved correctly")
	}

	// Check project-only secret
	if merged["project-only"] != "project-value" {
		t.Error("Project-only secret not added correctly")
	}

	// Check that project overrides global for shared secret
	if merged["shared"] != "project-shared" {
		t.Error("Project secrets should override global secrets for shared keys")
	}
}

func TestLoadConfigHierarchical(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up global config files in the temp directory to simulate HOME
	homeDir := tmpDir
	globalConfigPath := filepath.Join(homeDir, "config.json")
	globalSecretsPath := filepath.Join(homeDir, "secrets.json")

	globalConfigContent := map[string]interface{}{
		"endpoints": map[string]interface{}{
			"global": map[string]interface{}{
				"upstreamUrl": "https://global.com",
			},
		},
	}
	data, _ := json.Marshal(globalConfigContent)
	os.WriteFile(globalConfigPath, data, 0644)

	globalSecretsContent := map[string]string{
		"global_token": "global-secret",
	}
	data, _ = json.Marshal(globalSecretsContent)
	os.WriteFile(globalSecretsPath, data, 0644)

	// Set up project config
	projectConfigPath := filepath.Join(tmpDir, "project_config.json")
	projectConfigContent := map[string]interface{}{
		"endpoints": map[string]interface{}{
			"project": map[string]interface{}{
				"upstreamUrl": "https://project.com",
			},
		},
	}
	data, _ = json.Marshal(projectConfigContent)
	os.WriteFile(projectConfigPath, data, 0644)

	// Set up project secrets
	projectSecretsPath := filepath.Join(tmpDir, "project_secrets.json")
	projectSecretsContent := map[string]string{
		"project_token": "project-secret",
	}
	data, _ = json.Marshal(projectSecretsContent)
	os.WriteFile(projectSecretsPath, data, 0644)

	// Mock environment
	originalHome := os.Getenv("HOME")
	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalSecrets := os.Getenv("MCPROXY_SECRETS")

	os.Setenv("HOME", homeDir)
	os.Setenv("MCPROXY_CONFIG", projectConfigPath)
	os.Setenv("MCPROXY_SECRETS", projectSecretsPath)

	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("MCPROXY_CONFIG", originalConfig)
		os.Setenv("MCPROXY_SECRETS", originalSecrets)
	}()

	// Test hierarchical loading
	result, err := LoadConfigHierarchical()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check that both configs were loaded
	if len(result.Config.Endpoints) != 2 {
		t.Errorf("Expected 2 merged endpoints, got %d", len(result.Config.Endpoints))
	}

	// Check that both secrets were loaded
	if len(result.Secrets) != 2 {
		t.Errorf("Expected 2 merged secrets, got %d", len(result.Secrets))
	}

	// Check global endpoint
	if result.Config.Endpoints["global"].UpstreamURL != "https://global.com" {
		t.Error("Global endpoint not loaded correctly")
	}

	// Check project endpoint
	if result.Config.Endpoints["project"].UpstreamURL != "https://project.com" {
		t.Error("Project endpoint not loaded correctly")
	}

	// Check global secret
	if result.Secrets["global_token"] != "global-secret" {
		t.Error("Global secret not loaded correctly")
	}

	// Check project secret
	if result.Secrets["project_token"] != "project-secret" {
		t.Error("Project secret not loaded correctly")
	}
}

func TestLoadConfigHierarchicalWithMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock environment to point to non-existent files
	originalHome := os.Getenv("HOME")
	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalSecrets := os.Getenv("MCPROXY_SECRETS")

	os.Setenv("HOME", tmpDir)
	os.Setenv("MCPROXY_CONFIG", filepath.Join(tmpDir, "nonexistent_config.json"))
	os.Setenv("MCPROXY_SECRETS", filepath.Join(tmpDir, "nonexistent_secrets.json"))

	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("MCPROXY_CONFIG", originalConfig)
		os.Setenv("MCPROXY_SECRETS", originalSecrets)
	}()

	// Test hierarchical loading with no files
	result, err := LoadConfigHierarchical()
	if err != nil {
		t.Fatalf("Expected no error for missing files, got %v", err)
	}

	// Should get empty configuration
	if len(result.Config.Endpoints) != 0 {
		t.Errorf("Expected 0 endpoints for missing files, got %d", len(result.Config.Endpoints))
	}

	if len(result.Secrets) != 0 {
		t.Errorf("Expected 0 secrets for missing files, got %d", len(result.Secrets))
	}
}

func TestValidateMergedConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
		{
			name: "valid empty config",
			config: &Config{
				Endpoints: map[string]Endpoint{},
			},
			expectError: false,
		},
		{
			name: "valid config with endpoint",
			config: &Config{
				Endpoints: map[string]Endpoint{
					"test": {
						UpstreamURL: "https://example.com",
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid endpoint with empty name",
			config: &Config{
				Endpoints: map[string]Endpoint{
					"": {
						UpstreamURL: "https://example.com",
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid endpoint with empty upstream URL",
			config: &Config{
				Endpoints: map[string]Endpoint{
					"test": {
						UpstreamURL: "",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMergedConfig(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up traditional config file for fallback testing
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := map[string]interface{}{
		"endpoints": map[string]interface{}{
			"test": map[string]interface{}{
				"upstreamUrl": "https://example.com",
			},
		},
	}
	data, _ := json.Marshal(configContent)
	os.WriteFile(configPath, data, 0644)

	secretsPath := filepath.Join(tmpDir, "secrets.json")
	secretsContent := map[string]string{
		"token": "secret123",
	}
	data, _ = json.Marshal(secretsContent)
	os.WriteFile(secretsPath, data, 0644)

	// Mock HOME to prevent loading actual user's global secrets
	originalHome := os.Getenv("HOME")
	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalSecrets := os.Getenv("MCPROXY_SECRETS")

	// Use the tmpDir as HOME to avoid loading actual global secrets
	os.Setenv("HOME", tmpDir)
	os.Setenv("MCPROXY_CONFIG", configPath)
	os.Setenv("MCPROXY_SECRETS", secretsPath)

	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("MCPROXY_CONFIG", originalConfig)
		os.Setenv("MCPROXY_SECRETS", originalSecrets)
	}()

	config, secrets, err := LoadConfigWithDefaults()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}

	if len(secrets) != 1 {
		t.Errorf("Expected 1 secret, got %d", len(secrets))
	}
}