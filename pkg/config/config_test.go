package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Helper function for substring matching in tests
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := map[string]interface{}{
		"endpoints": map[string]interface{}{
			"test": map[string]interface{}{
				"url": "https://example.com",
				"headers": map[string]interface{}{
					"Authorization": "Bearer {{token}}",
				},
			},
		},
		"logFile": "/tmp/test.log",
	}

	data, _ := json.Marshal(configContent)
	os.WriteFile(configPath, data, 0o644)

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

	if config.Endpoints["test"].Url != "https://example.com" {
		t.Errorf("Expected upstream URL 'https://example.com', got '%s'", config.Endpoints["test"].Url)
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
	os.WriteFile(configPath, []byte("invalid json"), 0o644)

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
	os.WriteFile(configPath, data, 0o644)

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
		"api_token":   "secret123",
		"db_password": "pass456",
	}

	data, _ := json.Marshal(secretsContent)
	os.WriteFile(secretsPath, data, 0o644)

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
	os.WriteFile(secretsPath, []byte("invalid json"), 0o644)

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
		name            string
		input           string
		expected        string
		expectedMissing []string
		expectError     bool
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
				Url: "https://api.example.com",
				Headers: map[string]string{
					"Authorization": "Bearer {{api_token}}",
					"Content-Type":  "application/json",
				},
			},
			"missing": {
				Url: "https://api.example.com",
				Headers: map[string]string{
					"Authorization": "Bearer {{missing_token}}",
				},
			},
			"mixed": {
				Url: "https://api.example.com",
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
	if !equalSlicesUnordered(skipped, expectedSkipped) {
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

func equalSlicesUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps to count occurrences
	countA := make(map[string]int)
	countB := make(map[string]int)

	for _, item := range a {
		countA[item]++
	}
	for _, item := range b {
		countB[item]++
	}

	// Compare maps
	for item, count := range countA {
		if countB[item] != count {
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
	tmpDir := t.TempDir()

	// Save original environment variables
	originalHome := os.Getenv("HOME")
	originalUserConfigDir := os.Getenv("XDG_CONFIG_HOME")
	originalConfig := os.Getenv("MCPROXY_CONFIG")

	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("XDG_CONFIG_HOME", originalUserConfigDir)
		os.Setenv("MCPROXY_CONFIG", originalConfig)
	}()

	// Test default path when no XDG config files exist
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	os.Unsetenv("MCPROXY_CONFIG")

	// Ensure no config files exist in XDG directory
	path := getProjectConfigPath()
	expected := ".mcproxy/config.json"
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

func TestGetProjectConfigPath_XDGCompliant(t *testing.T) {
	tmpDir := t.TempDir()

	// Save original environment variables
	originalHome := os.Getenv("HOME")
	originalUserConfigDir := os.Getenv("XDG_CONFIG_HOME")
	originalConfig := os.Getenv("MCPROXY_CONFIG")

	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("XDG_CONFIG_HOME", originalUserConfigDir)
		os.Setenv("MCPROXY_CONFIG", originalConfig)
	}()

	// Test XDG config directory priority
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	os.Unsetenv("MCPROXY_CONFIG")

	// Create XDG mcproxy directory
	mcproxyConfigDir := filepath.Join(tmpDir, "mcproxy")
	err := os.MkdirAll(mcproxyConfigDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create XDG config directory: %v", err)
	}

	// Test 1: XDG config.jsonc is preferred
	xdgJSONCPath := filepath.Join(mcproxyConfigDir, "config.jsonc")
	os.WriteFile(xdgJSONCPath, []byte("{}"), 0o644)

	path := getProjectConfigPath()
	if path != xdgJSONCPath {
		t.Errorf("Expected XDG JSONC path '%s', got '%s'", xdgJSONCPath, path)
	}

	// Test 2: XDG config.json fallback
	os.Remove(xdgJSONCPath)
	xdgJSONPath := filepath.Join(mcproxyConfigDir, "config.json")
	os.WriteFile(xdgJSONPath, []byte("{}"), 0o644)

	path = getProjectConfigPath()
	if path != xdgJSONPath {
		t.Errorf("Expected XDG JSON path '%s', got '%s'", xdgJSONPath, path)
	}

	// Test 3: Fallback to project directory config.jsonc
	os.Remove(xdgJSONPath)
	// Change to the temp directory to test project config
	originalWd, _ := os.Getwd()
	tmpTestDir := t.TempDir()
	os.Chdir(tmpTestDir)
	defer os.Chdir(originalWd)

	// Create project JSONC file in .mcproxy directory
	mcproxyDir := filepath.Join(tmpTestDir, ".mcproxy")
	os.MkdirAll(mcproxyDir, 0o755)
	projectJSONCPath := filepath.Join(mcproxyDir, "config.jsonc")
	os.WriteFile(projectJSONCPath, []byte("{}"), 0o644)

	path = getProjectConfigPath()
	expected := ".mcproxy/config.jsonc"
	if path != expected {
		t.Errorf("Expected project JSONC path '%s', got '%s'", expected, path)
	}

	// Test 4: Fallback to project directory config.json
	os.Remove(projectJSONCPath)
	projectJSONPath := filepath.Join(mcproxyDir, "config.json")
	os.WriteFile(projectJSONPath, []byte("{}"), 0o644)

	path = getProjectConfigPath()
	expected = ".mcproxy/config.json"
	if path != expected {
		t.Errorf("Expected project JSON path '%s', got '%s'", expected, path)
	}
}

func TestGetProjectConfigPath_EnvironmentOverride(t *testing.T) {
	// Test that environment variable overrides XDG paths
	tmpDir := t.TempDir()

	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalUserConfigDir := os.Getenv("XDG_CONFIG_HOME")

	defer func() {
		os.Setenv("MCPROXY_CONFIG", originalConfig)
		os.Setenv("XDG_CONFIG_HOME", originalUserConfigDir)
	}()

	// Set up XDG config directory with files
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	mcproxyConfigDir := filepath.Join(tmpDir, "mcproxy")
	os.MkdirAll(mcproxyConfigDir, 0o755)

	xdgJSONCPath := filepath.Join(mcproxyConfigDir, "config.jsonc")
	os.WriteFile(xdgJSONCPath, []byte("{}"), 0o644)

	// Environment variable should override everything
	customPath := filepath.Join(tmpDir, "custom.jsonc")
	os.WriteFile(customPath, []byte("{}"), 0o644)
	os.Setenv("MCPROXY_CONFIG", customPath)

	path := getProjectConfigPath()
	if path != customPath {
		t.Errorf("Expected environment override path '%s', got '%s'", customPath, path)
	}
}

func TestGetProjectConfigPath_HomeDirectoryFallback(t *testing.T) {
	// Test XDG behavior when XDG_CONFIG_HOME is not set (should use HOME/.config)
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	originalUserConfigDir := os.Getenv("XDG_CONFIG_HOME")
	originalConfig := os.Getenv("MCPROXY_CONFIG")

	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("XDG_CONFIG_HOME", originalUserConfigDir)
		os.Setenv("MCPROXY_CONFIG", originalConfig)
	}()

	// Set HOME but not XDG_CONFIG_HOME
	os.Setenv("HOME", tmpDir)
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("MCPROXY_CONFIG")

	// Create HOME/.config/mcproxy directory
	configDir := filepath.Join(tmpDir, ".config", "mcproxy")
	err := os.MkdirAll(configDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create HOME config directory: %v", err)
	}

	homeJSONCPath := filepath.Join(configDir, "config.jsonc")
	os.WriteFile(homeJSONCPath, []byte("{}"), 0o644)

	path := getProjectConfigPath()
	if path != homeJSONCPath {
		t.Errorf("Expected HOME XDG JSONC path '%s', got '%s'", homeJSONCPath, path)
	}
}

func TestGetProjectSecretsPath(t *testing.T) {
	// Test default path
	oldSecrets := os.Getenv("MCPROXY_SECRETS")
	defer os.Setenv("MCPROXY_SECRETS", oldSecrets)

	os.Unsetenv("MCPROXY_SECRETS")
	path := getProjectSecretsPath()
	expected := ".mcproxy/secrets.json"
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

func TestGetProjectSecretsPath_McproxyDirectory(t *testing.T) {
	// Test that .mcproxy directory is checked correctly
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	oldSecrets := os.Getenv("MCPROXY_SECRETS")
	defer os.Setenv("MCPROXY_SECRETS", oldSecrets)
	os.Unsetenv("MCPROXY_SECRETS")

	// Test with .mcproxy directory containing JSONC file
	mcproxyDir := filepath.Join(tmpDir, ".mcproxy")
	os.MkdirAll(mcproxyDir, 0o755)

	secretsJSONCPath := filepath.Join(mcproxyDir, "secrets.jsonc")
	os.WriteFile(secretsJSONCPath, []byte(`{"test": "value"}`), 0o644)

	path := getProjectSecretsPath()
	expected := ".mcproxy/secrets.jsonc"
	if path != expected {
		t.Errorf("Expected .mcproxy JSONC secrets path '%s', got '%s'", expected, path)
	}

	// Test with .mcproxy directory containing JSON file only
	os.Remove(secretsJSONCPath)
	secretsJSONPath := filepath.Join(mcproxyDir, "secrets.json")
	os.WriteFile(secretsJSONPath, []byte(`{"test": "value"}`), 0o644)

	path = getProjectSecretsPath()
	expected = ".mcproxy/secrets.json"
	if path != expected {
		t.Errorf("Expected .mcproxy JSON secrets path '%s', got '%s'", expected, path)
	}

	// Test that old location is not checked
	os.WriteFile(filepath.Join(tmpDir, "secrets.json"), []byte(`{"old": "location"}`), 0o644)
	os.Remove(filepath.Join(mcproxyDir, "secrets.json"))

	path = getProjectSecretsPath()
	expected = ".mcproxy/secrets.json" // Should still return .mcproxy path even if file doesn't exist
	if path != expected {
		t.Errorf("Expected .mcproxy secrets path even when file doesn't exist '%s', got '%s'", expected, path)
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Test existing file
	testFile := filepath.Join(tmpDir, "test.json")
	os.WriteFile(testFile, []byte("{}"), 0o644)
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
				"url": "https://example.com",
			},
		},
	}
	data, _ := json.Marshal(configContent)
	os.WriteFile(configPath, data, 0o644)

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
	os.WriteFile(secretsPath, data, 0o644)

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
				Url:     "https://global.com",
				Headers: map[string]string{"Global": "true"},
			},
			"shared": {
				Url:     "https://global-shared.com",
				Headers: map[string]string{"Source": "global"},
			},
		},
		LogFile: "/var/log/global.log",
	}

	projectConfig := &Config{
		Endpoints: map[string]Endpoint{
			"project-only": {
				Url:     "https://project.com",
				Headers: map[string]string{"Project": "true"},
			},
			"shared": {
				Url:     "https://project-shared.com",
				Headers: map[string]string{"Source": "project"},
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
	if merged.Endpoints["global-only"].Url != "https://global.com" {
		t.Error("Global-only endpoint not preserved correctly")
	}

	// Check project-only endpoint
	if merged.Endpoints["project-only"].Url != "https://project.com" {
		t.Error("Project-only endpoint not added correctly")
	}

	// Check that project overrides global for shared endpoint
	if merged.Endpoints["shared"].Url != "https://project-shared.com" {
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
				"url": "https://global.com",
			},
		},
	}
	data, _ := json.Marshal(globalConfigContent)
	os.WriteFile(globalConfigPath, data, 0o644)

	globalSecretsContent := map[string]string{
		"global_token": "global-secret",
	}
	data, _ = json.Marshal(globalSecretsContent)
	os.WriteFile(globalSecretsPath, data, 0o644)

	// Set up project config in .mcproxy directory
	mcproxyDir := filepath.Join(tmpDir, ".mcproxy")
	os.MkdirAll(mcproxyDir, 0o755)

	projectConfigPath := filepath.Join(mcproxyDir, "config.json")
	projectConfigContent := map[string]interface{}{
		"endpoints": map[string]interface{}{
			"project": map[string]interface{}{
				"url": "https://project.com",
			},
		},
	}
	data, _ = json.Marshal(projectConfigContent)
	os.WriteFile(projectConfigPath, data, 0o644)

	// Set up project secrets in .mcproxy directory
	projectSecretsPath := filepath.Join(mcproxyDir, "secrets.json")
	projectSecretsContent := map[string]string{
		"project_token": "project-secret",
	}
	data, _ = json.Marshal(projectSecretsContent)
	os.WriteFile(projectSecretsPath, data, 0o644)

	// Mock environment - change to temp directory for project files
	originalHome := os.Getenv("HOME")
	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalSecrets := os.Getenv("MCPROXY_SECRETS")
	originalWd, _ := os.Getwd()

	// Change to temp directory so .mcproxy is found
	os.Chdir(tmpDir)
	os.Setenv("HOME", homeDir)
	// Clear env vars to test hierarchical loading
	os.Unsetenv("MCPROXY_CONFIG")
	os.Unsetenv("MCPROXY_SECRETS")

	defer func() {
		os.Chdir(originalWd)
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
	if result.Config.Endpoints["global"].Url != "https://global.com" {
		t.Error("Global endpoint not loaded correctly")
	}

	// Check project endpoint
	if result.Config.Endpoints["project"].Url != "https://project.com" {
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
						Url: "https://example.com",
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
						Url: "https://example.com",
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
						Url: "",
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
				"url": "https://example.com",
			},
		},
	}
	data, _ := json.Marshal(configContent)
	os.WriteFile(configPath, data, 0o644)

	secretsPath := filepath.Join(tmpDir, "secrets.json")
	secretsContent := map[string]string{
		"token": "secret123",
	}
	data, _ = json.Marshal(secretsContent)
	os.WriteFile(secretsPath, data, 0o644)

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

