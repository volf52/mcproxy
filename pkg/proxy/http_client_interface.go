package proxy

import (
	"net/http"
)

// HTTPClient interface defines the methods we need from HTTP client
// This allows us to create mock implementations for testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HTTPClientFactory creates HTTPClient instances
// This allows us to inject different implementations
type HTTPClientFactory interface {
	CreateClient() HTTPClient
}

// DefaultHTTPClientFactory creates real HTTP clients
type DefaultHTTPClientFactory struct{}

// CreateClient returns a real HTTP client
func (f *DefaultHTTPClientFactory) CreateClient() HTTPClient {
	return &httpClientWrapper{client: CreateHTTPClient()}
}

// httpClientWrapper wraps *http.Client to implement HTTPClient interface
type httpClientWrapper struct {
	client *http.Client
}

// Do implements HTTPClient interface
func (w *httpClientWrapper) Do(req *http.Request) (*http.Response, error) {
	return w.client.Do(req)
}

// Ensure DefaultHTTPClientFactory implements HTTPClientFactory
var _ HTTPClientFactory = (*DefaultHTTPClientFactory)(nil)

// Ensure httpClientWrapper implements HTTPClient
var _ HTTPClient = (*httpClientWrapper)(nil)
