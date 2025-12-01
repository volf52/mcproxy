package logging

import (
	"os"
	"testing"
)

func TestSecureLoggingInit(t *testing.T) {
	// Test initialization without debug mode
	oldDebug := os.Getenv("MCPROXY_DEBUG")
	defer os.Setenv("MCPROXY_DEBUG", oldDebug)

	os.Setenv("MCPROXY_DEBUG", "false")
	secrets := map[string]string{
		"api_key": "secret123",
		"token":   "abc123xyz",
	}
	Init(secrets)

	if IsDebugMode() {
		t.Error("Expected debug mode to be disabled")
	}

	// Test initialization with debug mode
	os.Setenv("MCPROXY_DEBUG", "true")
	Init(secrets)

	if !IsDebugMode() {
		t.Error("Expected debug mode to be enabled")
	}
}

func TestSanitizeForLogging(t *testing.T) {
	// Setup with non-debug mode
	oldDebug := os.Getenv("MCPROXY_DEBUG")
	oldLevel := os.Getenv("MCPROXY_DEBUG_LEVEL")
	defer os.Setenv("MCPROXY_DEBUG", oldDebug)
	defer os.Setenv("MCPROXY_DEBUG_LEVEL", oldLevel)

	os.Setenv("MCPROXY_DEBUG", "false")
	os.Setenv("MCPROXY_DEBUG_LEVEL", "sanitized")
	secrets := map[string]string{
		"api_key": "supersecret123",
		"token":   "mytoken456",
	}
	Init(secrets)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "secret template",
			input:    "Bearer {{api_key}}",
			expected: "Bearer [SECRET]",
		},
		{
			name:     "multiple secret templates",
			input:    "Token: {{api_key}}, Auth: {{token}}",
			expected: "Token: [SECRET], Auth: [SECRET]",
		},
		{
			name:     "mixed template and text",
			input:    "Bearer {{api_key}}-version1",
			expected: "Bearer [SECRET]-version1",
		},
		{
			name:     "no templates",
			input:    "static-value",
			expected: "static-value",
		},
		{
			name:     "actual secret value",
			input:    "Authorization: supersecret123",
			expected: "Authorization: [SECRET]",
		},
		{
			name:     "short value should not be replaced",
			input:    "value: abc",
			expected: "value: abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLogging(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSanitizeForLoggingDebugMode(t *testing.T) {
	// Setup with debug mode
	oldDebug := os.Getenv("MCPROXY_DEBUG")
	oldLevel := os.Getenv("MCPROXY_DEBUG_LEVEL")
	defer os.Setenv("MCPROXY_DEBUG", oldDebug)
	defer os.Setenv("MCPROXY_DEBUG_LEVEL", oldLevel)

	os.Setenv("MCPROXY_DEBUG", "true")
	os.Setenv("MCPROXY_DEBUG_LEVEL", "unredacted")
	secrets := map[string]string{
		"api_key": "supersecret123",
	}
	Init(secrets)

	input := "Bearer {{api_key}} and supersecret123"
	result := SanitizeForLogging(input)

	// In debug mode, should return the original text
	if result != input {
		t.Errorf("Expected '%s' in debug mode, got '%s'", input, result)
	}
}

func TestSanitizeHeadersForLogging(t *testing.T) {
	// Setup with non-debug mode
	oldDebug := os.Getenv("MCPROXY_DEBUG")
	oldLevel := os.Getenv("MCPROXY_DEBUG_LEVEL")
	defer os.Setenv("MCPROXY_DEBUG", oldDebug)
	defer os.Setenv("MCPROXY_DEBUG_LEVEL", oldLevel)

	os.Setenv("MCPROXY_DEBUG", "false")
	os.Setenv("MCPROXY_DEBUG_LEVEL", "sanitized")
	secrets := map[string]string{
		"api_key": "supersecret123",
	}
	Init(secrets)

	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "secret template in header",
			input: map[string]string{
				"Authorization": "Bearer {{api_key}}",
			},
			expected: map[string]string{
				"Authorization": "[SECRET]",
			},
		},
		{
			name: "looks like secret value",
			input: map[string]string{
				"X-API-Key": "verylongsecretkey12345",
			},
			expected: map[string]string{
				"X-API-Key": "[SECRET]",
			},
		},
		{
			name: "normal header",
			input: map[string]string{
				"Content-Type": "application/json",
			},
			expected: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name: "mixed headers",
			input: map[string]string{
				"Authorization":  "Bearer {{api_key}}",
				"Content-Type":   "application/json",
				"X-Custom-Token": "shorttoken",
			},
			expected: map[string]string{
				"Authorization":  "[SECRET]",
				"Content-Type":   "application/json",
				"X-Custom-Token": "shorttoken",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeHeadersForLogging(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d headers, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("Header '%s': expected '%s', got '%s'", key, expectedValue, result[key])
				}
			}
		})
	}
}

func TestLooksLikeSecretValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "short value",
			input:    "abc",
			expected: false,
		},
		{
			name:     "medium length no separators",
			input:    "mediumlength",
			expected: false,
		},
		{
			name:     "long value with dash",
			input:    "very-long-secret-key-value-123",
			expected: true,
		},
		{
			name:     "long value with underscore",
			input:    "very_long_secret_key_value_123",
			expected: true,
		},
		{
			name:     "long value with dots",
			input:    "api.key.that.is.very.long.12345",
			expected: true,
		},
		{
			name:     "long token-like value",
			input:    "abcdefghijklmnopqrstuvwxyz123456",
			expected: true,
		},
		{
			name:     "value with spaces",
			input:    "this contains spaces so not a secret",
			expected: false,
		},
		{
			name:     "exact threshold length",
			input:    "exactly16charslong",
			expected: true,
		},
		{
			name:     "just under threshold",
			input:    "only15charlong",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeSecretValue(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v for '%s', got %v", tt.expected, tt.input, result)
			}
		})
	}
}