// JSONC Tests

func TestIsJSONCFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"config.json", false},
		{"config.jsonc", true},
		{"config.JSONC", true},
		{"Config.Jsonc", true},
		{"/path/to/config.json", false},
		{"/path/to/config.jsonc", true},
		{"config.jsonc.txt", false},
		{"json", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsJSONCFile(tt.path)
			if result != tt.expected {
				t.Errorf("Expected %v for path '%s', got %v", tt.expected, tt.path, result)
			}
		})
	}
}

func TestUnmarshalWithAutoDetection_JSON(t *testing.T) {
	// Test that standard JSON still works
	jsonContent := `{
		"endpoints": {
			"test": {
				"url": "https://example.com",
				"headers": {
					"Authorization": "Bearer token123"
				}
			}
		},
		"logFile": "/tmp/test.log"
	}`

	var config Config
	err := UnmarshalWithAutoDetection([]byte(jsonContent), &config, "config.json")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}

	if config.Endpoints["test"].Url != "https://example.com" {
		t.Errorf("Expected upstream URL 'https://example.com', got '%s'", config.Endpoints["test"].Url)
	}

	if config.LogFile != "/tmp/test.log" {
		t.Errorf("Expected log file '/tmp/test.log', got '%s'", config.LogFile)
	}
}

