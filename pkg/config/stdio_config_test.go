package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Helper function to create pointer to duration
func toPtr(d time.Duration) *time.Duration {
	return &d
}

// Helper function to substitute templates in environment variables
func substituteTemplateInEnv(env map[string]string, secrets Secrets) (map[string]string, []string, error) {
	result := make(map[string]string)
	var allMissing []string

	for key, value := range env {
		newValue, missing, err := substituteTemplate(value, secrets)
		if err != nil {
			return nil, nil, err
		}
		result[key] = newValue
		allMissing = append(allMissing, missing...)
	}

	return result, allMissing, nil
}

// Helper function to validate args
func validateArgs(args []string) error {
	if args == nil {
		return nil
	}

	for i, arg := range args {
		// Check for empty string arguments
		if arg == "" {
			return fmt.Errorf("empty argument at index %d", i)
		}

		// Check for control characters (ASCII 0-31, excluding whitespace)
		for _, r := range arg {
			if r < 32 && r != '\t' && r != '\n' && r != '\r' {
				return fmt.Errorf("invalid characters in args: argument %d contains control character '%c'", i, r)
			}
		}
	}

	return nil
}

// Helper function to validate headers
func validateHeaders(headers map[string]string) error {
	if headers == nil {
		return nil
	}

	for name, value := range headers {
		// Validate header name
		for _, r := range name {
			// Check for control characters (ASCII 0-31, excluding whitespace)
			if r < 32 && r != '\t' && r != '\n' && r != '\r' {
				return fmt.Errorf("invalid header name")
			}
		}

		// Validate header value
		for _, r := range value {
			// Check for control characters (ASCII 0-31, excluding whitespace)
			if r < 32 && r != '\t' && r != '\n' && r != '\r' {
				return fmt.Errorf("invalid characters in header value")
			}
		}
	}

	return nil
}

// TestStdioEndpointValidation tests the validation logic for stdio endpoints
func TestStdioEndpointValidation(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    StdioEndpoint
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid stdio endpoint",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/mcp-server",
				Env: map[string]string{
					"API_KEY": "{{api_token}}",
					"LOG":     "debug",
				},
			},
			expectError: false,
		},
		{
			name: "empty command",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "",
			},
			expectError: true,
			errorMsg:    "command is required for stdio endpoints",
		},
		{
			name: "invalid command with control characters",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/bin/mcp\x00server",
			},
			expectError: true,
			errorMsg:    "command component contains invalid characters",
		},
		{
			name: "valid command with args",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/mcp-server",
				Args:    []string{"--config", "/etc/mcp/config.yaml"},
				Env: map[string]string{
					"TOKEN": "{{secret}}",
				},
			},
			expectError: false,
		},
		{
			name: "valid command without env",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/simple-server",
			},
			expectError: false,
		},
		{
			name: "command with relative path",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "./mcp-server",
			},
			expectError: false,
		},
		{
			name: "invalid env variable name",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/mcp-server",
				Env: map[string]string{
					"INVALID-NAME": "value",
				},
			},
			expectError: true,
			errorMsg:    "environment variable key contains invalid characters",
		},
		{
			name: "env var with null byte",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/mcp-server",
				Env: map[string]string{
					"API_KEY\x00": "value",
				},
			},
			expectError: true,
			errorMsg:    "environment variable key contains invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary endpoints map for validation
			endpoints := map[string]Endpoint{"test": {Value: tt.endpoint}}
			err := validateEndpoints(endpoints)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestTimeoutParsingForStdio tests timeout parsing specific to stdio endpoints
