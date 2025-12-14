package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"mcproxy/pkg/config"
)

// Response represents a proxy response
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// ErrorResponse represents a structured error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

// NewErrorResponse creates a JSON error response with appropriate status code
func NewErrorResponse(statusCode int, message string, code ...int) *Response {
	errorResp := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	}

	if len(code) > 0 {
		errorResp.Code = code[0]
	}

	body, err := json.Marshal(errorResp)
	if err != nil {
		// Fallback to plain text if JSON marshaling fails
		return &Response{
			StatusCode: statusCode,
			Header:     http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Body:       []byte(message),
		}
	}

	return &Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       body,
	}
}

// Endpoint represents a proxy endpoint that can handle HTTP requests
type Endpoint interface {
	// HandleRequest handles an HTTP request and returns a response
	HandleRequest(ctx context.Context, r *http.Request) (*Response, error)

	// Close gracefully closes the endpoint and releases resources
	Close() error
}

// EndpointWithPID is an optional interface for endpoints that have a process PID
type EndpointWithPID interface {
	Endpoint
	GetPID() int
}

// Factory creates an endpoint based on configuration
type Factory interface {
	// CreateEndpoint creates an endpoint from configuration
	CreateEndpoint(name string, cfg config.Endpoint, secrets map[string]string) (Endpoint, error)
}

// DefaultFactory implements the Factory interface
type DefaultFactory struct{}

// NewDefaultFactory creates a new default factory
func NewDefaultFactory() *DefaultFactory {
	return &DefaultFactory{}
}

// CreateEndpoint creates an endpoint based on the configuration type
func (f *DefaultFactory) CreateEndpoint(name string, cfg config.Endpoint, secrets map[string]string) (Endpoint, error) {
	// Check if endpoint value is nil
	if cfg.Value == nil {
		return nil, fmt.Errorf("endpoint value cannot be nil")
	}

	// Use type assertion to determine endpoint type
	switch cfg.Value.(type) {
	case config.HttpEndpoint:
		return NewHTTPEndpoint(name, cfg, secrets)
	case config.StdioEndpoint:
		return NewStdioEndpoint(name, cfg, secrets)
	default:
		return nil, fmt.Errorf("unsupported endpoint type: %T", cfg.Value)
	}
}
