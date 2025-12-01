package config

import (
	"testing"
)

func TestNormalizePort(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "empty string returns default",
			input:       "",
			expected:    ":8099",
			expectError: false,
		},
		{
			name:        "port with colon valid",
			input:       ":8080",
			expected:    ":8080",
			expectError: false,
		},
		{
			name:        "port without colon valid",
			input:       "8080",
			expected:    ":8080",
			expectError: false,
		},
		{
			name:        "boundary port 1",
			input:       "1",
			expected:    ":1",
			expectError: false,
		},
		{
			name:        "boundary port 65535",
			input:       "65535",
			expected:    ":65535",
			expectError: false,
		},
		{
			name:        "boundary port 1 with colon",
			input:       ":1",
			expected:    ":1",
			expectError: false,
		},
		{
			name:        "boundary port 65535 with colon",
			input:       ":65535",
			expected:    ":65535",
			expectError: false,
		},
		{
			name:        "port too low",
			input:       "0",
			expected:    "",
			expectError: true,
		},
		{
			name:        "port too high",
			input:       "65536",
			expected:    "",
			expectError: true,
		},
		{
			name:        "port too low with colon",
			input:       ":0",
			expected:    "",
			expectError: true,
		},
		{
			name:        "port too high with colon",
			input:       ":65536",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid text",
			input:       "invalid",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid text with colon",
			input:       ":invalid",
			expected:    "",
			expectError: true,
		},
		{
			name:        "negative port",
			input:       "-8080",
			expected:    "",
			expectError: true,
		},
		{
			name:        "negative port with colon",
			input:       ":-8080",
			expected:    "",
			expectError: true,
		},
		{
			name:        "decimal port",
			input:       "8080.5",
			expected:    "",
			expectError: true,
		},
		{
			name:        "decimal port with colon",
			input:       ":8080.5",
			expected:    "",
			expectError: true,
		},
		{
			name:        "port with leading zeros",
			input:       "0080",
			expected:    ":80",
			expectError: false,
		},
		{
			name:        "port with leading zeros and colon",
			input:       ":0080",
			expected:    ":0080",
			expectError: false,
		},
		{
			name:        "common port 80",
			input:       "80",
			expected:    ":80",
			expectError: false,
		},
		{
			name:        "common port 443",
			input:       "443",
			expected:    ":443",
			expectError: false,
		},
		{
			name:        "common port 8080",
			input:       "8080",
			expected:    ":8080",
			expectError: false,
		},
		{
			name:        "common port 8080 with colon",
			input:       ":8080",
			expected:    ":8080",
			expectError: false,
		},
		{
			name:        "whitespace only",
			input:       "   ",
			expected:    "",
			expectError: true,
		},
		{
			name:        "whitespace around port",
			input:       " 8080 ",
			expected:    "",
			expectError: true,
		},
		{
			name:        "port with text mixed",
			input:       "8080abc",
			expected:    "",
			expectError: true,
		},
		{
			name:        "port with text mixed and colon",
			input:       ":8080abc",
			expected:    "",
			expectError: true,
		},
		{
			name:        "port with multiple colons",
			input:       ":8080:9090",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizePort(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("NormalizePort(%q) expected error but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("NormalizePort(%q) unexpected error: %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("NormalizePort(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidatePortNumber(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "valid port 1",
			input:       "1",
			expectError: false,
		},
		{
			name:        "valid port 65535",
			input:       "65535",
			expectError: false,
		},
		{
			name:        "valid port 8080",
			input:       "8080",
			expectError: false,
		},
		{
			name:        "port too low",
			input:       "0",
			expectError: true,
		},
		{
			name:        "port too high",
			input:       "65536",
			expectError: true,
		},
		{
			name:        "invalid text",
			input:       "invalid",
			expectError: true,
		},
		{
			name:        "negative port",
			input:       "-8080",
			expectError: true,
		},
		{
			name:        "decimal port",
			input:       "8080.5",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "port with leading zeros",
			input:       "0080",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePortNumber(tt.input)

			if tt.expectError && err == nil {
				t.Errorf("validatePortNumber(%q) expected error but got none", tt.input)
			}

			if !tt.expectError && err != nil {
				t.Errorf("validatePortNumber(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}