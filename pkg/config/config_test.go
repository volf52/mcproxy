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