func TestUnmarshalWithAutoDetection_JSONC(t *testing.T) {
	// Test JSONC with single-line comments
	jsoncContent := `{
		// This is a single-line comment
		"endpoints": {
			"test": {
				"url": "https://example.com", // inline comment
				"headers": {
					"Authorization": "Bearer token123"
				}
			}
		},
		"logFile": "/tmp/test.log" // trailing comment
	}`

	var config Config
	err := UnmarshalWithAutoDetection([]byte(jsoncContent), &config, "config.jsonc")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}

	if config.Endpoints["test"].Url != "https://example.com" {
		t.Errorf("Expected upstream URL 'https://example.com', got '%s'", config.Endpoints["test"].Url)
	}

	if config.LogFile != "/tmp/test.log" {
		t.Errorf("Expected log file '/tmp/test.log', got '%s'", config.LogFile)
	}
}

func TestUnmarshalWithAutoDetection_JSONC_MultiLineComments(t *testing.T) {
	// Test JSONC with multi-line comments
	jsoncContent := `{
		/* This is a multi-line comment
		   that spans multiple lines */
		"endpoints": {
			"test": {
				"url": "https://example.com" /* inline multi-line comment */,
				"headers": {
					"Authorization": "Bearer token123"
				}
			}
		}
		/* Another multi-line comment */
	}`

	var config Config
	err := UnmarshalWithAutoDetection([]byte(jsoncContent), &config, "config.jsonc")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}

	if config.Endpoints["test"].Url != "https://example.com" {
		t.Errorf("Expected upstream URL 'https://example.com', got '%s'", config.Endpoints["test"].Url)
	}
}

