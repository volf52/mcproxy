package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileExclusive(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		setupFile       func() string
		wantErr         bool
		wantErrContains string
		wantConfig      bool
	}{
		{
			name: "valid config file",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "valid.json")
				content := `{"endpoints": {"test": {"url": "https://example.com"}}}`
				os.WriteFile(path, []byte(content), 0644)
				return path
			},
			wantErr:    false,
			wantConfig: true,
		},
		{
			name: "non-existent file",
			setupFile: func() string {
				return filepath.Join(tmpDir, "nonexistent.json")
			},
			wantErr:         true,
			wantErrContains: "does not exist",
		},
		{
			name: "invalid JSON",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "invalid.json")
				os.WriteFile(path, []byte("{ invalid json"), 0644)
				return path
			},
			wantErr:         true,
			wantErrContains: "failed to load exclusive config",
		},
		{
			name: "empty file",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "empty.json")
				os.WriteFile(path, []byte{}, 0644)
				return path
			},
			wantErr:         true,
			wantErrContains: "empty or invalid",
		},
		{
			name: "empty path",
			setupFile: func() string {
				return ""
			},
			wantErr:         true,
			wantErrContains: "cannot be empty",
		},
		{
			name: "directory instead of file",
			setupFile: func() string {
				return tmpDir
			},
			wantErr:         true,
			wantErrContains: "failed to load exclusive config",
		},
		{
			name: "valid JSONC with comments",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "valid.jsonc")
				content := `{
					// This is a comment
					"endpoints": {
						"test": {
							"url": "https://example.com"
						}
					}
				}`
				os.WriteFile(path, []byte(content), 0644)
				return path
			},
			wantErr:    false,
			wantConfig: true,
		},
		{
			name: "config with only endpoints",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "endpoints-only.json")
				content := `{"endpoints": {}}`
				os.WriteFile(path, []byte(content), 0644)
				return path
			},
			wantErr:    false,
			wantConfig: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFile()

			config, err := loadConfigFileExclusive(path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.wantErrContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
					return
				}
				if !tt.wantConfig && config != nil {
					t.Errorf("Expected no config but got one")
				} else if tt.wantConfig && config == nil {
					t.Errorf("Expected config but got nil")
				}
			}
		})
	}
}

func TestLoadSecretsFileExclusive(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		setupFile       func() string
		wantErr         bool
		wantErrContains string
		wantSecrets     bool
	}{
		{
			name: "valid secrets file",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "valid.json")
				content := `{"token": "secret123", "api_key": "abc123"}`
				os.WriteFile(path, []byte(content), 0644)
				return path
			},
			wantErr:     false,
			wantSecrets: true,
		},
		{
			name: "non-existent file",
			setupFile: func() string {
				return filepath.Join(tmpDir, "nonexistent.json")
			},
			wantErr:         true,
			wantErrContains: "does not exist",
		},
		{
			name: "invalid JSON",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "invalid.json")
				os.WriteFile(path, []byte("{ invalid json"), 0644)
				return path
			},
			wantErr:         true,
			wantErrContains: "failed to load exclusive secrets",
		},
		{
			name: "empty file",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "empty.json")
				os.WriteFile(path, []byte{}, 0644)
				return path
			},
			wantErr:         true,
			wantErrContains: "empty or invalid",
		},
		{
			name: "empty path",
			setupFile: func() string {
				return ""
			},
			wantErr:         true,
			wantErrContains: "cannot be empty",
		},
		{
			name: "empty secrets object",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "empty-object.json")
				content := `{}`
				os.WriteFile(path, []byte(content), 0644)
				return path
			},
			wantErr:     false,
			wantSecrets: true,
		},
		{
			name: "valid JSONC with comments",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "valid.jsonc")
				content := `{
					// API credentials
					"token": "secret123",
					"api_key": "abc123"
				}`
				os.WriteFile(path, []byte(content), 0644)
				return path
			},
			wantErr:     false,
			wantSecrets: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFile()

			secrets, err := loadSecretsFileExclusive(path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.wantErrContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
					return
				}
				if !tt.wantSecrets && secrets != nil {
					t.Errorf("Expected no secrets but got some")
				} else if tt.wantSecrets && secrets == nil {
					t.Errorf("Expected secrets but got nil")
				}
			}
		})
	}
}