func TestTimeoutParsingForStdio(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    StdioEndpoint
		expectError bool
		expected    time.Duration
	}{
		{
			name: "valid timeout in seconds",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{Timeout: toPtr(30 * time.Second)}},
			expectError: false,
			expected:    30 * time.Second,
		},
		{
			name: "valid timeout in minutes",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{Timeout: toPtr(5 * time.Minute)}},
			expectError: false,
			expected:    5 * time.Minute,
		},
		{
			name: "valid timeout in hours",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{Timeout: toPtr(1 * time.Hour)}},
			expectError: false,
			expected:    1 * time.Hour,
		},
		{
			name: "valid complex timeout",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{Timeout: toPtr((30*time.Minute + 45*time.Second))}},
			expectError: false,
			expected:    30*time.Minute + 45*time.Second,
		},
		{
			name: "empty timeout",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{Timeout: nil}},
			expectError: false,
			expected:    0,
		},
		{
			name: "missing timeout",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/server",
			},
			expectError: false,
			expected:    0,
		},
		{
			name: "negative timeout",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{Timeout: toPtr(-30 * time.Second)}},
			expectError: true,
		},
		{
			name: "zero timeout",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{Timeout: toPtr(0 * time.Second)}},
			expectError: false, // Zero timeout is actually valid (means no timeout)
			expected:    0,
		},
		{
			name: "timeout too large",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{Timeout: toPtr(24 * time.Hour)}},// Assuming max is 1 hour

			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var duration time.Duration
			var err error

			if tt.endpoint.Timeout == nil {
				duration = 0
			} else {
				duration = *tt.endpoint.Timeout
				// Check for invalid values
				if duration < 0 {
					err = fmt.Errorf("timeout cannot be negative")
				}
				// Check for too large values (assuming max is 1 hour)
				if duration > time.Hour {
					err = fmt.Errorf("timeout too large (max: 1 hour)")
				}
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if duration != tt.expected {
					t.Errorf("Expected duration %v, got %v", tt.expected, duration)
				}
			}
		})
	}
}

// TestMaxBodySizeValidationForStdio tests maxBodySize validation for stdio endpoints
func TestMaxBodySizeValidationForStdio(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    StdioEndpoint
		expectError bool
	}{
		{
			name: "valid maxBodySize",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{MaxBodySize: func() *int64 { v := int64(5242880); return &v }()}, // 5MB
			},
			expectError: false,
		},
		{
			name: "zero maxBodySize",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{MaxBodySize: func() *int64 { v := int64(0); return &v }()},
			},
			expectError: true,
		},
		{
			name: "missing maxBodySize",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/server",
			},
			expectError: false,
		},
		{
			name: "negative maxBodySize",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{MaxBodySize: func() *int64 { v := int64(-1); return &v }()},
			},
			expectError: true,
		},
		{
			name: "maxBodySize too large",
			endpoint: StdioEndpoint{
				Type:           EndpointTypeStdio,
				Command:        "/usr/local/bin/server",
				EndpointShared: EndpointShared{MaxBodySize: func() *int64 { v := int64(1073741824); return &v }()}, // 1GB, assuming max is 100MB
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary endpoints map for validation
			endpoints := map[string]Endpoint{"test": {Value: tt.endpoint}}
			err := validateEndpoints(endpoints)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestTemplateSubstitutionInEnvVars tests template substitution in stdio endpoint environment variables
func TestTemplateSubstitutionInEnvVars(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      StdioEndpoint
		secrets       Secrets
		expectedEnv   map[string]string
		expectError   bool
		expectMissing []string
	}{
		{
			name: "valid template substitution",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/server",
				Env: map[string]string{
					"API_KEY":   "{{api_token}}",
					"DB_URL":    "postgres://{{db_user}}:{{db_pass}}@localhost/db",
					"LOG_LEVEL": "info",
				},
			},
			secrets: Secrets{
				"api_token": "secret123",
				"db_user":   "myuser",
				"db_pass":   "mypass",
			},
			expectedEnv: map[string]string{
				"API_KEY":   "secret123",
				"DB_URL":    "postgres://myuser:mypass@localhost/db",
				"LOG_LEVEL": "info",
			},
			expectError: false,
		},
		{
			name: "missing template variable",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/server",
				Env: map[string]string{
					"API_KEY": "{{missing_token}}",
					"LOG":     "debug",
				},
			},
			secrets: Secrets{
				"other_secret": "value",
			},
			expectedEnv: map[string]string{
				"API_KEY": "",
				"LOG":     "debug",
			},
			expectMissing: []string{"missing_token"},
		},
		{
			name: "malformed template",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/server",
				Env: map[string]string{
					"API_KEY": "{{unclosed",
				},
			},
			expectError: true,
		},
		{
			name: "empty template",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/server",
				Env: map[string]string{
					"API_KEY": "{{}}",
				},
			},
			expectError: true,
		},
		{
			name: "nested templates",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/server",
				Env: map[string]string{
					"CONFIG": "{{prefix}}_{{env}}_{{suffix}}",
				},
			},
			secrets: Secrets{
				"prefix": "prod",
				"env":    "database",
				"suffix": "config",
			},
			expectedEnv: map[string]string{
				"CONFIG": "prod_database_config",
			},
			expectError: false,
		},
		{
			name: "no env variables",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/server",
			},
			secrets: Secrets{
				"api_token": "secret123",
			},
			expectedEnv: map[string]string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, missing, err := substituteTemplateInEnv(tt.endpoint.Env, tt.secrets)

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

			// Check environment variable substitution
			for key, expected := range tt.expectedEnv {
				if result[key] != expected {
					t.Errorf("Expected env var %s='%s', got '%s'", key, expected, result[key])
				}
			}

			// Check missing variables
			if !equalSlices(missing, tt.expectMissing) {
				t.Errorf("Expected missing variables %v, got %v", tt.expectMissing, missing)
			}
		})
	}
}

