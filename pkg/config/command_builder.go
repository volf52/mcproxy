package config

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// CommandBuilder provides a secure way to build commands from configuration
type CommandBuilder struct {
	allowedPaths []string
}

// NewCommandBuilder creates a new CommandBuilder with default security settings
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{
		allowedPaths: []string{
			"/usr/bin",
			"/usr/local/bin",
			"/bin",
			"/sbin",
			"/usr/sbin",
			"/opt",
			"/usr/local/opt",
		},
	}
}

// WithAllowedPaths adds additional allowed paths for command execution
func (cb *CommandBuilder) WithAllowedPaths(paths []string) *CommandBuilder {
	cb.allowedPaths = append(cb.allowedPaths, paths...)
	return cb
}

// BuildCommand securely parses and builds a command from config
func (cb *CommandBuilder) BuildCommand(commandStr string, args []string) (*exec.Cmd, error) {
	if commandStr == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Parse the command string to handle quoted arguments and spaces
	parts, err := cb.parseCommandString(commandStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %w", err)
	}

	// Validate the command path
	if err := cb.validateCommandPath(parts[0]); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Combine parsed command with additional args
	allArgs := append(parts[1:], args...)

	// Create the command
	cmd := exec.Command(parts[0], allArgs...)

	// Set sysproc attributes for better process management
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}

	return cmd, nil
}

// parseCommandString parses a command string handling quotes properly
func (cb *CommandBuilder) parseCommandString(commandStr string) ([]string, error) {
	// Use shlex-style parsing to handle quoted arguments
	// This is a simplified implementation - consider using a proper shlex library
	var parts []string
	var current strings.Builder
	var inQuotes bool
	var quoteChar rune

	for _, r := range commandStr {
		switch {
		case r == '"' || r == '\'':
			if !inQuotes {
				inQuotes = true
				quoteChar = r
			} else if r == quoteChar {
				inQuotes = false
			} else {
				current.WriteRune(r)
			}
		case r == ' ' && !inQuotes:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	if inQuotes {
		return nil, fmt.Errorf("unclosed quote in command string")
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	return parts, nil
}

// validateCommandPath ensures the command is in an allowed path
func (cb *CommandBuilder) validateCommandPath(command string) error {
	// Allow absolute paths
	if strings.HasPrefix(command, "/") {
		for _, allowedPath := range cb.allowedPaths {
			if strings.HasPrefix(command, allowedPath+"/") || command == allowedPath {
				return nil
			}
		}
		return fmt.Errorf("command path not in allowed paths: %s", command)
	}

	// For relative commands, check if they exist in PATH
	_, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("command not found in PATH: %s", command)
	}

	return nil
}

// ProcessConfig represents process configuration for execution
type ProcessConfig struct {
	Command    string
	Args       []string
	Env        map[string]string
	Timeout    *time.Duration
	WorkingDir string
}

// Validate ensures the process configuration is valid
func (pc *ProcessConfig) Validate() error {
	if pc.Command == "" {
		return fmt.Errorf("command is required")
	}

	for i, arg := range pc.Args {
		if strings.Contains(arg, "\x00") {
			return fmt.Errorf("argument %d contains null byte", i)
		}
	}

	return nil
}

// BuildCommand builds an exec.Cmd from the process configuration
func (pc *ProcessConfig) BuildCommand(builder *CommandBuilder) (*exec.Cmd, error) {
	if err := pc.Validate(); err != nil {
		return nil, err
	}

	cmd, err := builder.BuildCommand(pc.Command, pc.Args)
	if err != nil {
		return nil, err
	}

	// Set environment variables
	if pc.Env != nil {
		cmd.Env = []string{}
		for k, v := range pc.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Set working directory
	if pc.WorkingDir != "" {
		cmd.Dir = pc.WorkingDir
	}

	return cmd, nil
}

// CreateContextWithTimeout creates a context with timeout if configured
func (pc *ProcessConfig) CreateContextWithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if pc.Timeout != nil {
		return context.WithTimeout(parent, *pc.Timeout)
	}
	return context.WithCancel(parent)
}