func TestUnmarshalWithAutoDetection_JSONC_MixedComments(t *testing.T) {
	// Test JSONC with both single-line and multi-line comments
	jsoncContent := `{
		// Configuration for mcproxy service
		"endpoints": {
			"api": {
				"url": "https://api.example.com",
				"headers": {
					/* Authentication header required for all requests */
					"Authorization": "Bearer {{api_token}}",
					"Content-Type": "application/json" // content type
				}
			},
			"webhook": {
				"url": "https://webhook.example.com",
				// No custom headers needed for webhook
				"headers": {}
			}
		},
		"logFile": "/var/log/mcproxy.log" // log file location
	}`

	var config Config
	err := UnmarshalWithAutoDetection([]byte(jsoncContent), &config, "config.jsonc")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(config.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(config.Endpoints))
	}

	// Check API endpoint
	if config.Endpoints["api"].Url != "https://api.example.com" {
		t.Errorf("Expected API upstream URL 'https://api.example.com', got '%s'", config.Endpoints["api"].Url)
	}

	if config.Endpoints["api"].Headers["Authorization"] != "Bearer {{api_token}}" {
		t.Errorf("Expected Authorization header 'Bearer {{api_token}}', got '%s'", config.Endpoints["api"].Headers["Authorization"])
	}

	// Check webhook endpoint
	if config.Endpoints["webhook"].Url != "https://webhook.example.com" {
		t.Errorf("Expected webhook upstream URL 'https://webhook.example.com', got '%s'", config.Endpoints["webhook"].Url)
	}

	if config.LogFile != "/var/log/mcproxy.log" {
		t.Errorf("Expected log file '/var/log/mcproxy.log', got '%s'", config.LogFile)
	}
}