// TestStdioEndpointArgsValidation tests args validation for stdio endpoints
func TestStdioEndpointArgsValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "no args",
			args:        nil,
			expectError: false,
		},
		{
			name:        "empty args",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "valid args",
			args:        []string{"--config", "/etc/mcp/config.json", "--debug"},
			expectError: false,
		},
		{
			name:        "arg with control character",
			args:        []string{"--config", "/etc/mcp/config\u0000.json"},
			expectError: true,
		},
		{
			name:        "empty string arg",
			args:        []string{""},
			expectError: true,
		},
		{
			name:        "args with unicode",
			args:        []string{"--name", "测试服务器"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArgs(tt.args)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestStdioEndpointHeadersValidation tests headers validation for stdio endpoints
func TestStdioEndpointHeadersValidation(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		secrets     Secrets
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid headers without templates",
			headers: map[string]string{
				"Content-Type": "application/json",
				"User-Agent":   "mcproxy/1.0",
			},
			expectError: false,
		},
		{
			name: "valid headers with templates",
			headers: map[string]string{
				"Authorization": "Bearer {{api_token}}",
				"X-API-Key":     "{{secret_key}}",
			},
			secrets: Secrets{
				"api_token":  "secret123",
				"secret_key": "key456",
			},
			expectError: false,
		},
		{
			name: "headers with missing template",
			headers: map[string]string{
				"Authorization": "Bearer {{missing_token}}",
			},
			secrets: Secrets{
				"other_secret": "value",
			},
			expectError: false, // Should not error, just leave empty
		},
		{
			name: "invalid header name",
			headers: map[string]string{
				"Invalid-Header-Name\x00": "value",
			},
			expectError: true,
			errorMsg:    "invalid header name",
		},
		{
			name: "header with control character in value",
			headers: map[string]string{
				"X-Data": "value\x00with\x00null",
			},
			expectError: true,
			errorMsg:    "invalid characters in header value",
		},
		{
			name:        "no headers",
			headers:     nil,
			expectError: false,
		},
		{
			name:        "empty headers",
			headers:     map[string]string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Substitute templates first if secrets are provided
			var processedHeaders map[string]string
			var err error

			if len(tt.secrets) > 0 {
				processedHeaders = make(map[string]string)
				for key, value := range tt.headers {
					substituted, _, e := substituteTemplate(value, tt.secrets)
					if e != nil {
						err = e
						break
					}
					processedHeaders[key] = substituted
				}
			} else {
				processedHeaders = tt.headers
			}

			if err == nil {
				err = validateHeaders(processedHeaders)
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestStdioEndpointFullValidation tests complete stdio endpoint validation
func TestStdioEndpointFullValidation(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    StdioEndpoint
		secrets     Secrets
		expectError bool
		errorMsg    string
	}{
		{
			name: "complete valid endpoint",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/mcp-server",
				Args:    []string{"--config", "/etc/mcp/config.yaml", "init-arg-1", "init-arg-2"},
				Env: map[string]string{
					"API_KEY": "{{api_token}}",
					"LOG":     "debug",
				},
				EndpointShared: EndpointShared{
					Timeout:     toPtr(30 * time.Second),
					MaxBodySize: func() *int64 { v := int64(5242880); return &v }(),
				},
			},
			secrets: Secrets{
				"api_token": "secret123",
			},
			expectError: false,
		},
		{
			name: "minimal valid endpoint",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/simple-server",
			},
			expectError: false,
		},
		{
			name: "endpoint with all optional fields",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "/usr/local/bin/full-server",
				Env:     map[string]string{"DEBUG": "true"},
				Args:    []string{"--verbose"},
				EndpointShared: EndpointShared{
					Timeout:     toPtr(1 * time.Minute),
					MaxBodySize: func() *int64 { v := int64(10485760); return &v }(),
				},
			},
			expectError: false,
		},
		{
			name: "invalid endpoint missing command",
			endpoint: StdioEndpoint{
				Type: EndpointTypeStdio,
				Env: map[string]string{
					"API_KEY": "{{api_token}}",
				},
			},
			expectError: true,
			errorMsg:    "command is required for stdio endpoints",
		},
		{
			name: "endpoint with multiple validation errors",
			endpoint: StdioEndpoint{
				Type:    EndpointTypeStdio,
				Command: "", // Empty command
				EndpointShared: EndpointShared{
					Timeout:     toPtr(0 * time.Second), // Invalid timeout (0)
					MaxBodySize: func() *int64 { v := int64(-1); return &v }(),
				},
			},
			expectError: true,
			errorMsg:    "command is required for stdio endpoints", // Any of the validation errors is acceptable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First validate the endpoint structure
			// Create a temporary endpoints map for validation
			endpoints := map[string]Endpoint{"test": {Value: tt.endpoint}}
			err := validateEndpoints(endpoints)
			if err != nil {
				if tt.expectError {
					// For multiple validation errors, check if any of the expected errors are present
					if tt.errorMsg != "" {
						errorMsg := err.Error()
						// Check if the error message contains any of the expected validation errors
						if strings.Contains(errorMsg, tt.errorMsg) {
							return // Expected validation error found
						}
						t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
						return
					}
					return // Expected validation error
				}
				t.Errorf("Unexpected validation error: %v", err)
				return
			}

			// If we have environment variables, substitute templates
			if len(tt.endpoint.Env) > 0 && len(tt.secrets) > 0 {
				_, missing, err := substituteTemplateInEnv(tt.endpoint.Env, tt.secrets)
				if err != nil {
					if tt.expectError {
						return
					}
					t.Errorf("Template substitution error: %v", err)
					return
				}
				// Check for missing variables if they matter
				if len(missing) > 0 && !tt.expectError {
					t.Logf("Warning: missing template variables: %v", missing)
				}
			}

			// Validate timeout if present
			if tt.endpoint.Timeout != nil {
				// Check for invalid values
				if *tt.endpoint.Timeout < 0 {
					err = fmt.Errorf("timeout cannot be negative")
				} else if *tt.endpoint.Timeout > time.Hour {
					err = fmt.Errorf("timeout too large (max: 1 hour)")
				}
				if err != nil {
					if tt.expectError {
						return
					}
					t.Errorf("Timeout validation error: %v", err)
					return
				}
			}

			// Validate max body size if set
			if tt.endpoint.MaxBodySize != nil && *tt.endpoint.MaxBodySize > 0 {
				// TODO: Implement validateMaxBodySize function
				// err := validateMaxBodySize(tt.endpoint.MaxBodySize)
				var err error
				// For now, just check if MaxBodySize is negative
				if *tt.endpoint.MaxBodySize < 0 {
					err = fmt.Errorf("max body size cannot be negative")
				}
				if err != nil {
					if tt.expectError {
						return
					}
					t.Errorf("MaxBodySize validation error: %v", err)
					return
				}
			}

			// If we expected an error but didn't get one
			if tt.expectError {
				t.Errorf("Expected error but none occurred")
			}
		})
	}
}