func TestTemplatePatternMatching(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "simple template",
			input:    "{{api_key}}",
			expected: true,
		},
		{
			name:     "template with spaces",
			input:    "{{ api_key }}",
			expected: true,
		},
		{
			name:     "multiple templates",
			input:    "{{api_key}} and {{token}}",
			expected: true,
		},
		{
			name:     "no template",
			input:    "static value",
			expected: false,
		},
		{
			name:     "partial template",
			input:    "{api_key}",
			expected: false,
		},
		{
			name:     "unclosed template",
			input:    "{{api_key",
			expected: false,
		},
		{
			name:     "empty template",
			input:    "{{}}",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := templatePattern.MatchString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v for '%s', got %v", tt.expected, tt.input, result)
			}
		})
	}
}

func TestLoggingFunctions(t *testing.T) {
	// These tests just verify the functions don't panic
	// We can't easily capture log output in a unit test without additional setup

	// Setup with non-debug mode
	oldDebug := os.Getenv("MCPROXY_DEBUG")
	defer os.Setenv("MCPROXY_DEBUG", oldDebug)

	os.Setenv("MCPROXY_DEBUG", "false")
	secrets := map[string]string{
		"api_key": "secret123",
	}
	Init(secrets)

	// Test that logging functions don't panic
	t.Run("Printf", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Printf panicked: %v", r)
			}
		}()
		Printf("Test message with {{api_key}}")
	})

	t.Run("Println", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Println panicked: %v", r)
			}
		}()
		Println("Test message with", "secret123")
	})

	t.Run("Info", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Info panicked: %v", r)
			}
		}()
		Info("Test info message")
	})

	t.Run("Warning", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Warning panicked: %v", r)
			}
		}()
		Warning("Test warning message")
	})

	t.Run("Error", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error panicked: %v", r)
			}
		}()
		Error("Test error message")
	})

	t.Run("Debug (should not output in non-debug mode)", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Debug panicked: %v", r)
			}
		}()
		Debug("This should not appear in logs")
	})
}

func TestDebugModeLogging(t *testing.T) {
	// Setup with debug mode
	oldDebug := os.Getenv("MCPROXY_DEBUG")
	defer os.Setenv("MCPROXY_DEBUG", oldDebug)

	os.Setenv("MCPROXY_DEBUG", "true")
	secrets := map[string]string{
		"api_key": "secret123",
	}
	Init(secrets)

	if !IsDebugMode() {
		t.Error("Expected debug mode to be enabled")
	}

	// Test that debug functions don't panic in debug mode
	t.Run("Debugf", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Debugf panicked: %v", r)
			}
		}()
		Debugf("Debug message: %s", "test")
	})

	t.Run("Debug", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Debug panicked: %v", r)
			}
		}()
		Debug("Debug message")
	})
}

func TestLogEndpointRegistration(t *testing.T) {
	// Setup with non-debug mode
	oldDebug := os.Getenv("MCPROXY_DEBUG")
	defer os.Setenv("MCPROXY_DEBUG", oldDebug)

	os.Setenv("MCPROXY_DEBUG", "false")
	secrets := map[string]string{
		"api_key": "secret123",
	}
	Init(secrets)

	t.Run("normal mode", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("LogEndpointRegistration panicked: %v", r)
			}
		}()
		headers := map[string]string{
			"Authorization": "Bearer {{api_key}}",
			"Content-Type":  "application/json",
		}
		LogEndpointRegistration("test-endpoint", "https://example.com", "/mcp", headers)
	})

	// Test in debug mode
	os.Setenv("MCPROXY_DEBUG", "true")
	Init(secrets)

	t.Run("debug mode", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("LogEndpointRegistration panicked in debug mode: %v", r)
			}
		}()
		headers := map[string]string{
			"Authorization": "Bearer {{api_key}}",
			"Content-Type":  "application/json",
		}
		LogEndpointRegistration("test-endpoint", "https://example.com", "/mcp", headers)
	})
}

func TestLogTemplateResolution(t *testing.T) {
	// Setup with non-debug mode
	oldDebug := os.Getenv("MCPROXY_DEBUG")
	defer os.Setenv("MCPROXY_DEBUG", oldDebug)

	os.Setenv("MCPROXY_DEBUG", "false")
	secrets := map[string]string{
		"api_key": "secret123",
	}
	Init(secrets)

	t.Run("normal mode", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("LogTemplateResolution panicked: %v", r)
			}
		}()
		original := map[string]string{
			"Authorization": "Bearer {{api_key}}",
		}
		resolved := map[string]string{
			"Authorization": "Bearer secret123",
		}
		missing := []string{}
		LogTemplateResolution("test-endpoint", original, resolved, missing)
	})

	// Test in debug mode
	os.Setenv("MCPROXY_DEBUG", "true")
	Init(secrets)

	t.Run("debug mode", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("LogTemplateResolution panicked in debug mode: %v", r)
			}
		}()
		original := map[string]string{
			"Authorization": "Bearer {{api_key}}",
		}
		resolved := map[string]string{
			"Authorization": "Bearer secret123",
		}
		missing := []string{}
		LogTemplateResolution("test-endpoint", original, resolved, missing)
	})
}