func TestLoadConfig_JSONC(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.jsonc")

	// JSONC content with comments
	jsoncContent := `{
		// API endpoint configuration
		"endpoints": {
			"test": {
				"url": "https://example.com", // API server URL
				"headers": {
					"Authorization": "Bearer {{token}}" // auth token
				}
			}
		},
		"logFile": "/tmp/test.log" // log file location
	}`

	os.WriteFile(configPath, []byte(jsoncContent), 0o644)

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

	if config.Endpoints["test"].Url != "https://example.com" {
		t.Errorf("Expected upstream URL 'https://example.com', got '%s'", config.Endpoints["test"].Url)
	}

	if config.LogFile != "/tmp/test.log" {
		t.Errorf("Expected log file '/tmp/test.log', got '%s'", config.LogFile)
	}
}

func TestLoadSecrets_JSONC(t *testing.T) {
	tmpDir := t.TempDir()
	secretsPath := filepath.Join(tmpDir, "secrets.jsonc")

	// JSONC content with comments
	jsoncContent := `{
		// API secrets
		"api_token":   "secret123",       // main API token
		"db_password": "pass456",         /* database password
		                                    used for connections */
		"webhook_secret": "webhook789"   // webhook verification secret
	}`

	os.WriteFile(secretsPath, []byte(jsoncContent), 0o644)

	oldSecrets := os.Getenv("MCPROXY_SECRETS")
	os.Setenv("MCPROXY_SECRETS", secretsPath)
	defer os.Setenv("MCPROXY_SECRETS", oldSecrets)

	secrets, err := LoadSecrets()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(secrets) != 3 {
		t.Errorf("Expected 3 secrets, got %d", len(secrets))
	}

	if secrets["api_token"] != "secret123" {
		t.Errorf("Expected api_token 'secret123', got '%s'", secrets["api_token"])
	}

	if secrets["db_password"] != "pass456" {
		t.Errorf("Expected db_password 'pass456', got '%s'", secrets["db_password"])
	}

	if secrets["webhook_secret"] != "webhook789" {
		t.Errorf("Expected webhook_secret 'webhook789', got '%s'", secrets["webhook_secret"])
	}
}