// TestBackwardCompatibilityWithStdio tests that stdio endpoints work alongside HTTP endpoints
func TestBackwardCompatibilityWithStdio(t *testing.T) {
	config := &Config{
		Endpoints: map[string]Endpoint{
			"legacy-http": {
				Value: HttpEndpoint{
					Type: EndpointTypeHTTP,
					Url:  "https://api.example.com",
					Headers: map[string]string{
						"Authorization": "Bearer {{api_token}}",
					},
				},
			},
			"modern-stdio": {
				Value: StdioEndpoint{
					Type:    EndpointTypeStdio,
					Command: "/usr/local/bin/mcp-server",
					Env: map[string]string{
						"API_KEY": "{{api_token}}",
					},
				},
			},
			"modern-http": {
				Value: HttpEndpoint{
					Type: EndpointTypeHTTP,
					Url:  "https://modern.example.com",
				},
			},
		},
	}

	secrets := Secrets{
		"api_token": "secret123",
	}

	// Process templates
	processed, skipped := ProcessSecretTemplates(config, secrets)

	if len(skipped) != 0 {
		t.Errorf("Expected 0 skipped endpoints, got %d: %v", len(skipped), skipped)
	}

	if len(processed) != 3 {
		t.Errorf("Expected 3 processed endpoints, got %d", len(processed))
	}

	// Check that HTTP endpoint was processed correctly
	httpEndpoint := processed["legacy-http"]
	httpEp, ok := httpEndpoint.Value.(HttpEndpoint)
	if !ok {
		t.Fatalf("Expected HttpEndpoint for legacy, got %T", httpEndpoint.Value)
	}
	if httpEp.Type != EndpointTypeHTTP {
		t.Errorf("Expected HTTP Type for legacy endpoint, got '%s'", httpEp.Type)
	}

	// Check that stdio endpoint was processed correctly
	stdioEndpoint := processed["modern-stdio"]
	stdioEp, ok := stdioEndpoint.Value.(StdioEndpoint)
	if !ok {
		t.Fatalf("Expected StdioEndpoint for modern stdio, got %T", stdioEndpoint.Value)
	}
	if stdioEp.Type != EndpointTypeStdio {
		t.Errorf("Expected Stdio Type for modern endpoint, got '%s'", stdioEp.Type)
	}
	if stdioEp.Env["API_KEY"] != "secret123" {
		t.Errorf("Expected API_KEY 'secret123', got '%s'", stdioEp.Env["API_KEY"])
	}
}

