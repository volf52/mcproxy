package config

import (
	"testing"
	"time"
)

func TestValidateEndpointsComprehensive(t *testing.T) {
	tests := []struct {
		name        string
		endpoints   map[string]Endpoint
		expectError bool
		errorMsg    string // Substring to check in error message
	}{
		{
			name: "valid endpoints",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com/webhook",
						Headers: map[string]string{
							"Authorization": "Bearer token123",
							"Content-Type":  "application/json",
						},
					},
				},
				"webhook": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "http://localhost:3000/webhook",
						Headers: map[string]string{
							"X-API-Key": "secret123",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "no endpoints",
			endpoints:   map[string]Endpoint{},
			expectError: true,
			errorMsg:    "at least one endpoint must be defined",
		},
		{
			name: "empty endpoint name",
			endpoints: map[string]Endpoint{
				"": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com",
					},
				},
			},
			expectError: true,
			errorMsg:    "endpoint name cannot be empty",
		},
		{
			name: "invalid URL - missing scheme",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "api.example.com",
					},
				},
			},
			expectError: true,
			errorMsg:    "URL must use http or https scheme",
		},
		{
			name: "invalid URL - unsupported scheme",
			endpoints: map[string]Endpoint{
				"ftp": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "ftp://example.com",
					},
				},
			},
			expectError: true,
			errorMsg:    "URL must use http or https scheme",
		},
		{
			name: "invalid URL - no host",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://",
					},
				},
			},
			expectError: true,
			errorMsg:    "URL must have a host",
		},
		{
			name: "invalid URL - with fragment",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com#fragment",
					},
				},
			},
			expectError: true,
			errorMsg:    "URL fragments are not allowed",
		},
		{
			name: "invalid URL - malformed",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "not-a-url",
					},
				},
			},
			expectError: true,
			errorMsg:    "URL must use http or https scheme, got ''",
		},
		{
			name: "invalid header key - empty",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com",
						Headers: map[string]string{
							"": "value",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "header key cannot be empty",
		},
		{
			name: "invalid header key - with colon",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com",
						Headers: map[string]string{
							"Bad:Header": "value",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "header key cannot contain colon character",
		},
		{
			name: "invalid header key - control character",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com",
						Headers: map[string]string{
							"Bad\x00Header": "value",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "header key contains invalid character",
		},
		{
			name: "invalid header value - line break",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com",
						Headers: map[string]string{
							"Header": "value\nwith\nbreaks",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "header value cannot contain line breaks",
		},
		{
			name: "invalid timeout",
			endpoints: map[string]Endpoint{
				"api": {
					Value: func() HttpEndpoint {
						d := time.Duration(-1)
						return HttpEndpoint{
							Type: EndpointTypeHTTP,
							Url:  "https://api.example.com",
							EndpointShared: EndpointShared{
								Timeout: &d,
							},
						}
					}(),
				},
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
		},
		{
			name: "invalid timeout - too large",
			endpoints: map[string]Endpoint{
				"api": {
					Value: func() HttpEndpoint {
						d := time.Duration(400) * time.Second
						return HttpEndpoint{
							Type: EndpointTypeHTTP,
							Url:  "https://api.example.com",
							EndpointShared: EndpointShared{
								Timeout: &d,
							},
						}
					}(),
				},
			},
			expectError: true,
			errorMsg:    "timeout too large (max 300s)",
		},
		{
			name: "invalid max body size",
			endpoints: map[string]Endpoint{
				"api": {
					Value: func() HttpEndpoint {
						s := int64(-1)
						return HttpEndpoint{
							Type: EndpointTypeHTTP,
							Url:  "https://api.example.com",
							EndpointShared: EndpointShared{
								MaxBodySize: &s,
							},
						}
					}(),
				},
			},
			expectError: true,
			errorMsg:    "maxBodySize must be positive",
		},
		{
			name: "invalid max body size - too large",
			endpoints: map[string]Endpoint{
				"api": {
					Value: func() HttpEndpoint {
						s := int64(200 * 1024 * 1024)
						return HttpEndpoint{
							Type: EndpointTypeHTTP,
							Url:  "https://api.example.com",
							EndpointShared: EndpointShared{
								MaxBodySize: &s,
							},
						}
					}(),
				},
			},
			expectError: true,
			errorMsg:    "maxBodySize too large (max 100MB)",
		},
		{
			name: "case-insensitive name collision",
			endpoints: map[string]Endpoint{
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com",
					},
				},
				"API": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com",
					},
				},
			},
			expectError: true,
			errorMsg:    "endpoint name 'API' conflicts with endpoint 'api' (names are case-insensitive)",
		},
		{
			name: "multiple errors",
			endpoints: map[string]Endpoint{
				"": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "ftp://example.com",
						Headers: map[string]string{
							"":        "value",
							"Bad:Key": "value",
						},
					},
				},
				"API": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "not-a-url",
					},
				},
				"api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com",
					},
				},
			},
			expectError: true,
			errorMsg:    "validation failed with",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpoints(tt.endpoints)

			if tt.expectError {
				if err == nil {
					t.Errorf("validateEndpoints() expected error but got none")
					return
				}

				if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("validateEndpoints() error = %v, expected to contain %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateEndpoints() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid HTTP URL",
			url:         "http://example.com",
			expectError: false,
		},
		{
			name:        "valid HTTPS URL",
			url:         "https://api.example.com/webhook",
			expectError: false,
		},
		{
			name:        "valid HTTPS URL with port",
			url:         "https://localhost:8443/api",
			expectError: false,
		},
		{
			name:        "empty URL",
			url:         "",
			expectError: true,
			errorMsg:    "URL cannot be empty",
		},
		{
			name:        "no scheme",
			url:         "api.example.com",
			expectError: true,
			errorMsg:    "URL must use http or https scheme",
		},
		{
			name:        "unsupported scheme",
			url:         "ftp://example.com",
			expectError: true,
			errorMsg:    "URL must use http or https scheme",
		},
		{
			name:        "no host",
			url:         "https://",
			expectError: true,
			errorMsg:    "URL must have a host",
		},
		{
			name:        "with fragment",
			url:         "https://api.example.com#section",
			expectError: true,
			errorMsg:    "URL fragments are not allowed",
		},
		{
			name:        "malformed URL",
			url:         "not a url at all",
			expectError: true,
			errorMsg:    "URL must use http or https scheme, got ''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValidURL(tt.url)

			if tt.expectError {
				if err == nil {
					t.Errorf("isValidURL() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("isValidURL() error = %v, expected to contain %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("isValidURL() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsValidHeaderKey(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid header key",
			key:         "Authorization",
			expectError: false,
		},
		{
			name:        "valid header key with hyphen",
			key:         "X-API-Key",
			expectError: false,
		},
		{
			name:        "empty key",
			key:         "",
			expectError: true,
			errorMsg:    "header key cannot be empty",
		},
		{
			name:        "key with colon",
			key:         "Bad:Header",
			expectError: true,
			errorMsg:    "header key cannot contain colon character",
		},
		{
			name:        "key with space",
			key:         "Bad Header",
			expectError: true,
			errorMsg:    "header key contains invalid character",
		},
		{
			name:        "key with control character",
			key:         "Bad\x00Header",
			expectError: true,
			errorMsg:    "header key contains invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValidHeaderKey(tt.key)

			if tt.expectError {
				if err == nil {
					t.Errorf("isValidHeaderKey() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("isValidHeaderKey() error = %v, expected to contain %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("isValidHeaderKey() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsValidHeaderValue(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid header value",
			value:       "Bearer token123",
			expectError: false,
		},
		{
			name:        "valid header value with special characters",
			value:       "application/json; charset=utf-8",
			expectError: false,
		},
		{
			name:        "valid header value with tab",
			value:       "value\twith\ttab",
			expectError: false,
		},
		{
			name:        "value with newline",
			value:       "value\nwith\nnewline",
			expectError: true,
			errorMsg:    "header value cannot contain line breaks",
		},
		{
			name:        "value with carriage return",
			value:       "value\rwith\rcarriage",
			expectError: true,
			errorMsg:    "header value cannot contain line breaks",
		},
		{
			name:        "value with control character",
			value:       "value\x00with\x00control",
			expectError: true,
			errorMsg:    "header value contains invalid control character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValidHeaderValue(tt.value)

			if tt.expectError {
				if err == nil {
					t.Errorf("isValidHeaderValue() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("isValidHeaderValue() error = %v, expected to contain %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("isValidHeaderValue() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateStdioEndpoints(t *testing.T) {
	tests := []struct {
		name        string
		endpoints   map[string]Endpoint
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid stdio endpoint",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "/usr/local/bin/mcp-server",
						Env: map[string]string{
							"API_KEY":   "secret123",
							"LOG_LEVEL": "debug",
						},
						Args: []string{"--init"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid stdio endpoint without env and args",
			endpoints: map[string]Endpoint{
				"simple-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "/usr/bin/simple-server",
					},
				},
			},
			expectError: false,
		},
		{
			name: "stdio endpoint missing command",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type: EndpointTypeStdio,
						Env: map[string]string{
							"API_KEY": "secret123",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "command is required for stdio endpoints",
		},
		{
			name: "stdio endpoint with empty command array",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "",
					},
				},
			},
			expectError: true,
			errorMsg:    "command is required for stdio endpoints",
		},
		{
			name: "stdio endpoint with empty command component",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "",
					},
				},
			},
			expectError: true,
			errorMsg:    "command is required for stdio endpoints",
		},
		{
			name: "stdio endpoint with invalid characters in command",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "/usr/local/bin/mcp-server\n",
					},
				},
			},
			expectError: true,
			errorMsg:    "command contains control characters",
		},
		{
			name: "invalid endpoint type",
			endpoints: map[string]Endpoint{
				"invalid": {
					Value: StdioEndpoint{
						Type:    EndpointType("invalid"),
						Command: "/usr/bin/invalid",
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid endpoint type 'invalid', must be 'http' or 'stdio'",
		},
		{
			name: "stdio endpoint with invalid env key",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "/usr/local/bin/mcp-server",
						Env: map[string]string{
							"": "value",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "environment variable key cannot be empty",
		},
		{
			name: "stdio endpoint with invalid characters in env key",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "/usr/local/bin/mcp-server",
						Env: map[string]string{
							"KEY\x00": "value",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "environment variable key contains invalid characters",
		},
		{
			name: "stdio endpoint with invalid characters in env value",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "/usr/local/bin/mcp-server",
						Env: map[string]string{
							"KEY": "value\x00",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "environment variable value contains invalid characters",
		},
		{
			name: "stdio endpoint with empty arg",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "/usr/local/bin/mcp-server",
						Args:    []string{"--init", ""},
					},
				},
			},
			expectError: true,
			errorMsg:    "arg cannot be empty",
		},
		{
			name: "mixed HTTP and stdio endpoints",
			endpoints: map[string]Endpoint{
				"http-api": {
					Value: HttpEndpoint{
						Type: EndpointTypeHTTP,
						Url:  "https://api.example.com",
						Headers: map[string]string{
							"Authorization": "Bearer token123",
						},
					},
				},
				"stdio-mcp": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "/usr/local/bin/mcp-server",
						Env: map[string]string{
							"API_KEY": "secret456",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "stdio endpoint with tab in command",
			endpoints: map[string]Endpoint{
				"mcp-server": {
					Value: StdioEndpoint{
						Type:    EndpointTypeStdio,
						Command: "/usr/local/bin/mcp-server\t",
					},
				},
			},
			expectError: true,
			errorMsg:    "command contains invalid trailing whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpoints(tt.endpoints)

			if tt.expectError {
				if err == nil {
					t.Errorf("validateEndpoints() expected error but got none")
					return
				}

				if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("validateEndpoints() error = %v, expected to contain %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateEndpoints() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	t.Run("single error", func(t *testing.T) {
		var errors validationErrors
		errors.add("field1", "error message 1")

		errStr := errors.Error()
		if errStr != "error message 1" {
			t.Errorf("Expected single error message, got: %v", errStr)
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		var errors validationErrors
		errors.add("field1", "error message 1")
		errors.add("field2", "error message 2")
		errors.add("field3", "error message 3")

		errStr := errors.Error()
		expected := "validation failed with 3 errors:\n  error message 1\n  error message 2\n  error message 3"
		if errStr != expected {
			t.Errorf("Expected multiple error message, got:\n%v", errStr)
		}
	})

	t.Run("empty errors", func(t *testing.T) {
		var errors validationErrors
		errStr := errors.Error()
		if errStr != "" {
			t.Errorf("Expected empty error string, got: %v", errStr)
		}
	})
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstringMiddle(s, substr)))
}

func containsSubstringMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
