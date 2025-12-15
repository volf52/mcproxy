package config

import (
	"time"
)

// NewStdioEndpoint creates a new StdioEndpoint with sensible defaults
func NewStdioEndpoint(command string, args ...string) *StdioEndpoint {
	return &StdioEndpoint{
		Type:    EndpointTypeStdio,
		Command: command,
		Args:    args,
		Env:     make(map[string]string),
	}
}

// NewHTTPEndpoint creates a new HttpEndpoint with sensible defaults
func NewHTTPEndpoint(url string) *HttpEndpoint {
	return &HttpEndpoint{
		Type:    EndpointTypeHTTP,
		Url:     url,
		Headers: make(map[string]string),
	}
}

// WithTimeout sets the timeout for the endpoint
func (e *StdioEndpoint) WithTimeout(timeout time.Duration) *StdioEndpoint {
	e.Timeout = &timeout
	return e
}

// WithMaxBodySize sets the max body size for the endpoint
func (e *StdioEndpoint) WithMaxBodySize(size int64) *StdioEndpoint {
	e.MaxBodySize = &size
	return e
}

// WithEnv adds an environment variable
func (e *StdioEndpoint) WithEnv(key, value string) *StdioEndpoint {
	if e.Env == nil {
		e.Env = make(map[string]string)
	}
	e.Env[key] = value
	return e
}

// WithArgs sets the arguments for the endpoint
func (e *StdioEndpoint) WithArgs(args ...string) *StdioEndpoint {
	e.Args = args
	return e
}

// GetTimeout returns the configured timeout or default
func (e *StdioEndpoint) GetTimeout(defaultTimeout time.Duration) time.Duration {
	if e.Timeout != nil {
		return *e.Timeout
	}
	return defaultTimeout
}

// GetMaxBodySize returns the configured max body size or default
func (e *StdioEndpoint) GetMaxBodySize(defaultSize int64) int64 {
	if e.MaxBodySize != nil {
		return *e.MaxBodySize
	}
	return defaultSize
}

// WithTimeout sets the timeout for the HTTP endpoint
func (e *HttpEndpoint) WithTimeout(timeout time.Duration) *HttpEndpoint {
	e.Timeout = &timeout
	return e
}

// WithMaxBodySize sets the max body size for the HTTP endpoint
func (e *HttpEndpoint) WithMaxBodySize(size int64) *HttpEndpoint {
	e.MaxBodySize = &size
	return e
}

// WithHeader adds a header to the HTTP endpoint
func (e *HttpEndpoint) WithHeader(key, value string) *HttpEndpoint {
	if e.Headers == nil {
		e.Headers = make(map[string]string)
	}
	e.Headers[key] = value
	return e
}

// GetTimeout returns the configured timeout or default
func (e *HttpEndpoint) GetTimeout(defaultTimeout time.Duration) time.Duration {
	if e.Timeout != nil {
		return *e.Timeout
	}
	return defaultTimeout
}

// GetMaxBodySize returns the configured max body size or default
func (e *HttpEndpoint) GetMaxBodySize(defaultSize int64) int64 {
	if e.MaxBodySize != nil {
		return *e.MaxBodySize
	}
	return defaultSize
}