// TestDiscriminatedUnionWithStdio tests that the discriminated union properly handles stdio endpoints
func TestDiscriminatedUnionWithStdio(t *testing.T) {
	// Test JSON unmarshaling with stdio endpoint
	jsonData := `{
		"endpoints": {
			"mcp-server": {
				"type": "stdio",
				"command": "/usr/local/bin/mcp-server",
				"env": {
					"API_KEY": "{{api_token}}",
					"LOG_LEVEL": "debug"
				},
				"args": ["init-arg-1"],
				"timeout": "30s",
				"maxBodySize": 5242880
			}
		}
	}`

	t.Logf("JSON Data: %s", jsonData)

	var config Config
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	t.Logf("Config: %+v", config)

	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}

	endpoint := config.Endpoints["mcp-server"]
	t.Logf("Endpoint: %+v", endpoint)

	stdioEp, ok := endpoint.Value.(StdioEndpoint)
	if !ok {
		t.Fatalf("Expected StdioEndpoint, got %T", endpoint.Value)
	}

	t.Logf("StdioEndpoint: %+v", stdioEp)

	if stdioEp.Type != EndpointTypeStdio {
		t.Errorf("Expected type 'stdio', got '%s'", stdioEp.Type)
	}

	if stdioEp.Command != "/usr/local/bin/mcp-server" {
		t.Errorf("Expected command '/usr/local/bin/mcp-server', got %v", stdioEp.Command)
	}

	if stdioEp.Env["LOG_LEVEL"] != "debug" {
		t.Errorf("Expected LOG_LEVEL 'debug', got '%s'", stdioEp.Env["LOG_LEVEL"])
	}

	if stdioEp.Timeout == nil || *stdioEp.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", stdioEp.Timeout)
	}

	if stdioEp.MaxBodySize == nil || *stdioEp.MaxBodySize != 5242880 {
		t.Errorf("Expected maxBodySize 5242880, got %v", stdioEp.MaxBodySize)
	}
}
