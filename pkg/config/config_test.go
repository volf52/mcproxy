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