package endpoints

import (
	"context"
	"net/http"
)

// ServeHTTPAdapter adapts an Endpoint to http.Handler interface
type ServeHTTPAdapter struct {
	endpoint Endpoint
}

// NewServeHTTPAdapter creates an http.Handler from an Endpoint
func NewServeHTTPAdapter(endpoint Endpoint) http.Handler {
	return &ServeHTTPAdapter{endpoint: endpoint}
}

// ServeHTTP implements http.Handler
func (a *ServeHTTPAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle request
	resp, err := a.endpoint.HandleRequest(r.Context(), r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Write body
	if len(resp.Body) > 0 {
		w.Write(resp.Body)
	}
}

// GetPID is a helper function to safely get PID from an endpoint
func GetPID(endpoint Endpoint) int {
	if endpointWithPID, ok := endpoint.(EndpointWithPID); ok {
		return endpointWithPID.GetPID()
	}
	return 0
}

// EndpointWithInitialize is an optional interface for endpoints that need explicit initialization
type EndpointWithInitialize interface {
	Endpoint
	Initialize() error
}

// Initialize is a helper function to safely initialize an endpoint if needed
func Initialize(endpoint Endpoint) error {
	if endpointWithInit, ok := endpoint.(EndpointWithInitialize); ok {
		return endpointWithInit.Initialize()
	}
	return nil
}

// EndpointWithHealthCheck is an optional interface for endpoints that support health checking
type EndpointWithHealthCheck interface {
	Endpoint
	HealthCheck(ctx context.Context) error
}

// HealthCheck is a helper function to safely health check an endpoint
func HealthCheck(ctx context.Context, endpoint Endpoint) error {
	if endpointWithHealth, ok := endpoint.(EndpointWithHealthCheck); ok {
		return endpointWithHealth.HealthCheck(ctx)
	}
	return nil
}