func TestLoadConfigHierarchical_JSONC(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up global JSONC config file
	globalConfigPath := filepath.Join(tmpDir, "config.jsonc")
	globalConfigContent := `{
		// Global configuration
		"endpoints": {
			"global": {
				"url": "https://global.com"
			}
		}
	}`
	os.WriteFile(globalConfigPath, []byte(globalConfigContent), 0o644)

	// Set up project JSONC config file in .mcproxy directory
	mcproxyDir := filepath.Join(tmpDir, ".mcproxy")
	os.MkdirAll(mcproxyDir, 0o755)

	projectConfigPath := filepath.Join(mcproxyDir, "config.jsonc")
	projectConfigContent := `{
		// Project-specific configuration
		"endpoints": {
			"project": {
				"url": "https://project.com"
			}
		}
	}`
	os.WriteFile(projectConfigPath, []byte(projectConfigContent), 0o644)

	// Set up global JSONC secrets file
	globalSecretsPath := filepath.Join(tmpDir, "secrets.jsonc")
	globalSecretsContent := `{
		// Global secrets
		"global_token": "global-secret"
	}`
	os.WriteFile(globalSecretsPath, []byte(globalSecretsContent), 0o644)

	// Set up project JSONC secrets file
	projectSecretsPath := filepath.Join(mcproxyDir, "secrets.jsonc")
	projectSecretsContent := `{
		// Project secrets
		"project_token": "project-secret"
	}`
	os.WriteFile(projectSecretsPath, []byte(projectSecretsContent), 0o644)

	// Mock environment
	originalHome := os.Getenv("HOME")
	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalSecrets := os.Getenv("MCPROXY_SECRETS")
	originalWd, _ := os.Getwd()

	// Change to temp directory so .mcproxy is found
	os.Chdir(tmpDir)
	os.Setenv("HOME", tmpDir)
	// Clear env vars to test hierarchical loading
	os.Unsetenv("MCPROXY_CONFIG")
	os.Unsetenv("MCPROXY_SECRETS")

	defer func() {
		os.Chdir(originalWd)
		os.Setenv("HOME", originalHome)
		os.Setenv("MCPROXY_CONFIG", originalConfig)
		os.Setenv("MCPROXY_SECRETS", originalSecrets)
	}()

	// Test hierarchical loading with JSONC files
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
	if result.Config.Endpoints["global"].Url != "https://global.com" {
		t.Error("Global endpoint not loaded correctly")
	}

	// Check project endpoint
	if result.Config.Endpoints["project"].Url != "https://project.com" {
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

func TestUnmarshalWithAutoDetection_InvalidJSONC(t *testing.T) {
	// Test invalid JSONC (unterminated multi-line comment)
	invalidJSONC := `{
		"endpoints": {
			"test": {
				"url": "https://example.com"
			}
		}
		/* Unterminated comment
	}`

	var config Config
	err := UnmarshalWithAutoDetection([]byte(invalidJSONC), &config, "config.jsonc")
	if err == nil {
		t.Error("Expected error for invalid JSONC, got nil")
	}
}

func TestUnmarshalWithAutoDetection_BackwardCompatibility(t *testing.T) {
	// Test that unknown extensions default to JSON parser
	jsonContent := `{
		"endpoints": {
			"test": {
				"url": "https://example.com"
			}
		}
	}`

	var config Config
	err := UnmarshalWithAutoDetection([]byte(jsonContent), &config, "config.txt")
	if err != nil {
		t.Fatalf("Expected no error for unknown extension, got %v", err)
	}

	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}
}

func TestLoadConfigWithDefaults_ExclusiveModeErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Save original environment variables
	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalSecrets := os.Getenv("MCPROXY_SECRETS")

	defer func() {
		os.Setenv("MCPROXY_CONFIG", originalConfig)
		os.Setenv("MCPROXY_SECRETS", originalSecrets)
	}()

	// Test with invalid config file via environment variable
	configPath := filepath.Join(tmpDir, "invalid.json")
	os.WriteFile(configPath, []byte("{ invalid json"), 0644)
	os.Setenv("MCPROXY_CONFIG", configPath)

	_, _, err := LoadConfigWithDefaults()
	if err == nil {
		t.Error("Expected error for invalid config file in exclusive mode")
	}
	if !contains(err.Error(), "failed to load exclusive config") {
		t.Errorf("Expected config error, got: %v", err)
	}
}

