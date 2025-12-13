package endpoints

import (
	"context"
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

// Endpoint represents a proxy endpoint that can handle HTTP requests
type Endpoint interface {
	// HandleRequest handles an HTTP request and returns a response
	HandleRequest(ctx context.Context, r *http.Request) (*Response, error)

	// Close gracefully closes the endpoint and releases resources
	Close() error
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