func TestLoadConfigHierarchical_ExclusiveModeErrors(t *testing.T) {
	tmpDir := t.TempDir()

	// Save original environment variables
	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalSecrets := os.Getenv("MCPROXY_SECRETS")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("MCPROXY_CONFIG", originalConfig)
		os.Setenv("MCPROXY_SECRETS", originalSecrets)
		os.Setenv("HOME", originalHome)
	}()

	tests := []struct {
		name            string
		setupEnv        func()
		wantErr         bool
		wantErrContains string
	}{
		{
			name: "invalid config file",
			setupEnv: func() {
				path := filepath.Join(tmpDir, "config.json")
				os.WriteFile(path, []byte("{ invalid"), 0644)
				os.Setenv("MCPROXY_CONFIG", path)
				os.Unsetenv("MCPROXY_SECRETS")
			},
			wantErr:         true,
			wantErrContains: "failed to load exclusive config",
		},
		{
			name: "non-existent secrets file",
			setupEnv: func() {
				path := filepath.Join(tmpDir, "config.json")
				os.WriteFile(path, []byte(`{"endpoints": {}}`), 0644)
				os.Setenv("MCPROXY_CONFIG", path)
				os.Setenv("MCPROXY_SECRETS", filepath.Join(tmpDir, "nonexistent.json"))
			},
			wantErr:         true,
			wantErrContains: "failed to load exclusive secrets",
		},
		{
			name: "both files invalid",
			setupEnv: func() {
				os.Setenv("HOME", tmpDir) // Ensure hierarchical mode doesn't pick up files from real home
				os.Setenv("MCPROXY_CONFIG", filepath.Join(tmpDir, "bad1.json"))
				os.Setenv("MCPROXY_SECRETS", filepath.Join(tmpDir, "bad2.json"))
			},
			wantErr: false, // Should fall back to hierarchical mode and return empty config
		},
		{
			name: "both files valid",
			setupEnv: func() {
				configPath := filepath.Join(tmpDir, "config.json")
				os.WriteFile(configPath, []byte(`{"endpoints": {"test": {"url": "https://example.com"}}}`), 0644)
				os.Setenv("MCPROXY_CONFIG", configPath)

				secretsPath := filepath.Join(tmpDir, "secrets.json")
				os.WriteFile(secretsPath, []byte(`{"token": "secret"}`), 0644)
				os.Setenv("MCPROXY_SECRETS", secretsPath)
			},
			wantErr: false,
		},
		{
			name: "config only",
			setupEnv: func() {
				configPath := filepath.Join(tmpDir, "config.json")
				os.WriteFile(configPath, []byte(`{"endpoints": {"test": {"url": "https://example.com"}}}`), 0644)
				os.Setenv("MCPROXY_CONFIG", configPath)
				os.Unsetenv("MCPROXY_SECRETS")
			},
			wantErr: false,
		},
		{
			name: "secrets only",
			setupEnv: func() {
				os.Unsetenv("MCPROXY_CONFIG")
				secretsPath := filepath.Join(tmpDir, "secrets.json")
				os.WriteFile(secretsPath, []byte(`{"token": "secret"}`), 0644)
				os.Setenv("MCPROXY_SECRETS", secretsPath)
			},
			wantErr: false,
		},
		{
			name: "empty config file",
			setupEnv: func() {
				path := filepath.Join(tmpDir, "empty.json")
				os.WriteFile(path, []byte{}, 0644)
				os.Setenv("MCPROXY_CONFIG", path)
			},
			wantErr:         true,
			wantErrContains: "empty or invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()

			_, err := LoadConfigHierarchical()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.wantErrContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestLoadConfigHierarchical_ExclusiveModeEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Save original environment variables
	originalConfig := os.Getenv("MCPROXY_CONFIG")
	originalSecrets := os.Getenv("MCPROXY_SECRETS")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("MCPROXY_CONFIG", originalConfig)
		os.Setenv("MCPROXY_SECRETS", originalSecrets)
		os.Setenv("HOME", originalHome)
	}()

	tests := []struct {
		name            string
		setupEnv        func()
		wantErr         bool
		wantErrContains string
		validateResult  func(*HierarchicalLoadResult, error)
	}{
		{
			name: "config with secrets unset loads config exclusively",
			setupEnv: func() {
				configPath := filepath.Join(tmpDir, "config.json")
				os.WriteFile(configPath, []byte(`{"endpoints": {"test": {"url": "https://example.com"}}}`), 0644)
				os.Setenv("MCPROXY_CONFIG", configPath)
				os.Unsetenv("MCPROXY_SECRETS") // Explicitly unset
			},
			wantErr: false,
			validateResult: func(result *HierarchicalLoadResult, err error) {
				if result == nil {
					t.Error("Expected result to not be nil")
					return
				}
				if result.Config == nil || len(result.Config.Endpoints) != 1 {
					t.Error("Expected 1 endpoint in config")
				}
				if result.Config.Endpoints["test"].Url != "https://example.com" {
					t.Error("Expected endpoint URL to match")
				}
				// Should be in exclusive mode
				if !result.ProjectFiles.Loaded {
					t.Error("Expected project files to be marked as loaded")
				}
			},
		},
		{
			name: "secrets with config unset loads secrets exclusively",
			setupEnv: func() {
				os.Unsetenv("MCPROXY_CONFIG") // Explicitly unset
				secretsPath := filepath.Join(tmpDir, "secrets.json")
				os.WriteFile(secretsPath, []byte(`{"API_TOKEN": "secret123"}`), 0644)
				os.Setenv("MCPROXY_SECRETS", secretsPath)
			},
			wantErr: false,
			validateResult: func(result *HierarchicalLoadResult, err error) {
				if result == nil {
					t.Error("Expected result to not be nil")
					return
				}
				if result.Secrets == nil || len(result.Secrets) != 1 {
					t.Error("Expected 1 secret entry")
				}
				if result.Secrets["API_TOKEN"] != "secret123" {
					t.Error("Expected secret value to match")
				}
				// Should be in exclusive mode
				if result.ProjectFiles.Secrets != 1 {
					t.Error("Expected project secrets count to be 1")
				}
			},
		},
		{
			name: "both env vars set but only config exists - error",
			setupEnv: func() {
				configPath := filepath.Join(tmpDir, "config.json")
				os.WriteFile(configPath, []byte(`{"endpoints": {}}`), 0644)
				os.Setenv("MCPROXY_CONFIG", configPath)
				os.Setenv("MCPROXY_SECRETS", filepath.Join(tmpDir, "nonexistent.json"))
			},
			wantErr:         true,
			wantErrContains: "does not exist",
		},
		{
			name: "both env vars set but only secrets exists - error",
			setupEnv: func() {
				os.Setenv("MCPROXY_CONFIG", filepath.Join(tmpDir, "nonexistent.json"))
				secretsPath := filepath.Join(tmpDir, "secrets.json")
				os.WriteFile(secretsPath, []byte(`{"TOKEN": "value"}`), 0644)
				os.Setenv("MCPROXY_SECRETS", secretsPath)
			},
			wantErr:         true,
			wantErrContains: "does not exist",
		},
		{
			name: "empty string config path with valid secrets",
			setupEnv: func() {
				os.Setenv("MCPROXY_CONFIG", "") // Empty string
				secretsPath := filepath.Join(tmpDir, "secrets.json")
				os.WriteFile(secretsPath, []byte(`{"TOKEN": "value"}`), 0644)
				os.Setenv("MCPROXY_SECRETS", secretsPath)
			},
			wantErr: false,
			validateResult: func(result *HierarchicalLoadResult, err error) {
				if result == nil {
					t.Error("Expected result to not be nil")
					return
				}
				// Should only load secrets
				if result.Secrets == nil || len(result.Secrets) != 1 {
					t.Error("Expected 1 secret entry")
				}
			},
		},
		{
			name: "empty string secrets path with valid config",
			setupEnv: func() {
				configPath := filepath.Join(tmpDir, "config.json")
				os.WriteFile(configPath, []byte(`{"endpoints": {"ep": {"url": "http://test"}}}`), 0644)
				os.Setenv("MCPROXY_CONFIG", configPath)
				os.Setenv("MCPROXY_SECRETS", "") // Empty string
			},
			wantErr: false,
			validateResult: func(result *HierarchicalLoadResult, err error) {
				if result == nil {
					t.Error("Expected result to not be nil")
					return
				}
				// Should only load config
				if result.Config == nil || len(result.Config.Endpoints) != 1 {
					t.Error("Expected 1 endpoint")
				}
			},
		},
		{
			name: "both empty strings falls back to hierarchical",
			setupEnv: func() {
				os.Setenv("MCPROXY_CONFIG", "")
				os.Setenv("MCPROXY_SECRETS", "")
				// Set up hierarchical files
				homeConfigPath := filepath.Join(tmpDir, ".config", "mcproxy", "config.json")
				os.MkdirAll(filepath.Dir(homeConfigPath), 0755)
				os.WriteFile(homeConfigPath, []byte(`{"endpoints": {"hier": {"url": "http://hierarchical"}}}`), 0644)
				os.Setenv("HOME", tmpDir)
			},
			wantErr: false,
			validateResult: func(result *HierarchicalLoadResult, err error) {
				if result == nil {
					t.Error("Expected result to not be nil")
					return
				}
				// Should load hierarchical config (may have merged endpoints)
				if result.Config == nil || len(result.Config.Endpoints) == 0 {
					t.Error("Expected at least 1 endpoint from hierarchical mode")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()

			result, err := LoadConfigHierarchical()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.wantErrContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}

			if tt.validateResult != nil {
				tt.validateResult(result, err)
			}
		})
	}
}
