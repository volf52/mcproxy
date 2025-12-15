package mcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestClientRaceConditionFix specifically tests that the race condition
// in client initialization has been fixed
func TestClientRaceConditionFix(t *testing.T) {
	// This test should be run with: go test -race ./pkg/mcp -run TestClientRaceConditionFix

	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command: []string{"echo", "test"},
	})
	assert.NoError(t, err)
	defer pm.Stop()

	client := NewMCPClient(pm)

	params := &InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Experimental: make(map[string]interface{}),
		},
		ClientInfo: ImplementationInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	// Test concurrent initialization without stopping/starting the process
	// to avoid ProcessManager race conditions
	const numGoroutines = 10
	var ch = make(chan error, numGoroutines)

	// Launch multiple goroutines trying to initialize simultaneously
	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := client.Initialize(params)
			ch <- err
		}()
	}

	// Collect all results
	var errors []error
	for i := 0; i < numGoroutines; i++ {
		errors = append(errors, <-ch)
	}

	// All should fail gracefully (since echo is not an MCP server)
	// but without race conditions or panics
	for i, err := range errors {
		assert.Error(t, err, "Initialization %d should fail", i)
	}
}

// TestClientInitializationState tests that initialization state is
// properly maintained under concurrent access
func TestClientInitializationState(t *testing.T) {
	// This test should be run with: go test -race ./pkg/mcp -run TestClientInitializationState

	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command: []string{"echo", "test"},
	})
	assert.NoError(t, err)
	defer pm.Stop()

	client := NewMCPClient(pm)

	params := &InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Experimental: make(map[string]interface{}),
		},
		ClientInfo: ImplementationInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	// Test state consistency under concurrent access
	const numGoroutines = 5
	done := make(chan struct{}, numGoroutines)

	// Start multiple goroutines that all check the state
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()

			// Check initial state
			healthy := client.IsHealthy()
			assert.False(t, healthy, "Client should not be healthy initially")

			// Try to initialize
			_, err := client.Initialize(params)
			// Should fail but not panic
			assert.Error(t, err)

			// Check state after initialization attempt
			client.IsHealthy()
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out - possible deadlock")
		}
	}
}
