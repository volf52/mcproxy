package endpoints

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mcproxy/pkg/config"
)

// Registry manages a collection of endpoints
type Registry struct {
	mu        sync.RWMutex
	endpoints map[string]Endpoint
	factory   Factory
}

// NewRegistry creates a new endpoint registry
func NewRegistry(factory Factory) *Registry {
	return &Registry{
		endpoints: make(map[string]Endpoint),
		factory:   factory,
	}
}

// Register registers an endpoint with the registry
func (r *Registry) Register(name string, cfg config.Endpoint, secrets map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.endpoints[name]; exists {
		return fmt.Errorf("endpoint '%s' already registered", name)
	}

	endpoint, err := r.factory.CreateEndpoint(name, cfg, secrets)
	if err != nil {
		return fmt.Errorf("failed to create endpoint '%s': %w", name, err)
	}

	// Initialize if the endpoint requires it
	if err := Initialize(endpoint); err != nil {
		endpoint.Close()
		return fmt.Errorf("failed to initialize endpoint '%s': %w", name, err)
	}

	r.endpoints[name] = endpoint
	return nil
}

// Get retrieves an endpoint by name
func (r *Registry) Get(name string) (Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endpoint, exists := r.endpoints[name]
	if !exists {
		return nil, fmt.Errorf("endpoint '%s' not found", name)
	}

	return endpoint, nil
}

// List returns all registered endpoint names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.endpoints))
	for name := range r.endpoints {
		names = append(names, name)
	}
	return names
}

// HealthCheck performs health checks on all endpoints
func (r *Registry) HealthCheck(ctx context.Context) map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]error)
	for name, endpoint := range r.endpoints {
		results[name] = HealthCheck(ctx, endpoint)
	}
	return results
}

// Close gracefully closes all endpoints
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errors []string
	for name, endpoint := range r.endpoints {
		if err := endpoint.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("endpoint '%s': %v", name, err))
		}
	}
	r.endpoints = make(map[string]Endpoint)

	if len(errors) > 0 {
		return fmt.Errorf("errors closing endpoints: %v", errors)
	}
	return nil
}

// EndpointInfo provides information about an endpoint
type EndpointInfo struct {
	Name      string
	Type      string
	IsHealthy bool
	PID       int
	LastCheck time.Time
}

// GetInfo returns information about all endpoints
func (r *Registry) GetInfo() []EndpointInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info := make([]EndpointInfo, 0, len(r.endpoints))
	for name, endpoint := range r.endpoints {
		ei := EndpointInfo{
			Name:      name,
			Type:      fmt.Sprintf("%T", endpoint),
			IsHealthy: true, // Default to healthy
			PID:       GetPID(endpoint),
			LastCheck: time.Now(),
		}

		// Determine type more specifically
		switch endpoint.(type) {
		case *StdioEndpoint:
			ei.Type = "stdio"
		case *HTTPEndpoint:
			ei.Type = "http"
		}

		info = append(info, ei)
	}
	return info
}

// Restart restarts an endpoint
func (r *Registry) Restart(name string, cfg config.Endpoint, secrets map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close existing endpoint
	if endpoint, exists := r.endpoints[name]; exists {
		if err := endpoint.Close(); err != nil {
			return fmt.Errorf("failed to close endpoint '%s': %w", name, err)
		}
		delete(r.endpoints, name)
	}

	// Create and register new endpoint
	endpoint, err := r.factory.CreateEndpoint(name, cfg, secrets)
	if err != nil {
		return fmt.Errorf("failed to create endpoint '%s': %w", name, err)
	}

	if err := Initialize(endpoint); err != nil {
		endpoint.Close()
		return fmt.Errorf("failed to initialize endpoint '%s': %w", name, err)
	}

	r.endpoints[name] = endpoint
	return nil
}