func TestLoadConfigHierarchical_ExclusiveEnv(t *testing.T) {
	// Setup temp directories
	tempHome := t.TempDir()
	tempConfigDir := t.TempDir()

	// Save original environment variables
	originalHome := os.Getenv("HOME")
	originalUserConfigDir := os.Getenv("XDG_CONFIG_HOME")
	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalSecrets := os.Getenv("MCPROXY_SECRETS")

	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("XDG_CONFIG_HOME", originalUserConfigDir)
		os.Setenv("MCPROXY_CONFIG", originalConfig)
		os.Setenv("MCPROXY_SECRETS", originalSecrets)
	}()

	// Set up environment
	os.Setenv("HOME", tempHome)
	os.Setenv("XDG_CONFIG_HOME", tempConfigDir)

	// Create global config file in HOME directory
	globalConfig := Config{
		Endpoints: map[string]Endpoint{
			"global-ep": {
				Url: "http://global.example.com",
				Headers: map[string]string{
					"Global": "true",
				},
			},
		},
	}
	globalConfigPath := filepath.Join(tempHome, "config.json")
	globalConfigData, _ := json.Marshal(globalConfig)
	os.WriteFile(globalConfigPath, globalConfigData, 0o644)

	// Create global secrets file in HOME directory
	globalSecrets := Secrets{
		"global_secret": "global_value",
	}
	globalSecretsPath := filepath.Join(tempHome, "secrets.json")
	globalSecretsData, _ := json.Marshal(globalSecrets)
	os.WriteFile(globalSecretsPath, globalSecretsData, 0o644)

	// Create specific config file (for testing exclusive mode)
	specificConfig := Config{
		Endpoints: map[string]Endpoint{
			"specific-ep": {
				Url: "http://specific.example.com",
				Headers: map[string]string{
					"Specific": "true",
				},
			},
		},
	}
	specificConfigPath := filepath.Join(tempConfigDir, "custom.json")
	specificConfigData, _ := json.Marshal(specificConfig)
	os.WriteFile(specificConfigPath, specificConfigData, 0o644)

	// Create specific secrets file (for testing exclusive mode)
	specificSecrets := Secrets{
		"specific_secret": "specific_value",
	}
	specificSecretsPath := filepath.Join(tempConfigDir, "custom_secrets.json")
	specificSecretsData, _ := json.Marshal(specificSecrets)
	os.WriteFile(specificSecretsPath, specificSecretsData, 0o644)

	// Also create XDG config files for hierarchical testing
	xdgMcproxyDir := filepath.Join(tempConfigDir, "mcproxy")
	os.MkdirAll(xdgMcproxyDir, 0o755)

	xdgConfigPath := filepath.Join(xdgMcproxyDir, "config.json")
	os.WriteFile(xdgConfigPath, specificConfigData, 0o644)

	xdgSecretsPath := filepath.Join(xdgMcproxyDir, "secrets.json")
	os.WriteFile(xdgSecretsPath, specificSecretsData, 0o644)

	// Test with MCPROXY_CONFIG set (exclusive mode)
	os.Setenv("MCPROXY_CONFIG", specificConfigPath)
	os.Unsetenv("MCPROXY_SECRETS") // Let secrets load hierarchically for now

	result, err := LoadConfigHierarchical()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Assert global config is NOT loaded
	if _, exists := result.Config.Endpoints["global-ep"]; exists {
		t.Errorf("Expected global endpoint to be ignored when MCPROXY_CONFIG is set")
	}

	// Assert specific config IS loaded
	if _, exists := result.Config.Endpoints["specific-ep"]; !exists {
		t.Errorf("Expected specific endpoint to be present")
	}

	// Check that global files are not loaded
	if result.GlobalFiles.Loaded {
		t.Errorf("Expected GlobalFiles.Loaded to be false when MCPROXY_CONFIG is set")
	}

	// Check that project files path matches the env var
	if result.ProjectFiles.Path != specificConfigPath {
		t.Errorf("Expected ProjectFiles.Path to be '%s', got '%s'", specificConfigPath, result.ProjectFiles.Path)
	}

	// Now test with MCPROXY_SECRETS set (exclusive mode)
	os.Unsetenv("MCPROXY_CONFIG")
	os.Setenv("MCPROXY_SECRETS", specificSecretsPath)

	result, err = LoadConfigHierarchical()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Assert global secrets are NOT loaded
	if _, exists := result.Secrets["global_secret"]; exists {
		t.Errorf("Expected global secret to be ignored when MCPROXY_SECRETS is set")
	}

	// Assert specific secrets ARE loaded
	if _, exists := result.Secrets["specific_secret"]; !exists {
		t.Errorf("Expected specific secret to be present")
	}

	// Check that global files have no secrets loaded
	if result.GlobalFiles.Secrets > 0 {
		t.Errorf("Expected GlobalFiles.Secrets to be 0 when MCPROXY_SECRETS is set")
	}

	// Test with both env vars set (both exclusive)
	os.Setenv("MCPROXY_CONFIG", specificConfigPath)
	os.Setenv("MCPROXY_SECRETS", specificSecretsPath)

	result, err = LoadConfigHierarchical()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Assert only specific configs are loaded
	if len(result.Config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(result.Config.Endpoints))
	}
	if _, exists := result.Config.Endpoints["specific-ep"]; !exists {
		t.Errorf("Expected specific endpoint to be present")
	}

	// Assert only specific secrets are loaded
	if len(result.Secrets) != 1 {
		t.Errorf("Expected 1 secret, got %d", len(result.Secrets))
	}
	if _, exists := result.Secrets["specific_secret"]; !exists {
		t.Errorf("Expected specific secret to be present")
	}

	// Test with no env vars (hierarchical loading)
	os.Unsetenv("MCPROXY_CONFIG")
	os.Unsetenv("MCPROXY_SECRETS")

	result, err = LoadConfigHierarchical()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Assert both global and specific configs are loaded via XDG
	// (Note: this tests the XDG path since that's where our specific config was created)
	if _, exists := result.Config.Endpoints["global-ep"]; !exists {
		t.Errorf("Expected global endpoint to be present in hierarchical mode")
	}
	if _, exists := result.Config.Endpoints["specific-ep"]; !exists {
		t.Errorf("Expected specific endpoint to be present in hierarchical mode")
	}
}
