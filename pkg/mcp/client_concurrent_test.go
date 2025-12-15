package mcp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentInitialization tests that multiple goroutines can safely
// attempt to initialize the client concurrently
func TestConcurrentInitialization(t *testing.T) {
	// Create a real process manager with a non-MCP command
	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command: []string{"echo"},
	})
	require.NoError(t, err)
	defer pm.Stop()

	client := NewMCPClient(pm)

	// Prepare initialization parameters
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

	// Number of concurrent goroutines
	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([]*InitializeResult, numGoroutines)
	errors := make([]error, numGoroutines)

	// Launch multiple goroutines trying to initialize simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errors[index] = client.Initialize(params)
		}(i)
	}

	// Wait for all to complete
	wg.Wait()

	// At least some should fail (since we're using echo, not a real MCP server)
	// But the important thing is that there should be no race conditions
	var failureCount int
	for i := 0; i < numGoroutines; i++ {
		if errors[i] != nil {
			failureCount++
		}
	}

	// All should fail gracefully
	assert.Equal(t, numGoroutines, failureCount, "All initialization attempts should fail gracefully")
}

// TestRaceConditionDetection tests for race conditions in concurrent access
func TestRaceConditionDetection(t *testing.T) {
	// This test is designed to detect race conditions by running with the race detector
	// Run: go test -race ./pkg/mcp -run TestRaceConditionDetection

	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command: []string{"sleep", "10"},
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

	// This pattern simulates rapid concurrent access that could trigger race conditions
	const numIterations = 10
	var wg sync.WaitGroup

	for i := 0; i < numIterations; i++ {
		wg.Add(3) // Three goroutines per iteration

		// Goroutine 1: Try to initialize
		go func() {
			defer wg.Done()
			client.Initialize(params)
		}()

		// Goroutine 2: Check health
		go func() {
			defer wg.Done()
			client.IsHealthy()
		}()

		// Goroutine 3: Try to initialize with nil params (should use cached params)
		go func() {
			defer wg.Done()
			client.Initialize(nil)
		}()
	}

	// Stop and start the process manager to trigger reinitialization
	go func() {
		for i := 0; i < 3; i++ {
			time.Sleep(10 * time.Millisecond)
			pm.Stop()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()

	// If we reach here without race detector warnings, the test passes
	t.Log("Race condition detection test completed - no race conditions detected")
}

// TestConcurrentHealthCheck tests concurrent health checks
func TestConcurrentHealthCheck(t *testing.T) {
	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command: []string{"echo"},
	})
	require.NoError(t, err)
	defer pm.Stop()

	client := NewMCPClient(pm)

	// Launch multiple concurrent health checks
	const numGoroutines = 50
	var wg sync.WaitGroup
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			errors[index] = client.HealthCheck()
		}(i)
	}

	// Wait for all to complete
	wg.Wait()

	// Verify that all health checks completed without panics
	// They should all fail gracefully since echo is not an MCP server
	for i := 0; i < numGoroutines; i++ {
		assert.Error(t, errors[i], "Health check should fail for non-MCP process")
	}
}

// TestInitializationStateTransitions tests that initialization state transitions
// are handled correctly under concurrent access
func TestInitializationStateTransitions(t *testing.T) {
	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command: []string{"cat"},
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

	// Start the process initially
	err = pm.Start(context.Background())
	require.NoError(t, err)

	// Start and stop the process manager repeatedly while trying to initialize
	const numCycles = 5
	var wg sync.WaitGroup

	for cycle := 0; cycle < numCycles; cycle++ {
		wg.Add(2)

		// Goroutine that starts/stops the process
		go func() {
			defer wg.Done()
			for i := 0; i < 3; i++ {
				pm.Stop()
				time.Sleep(5 * time.Millisecond)
				pm.Start(context.Background())
				time.Sleep(5 * time.Millisecond)
			}
		}()

		// Goroutine that tries to initialize
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				client.Initialize(params)
				client.Initialize(nil) // Use cached params
				client.HealthCheck()
				client.IsHealthy()
			}
		}()
	}

	wg.Wait()

	// Verify final state
	assert.False(t, client.IsHealthy(), "Client should not be healthy with cat process")
}
