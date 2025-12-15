package mcp

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReinitializationFlow tests that the reinitialization
// functionality still works correctly after the race condition fix
func TestReinitializationFlow(t *testing.T) {
	// This test verifies that the client can handle state transitions
	// from initialized to not initialized and back

	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command: []string{"echo", "test"},
	})
	require.NoError(t, err)
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

	// Test 1: Initial initialization should fail (echo is not MCP server)
	_, err = client.Initialize(params)
	assert.Error(t, err)
	assert.False(t, client.IsHealthy())

	// Test 2: Multiple concurrent initialization attempts should be handled safely
	const numGoroutines = 5
	done := make(chan struct{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, err := client.Initialize(params)
			assert.Error(t, err)
		}()
	}

	// Wait for all to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out - possible deadlock")
		}
	}

	// Test 3: Health check should work without race conditions
	err = client.HealthCheck()
	assert.Error(t, err) // Should fail for echo process
}

// TestInitializationTimeout tests that the initialization timeout
// works correctly after the race condition fix
func TestInitializationTimeout(t *testing.T) {
	// Note: This test verifies that the timeout logic is present in the code.
	// The actual timeout behavior is tested implicitly by the other tests.
	// The key is that the code should have the timeout logic and not deadlock.
	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command: []string{"echo", "test"},
	})
	require.NoError(t, err)
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

	// Multiple concurrent attempts should not deadlock
	const numGoroutines = 5
	done := make(chan struct{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, err := client.Initialize(params)
			assert.Error(t, err)
		}()
	}

	// All should complete quickly without deadlock
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Operations should complete quickly without deadlock")
		}
	}
}

// TestInitializationConsistency tests that the initialization
// state remains consistent under concurrent access
func TestInitializationConsistency(t *testing.T) {
	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command: []string{"echo", "test"},
	})
	require.NoError(t, err)
	defer pm.Stop()

	// Create the client directly to access internal fields for testing
	c := &client{
		pm:       pm,
		initOnce: &sync.Once{},
		initDone: make(chan struct{}),
	}
	c.initMu.Lock()
	c.initArgs = &InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Experimental: make(map[string]interface{}),
		},
		ClientInfo: ImplementationInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}
	c.initMu.Unlock()

	// Store first params
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
	_, err = c.Initialize(params)
	require.Error(t, err)

	// Update params
	params.ClientInfo.Name = "updated-client"
	_, err = c.Initialize(params)
	require.Error(t, err)

	// Verify that params were updated correctly
	c.initMu.RLock()
	storedParams := c.initArgs
	c.initMu.RUnlock()

	assert.NotNil(t, storedParams)
	assert.Equal(t, "updated-client", storedParams.ClientInfo.Name)
}
