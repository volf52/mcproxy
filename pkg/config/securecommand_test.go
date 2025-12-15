package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSecureCommandValidator_ValidateAndParseCommand(t *testing.T) {
	validator := NewSecureCommandValidator()

	tests := []struct {
		name        string
		command     string
		expectError bool
		expected    []string
	}{
		// Valid commands
		{
			name:     "simple command",
			command:  "ls",
			expected: []string{"ls"},
		},
		{
			name:     "command with arguments",
			command:  "ls -la /tmp",
			expected: []string{"ls", "-la", "/tmp"},
		},
		{
			name:     "command with quoted argument",
			command:  `echo "hello world"`,
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "absolute path",
			command:  "/usr/bin/ls -la",
			expected: []string{"/usr/bin/ls", "-la"},
		},
		{
			name:     "relative path",
			command:  "./myprogram arg1 arg2",
			expected: []string{"./myprogram", "arg1", "arg2"},
		},

		// Invalid commands - should error
		{
			name:        "command with semicolon",
			command:     "ls; rm -rf /",
			expectError: true,
		},
		{
			name:        "command with pipe",
			command:     "ls | grep test",
			expectError: true,
		},
		{
			name:        "command with ampersand",
			command:     "ls & rm -rf /",
			expectError: true,
		},
		{
			name:        "command with backtick",
			command:     "echo `rm -rf /`",
			expectError: true,
		},
		{
			name:        "command with redirect",
			command:     "ls > /tmp/out.txt",
			expectError: true,
		},
		{
			name:        "command with null byte",
			command:     "echo\x00hello",
			expectError: true,
		},
		{
			name:        "command with path traversal",
			command:     "../../../bin/sh",
			expectError: true,
		},
		{
			name:        "unterminated quote",
			command:     `echo "hello`,
			expectError: true,
		},
		{
			name:        "empty command",
			command:     "",
			expectError: true,
		},
		{
			name:        "command with control characters",
			command:     "ls\r\nrm -rf /",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ValidateAndParseCommand(tt.command)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d parts, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("part %d: expected %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestSecureCommandValidator_WithAllowedPaths(t *testing.T) {
	validator := NewSecureCommandValidator().WithAllowedPaths([]string{
		"/usr/bin",
		"/opt/myapp",
	})

	// Should allow paths in allowed directories
	_, err := validator.ValidateAndParseCommand("/usr/bin/ls -la")
	if err != nil {
		t.Errorf("expected allowed path to succeed: %v", err)
	}

	// Should deny paths not in allowed directories
	_, err = validator.ValidateAndParseCommand("/tmp/evil.sh")
	if err == nil {
		t.Errorf("expected disallowed path to fail")
	}

	// Should still allow simple commands (PATH lookup)
	_, err = validator.ValidateAndParseCommand("ls -la")
	if err != nil {
		t.Errorf("expected simple command to succeed: %v", err)
	}
}

func TestSecureCommandValidator_IsSafeCommand(t *testing.T) {
	validator := NewSecureCommandValidator()

	// Safe commands
	safeCommands := []string{
		"ls -la",
		"/usr/bin/python3 script.py",
		"node server.js",
		"java -jar app.jar",
		"docker run myimage",
	}

	for _, cmd := range safeCommands {
		if !validator.IsSafeCommand(cmd) {
			t.Errorf("expected command %q to be safe", cmd)
		}
	}

	// Unsafe commands
	unsafeCommands := []string{
		"ls; rm -rf /",
		"cat /etc/passwd",
		"rm -rf /",
		"chmod 777 /etc/shadow",
		"nc -l 4444 -e /bin/sh",
		"python -c 'import os; os.system(\"rm -rf /\")'",
	}

	for _, cmd := range unsafeCommands {
		if validator.IsSafeCommand(cmd) {
			t.Errorf("expected command %q to be unsafe", cmd)
		}
	}
}

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectError bool
	}{
		// Valid commands
		{
			name:    "simple command",
			command: "ls",
		},
		{
			name:    "command with arguments",
			command: "ls -la /tmp",
		},
		{
			name:    "absolute path",
			command: "/usr/bin/python3",
		},
		{
			name:    "common safe command",
			command: "node server.js",
		},

		// Invalid commands
		{
			name:        "command with semicolon",
			command:     "ls; rm -rf /",
			expectError: true,
		},
		{
			name:        "command with pipe",
			command:     "cat /etc/passwd | grep root",
			expectError: true,
		},
		{
			name:        "command with redirect",
			command:     "ls > file.txt",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(tt.command)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSecureCommandValidator_WithRealFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "securecommand_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test executable
	testExe := filepath.Join(tempDir, "testexe")
	if err := os.WriteFile(testExe, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatal(err)
	}

	validator := NewSecureCommandValidator().WithStrictMode(true).WithAllowedPaths([]string{tempDir})

	// Test with allowed path
	_, err = validator.ValidateAndParseCommand(testExe)
	if err != nil {
		t.Errorf("expected allowed executable to succeed: %v", err)
	}

	// Test with non-executable file
	nonExe := filepath.Join(tempDir, "nonexe")
	if err := os.WriteFile(nonExe, []byte("not executable"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = validator.ValidateAndParseCommand(nonExe)
	if err == nil {
		t.Errorf("expected non-executable file to fail")
	}
}

func TestSecureCommandValidator_EdgeCases(t *testing.T) {
	validator := NewSecureCommandValidator()

	// Test various edge cases
	edgeCases := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "only spaces",
			command:     "   ",
			expectError: true,
		},
		{
			name:        "only tabs",
			command:     "\t\t",
			expectError: true,
		},
		{
			name:        "mixed whitespace",
			command:     " \t \n ",
			expectError: true,
		},
		{
			name:    "command with multiple spaces",
			command: "ls    -la",
		},
		{
			name:    "command with tab characters in args",
			command: "echo hello\tworld",
		},
		{
			name:        "command with newlines",
			command:     "ls\nrm -rf /",
			expectError: true,
		},
		{
			name:    "empty quoted string",
			command: `echo ""`,
		},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validator.ValidateAndParseCommand(tc.command)
			if tc.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
