package mcp

import (
	"context"
	"testing"
	"time"
)

// TestNewMCPClient tests creating a new MCP client
func TestNewMCPClient(t *testing.T) {
	// Use cat process which stays alive and echoes input
	opts := &ProcessManagerOptions{
		Command: []string{"cat"},
	}

	pm, err := NewProcessManager(opts)
	if err != nil {
		t.Fatalf("Failed to create process manager: %v", err)
	}
	defer pm.Stop()

	client := NewMCPClient(pm)
	if client == nil {
		t.Error("Expected non-nil client")
	}

	// Check initial state
	if client.IsHealthy() {
		t.Error("Client should not be healthy initially")
	}
}

// TestClientInitializationWithoutParams tests client initialization without parameters
func TestClientInitializationWithoutParams(t *testing.T) {
	// Use cat process which stays alive
	opts := &ProcessManagerOptions{
		Command: []string{"cat"},
	}

	pm, err := NewProcessManager(opts)
	if err != nil {
		t.Fatalf("Failed to create process manager: %v", err)
	}

	// Start the process with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := pm.Start(ctx); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	defer pm.Stop()

	client := NewMCPClient(pm)
	defer client.Close()

	// Test initialization without params (should fail gracefully)
	_, err = client.Initialize(nil)
	if err == nil {
		t.Error("Expected error when initializing with nil params")
	}
}
