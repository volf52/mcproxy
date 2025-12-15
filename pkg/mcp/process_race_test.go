package mcp

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestProcessManagerRaceConditions tests for race conditions in ProcessManager
func TestProcessManagerRaceConditions(t *testing.T) {
	// Use multiple CPU cores to increase likelihood of races
	runtime.GOMAXPROCS(runtime.NumCPU())

	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command:      []string{"mock-command"},
		RestartDelay: 1 * time.Millisecond,
		Commander:    &MockCommander{},
	})
	if err != nil {
		t.Fatalf("Failed to create ProcessManager: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 20

	// Test concurrent Start/Stop operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)

		// Start goroutine
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if !pm.IsRunning() {
					ctx := context.Background()
					_ = pm.Start(ctx)
				}
				time.Sleep(time.Millisecond)
			}
		}(i)

		// Stop goroutine
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if pm.IsRunning() {
					_ = pm.Stop()
				}
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Test concurrent GetPID calls
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations*2; j++ {
				pid := pm.GetPID()
				// PID should be non-negative
				if pid < 0 {
					t.Errorf("Invalid PID: %d", pid)
				}
			}
		}()
	}

	// Test concurrent IsRunning calls
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations*2; j++ {
				pm.IsRunning()
			}
		}()
	}

	wg.Wait()

	// Final cleanup
	if err := pm.Stop(); err != nil {
		t.Errorf("Final Stop error: %v", err)
	}
}

// TestProcessManagerRestartRace tests race conditions during process restarts
func TestProcessManagerRestartRace(t *testing.T) {
	// Use a mock commander that exits quickly
	mc := &MockCommander{}
	mc.OnCommand = func(ctx context.Context, name string, arg ...string) ExecCommand {
		m := NewMockExecCommand()
		// Exit after a tiny delay to trigger restarts
		go func() {
			time.Sleep(1 * time.Millisecond)
			m.Exit()
		}()
		return m
	}

	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command:      []string{"mock-true"},
		RestartDelay: 1 * time.Millisecond,
		Commander:    mc,
	})
	if err != nil {
		t.Fatalf("Failed to create ProcessManager: %v", err)
	}

	ctx := context.Background()
	if err := pm.Start(ctx); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Run operations while process is restarting
	for i := 0; i < numGoroutines; i++ {
		wg.Add(3)

		// Monitor PID changes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				pm.GetPID()
				time.Sleep(time.Millisecond)
			}
		}(i)

		// Check running state
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				pm.IsRunning()
				time.Sleep(time.Millisecond)
			}
		}(i)

		// Try to send messages (will likely fail but shouldn't panic)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = pm.SendNotification("test", map[string]interface{}{"id": id})
				time.Sleep(time.Millisecond * 2)
			}
		}(i)
	}

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	wg.Wait()

	if err := pm.Stop(); err != nil {
		t.Errorf("Failed to stop process: %v", err)
	}
}

// TestProcessManagerStress performs stress testing with many concurrent operations
func TestProcessManagerStress(t *testing.T) {
	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command:      []string{"mock-sleep"},
		RestartDelay: 10 * time.Millisecond,
		Commander:    &MockCommander{},
	})
	if err != nil {
		t.Fatalf("Failed to create ProcessManager: %v", err)
	}

	ctx := context.Background()
	if err := pm.Start(ctx); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	var wg sync.WaitGroup
	numOperations := 200

	// Stress test with many operations
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Randomly perform different operations
			switch id % 4 {
			case 0:
				pm.GetPID()
			case 1:
				pm.IsRunning()
			case 2:
				_ = pm.SendNotification("test", map[string]interface{}{"id": id})
			case 3:
				// Try to start (should fail if already running)
				_ = pm.Start(ctx)
			}
		}(i)
	}

	// Concurrently try to stop and restart
	for i := 0; i < 5; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			_ = pm.Stop()
		}()
		go func() {
			defer wg.Done()
			time.Sleep(20 * time.Millisecond)
			_ = pm.Start(ctx)
		}()
	}

	wg.Wait()

	if err := pm.Stop(); err != nil {
		t.Errorf("Failed to stop process: %v", err)
	}
}

// TestProcessManagerContextCancellation tests context cancellation during concurrent operations
func TestProcessManagerContextCancellation(t *testing.T) {
	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command:      []string{"mock-sleep"},
		RestartDelay: 10 * time.Millisecond,
		Commander:    &MockCommander{},
	})
	if err != nil {
		t.Fatalf("Failed to create ProcessManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := pm.Start(ctx); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Start operations that will be interrupted by context cancellation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// Check if context is cancelled
				if ctx.Err() != nil {
					break
				}
				pm.GetPID()
				pm.IsRunning()
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	wg.Wait()

	// Process should stop due to context cancellation
	// Use a shorter wait
	time.Sleep(20 * time.Millisecond)
	if pm.IsRunning() {
		t.Error("Process should have stopped after context cancellation")
	}

	if err := pm.Stop(); err != nil {
		t.Errorf("Failed to stop process: %v", err)
	}
}

// TestProcessManagerMultipleInstances tests multiple ProcessManager instances running concurrently
func TestProcessManagerMultipleInstances(t *testing.T) {
	numInstances := 5
	managers := make([]*ProcessManager, numInstances)
	var wg sync.WaitGroup

	// Create multiple ProcessManager instances
	for i := 0; i < numInstances; i++ {
		pm, err := NewProcessManager(&ProcessManagerOptions{
			Command:      []string{"mock-sleep"},
			RestartDelay: 1 * time.Millisecond,
			Commander:    &MockCommander{},
		})
		if err != nil {
			t.Fatalf("Failed to create ProcessManager %d: %v", i, err)
		}
		managers[i] = pm
	}

	// Start all instances concurrently
	for i := 0; i < numInstances; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()
			if err := managers[id].Start(ctx); err != nil {
				// t.Logf("Failed to start ProcessManager %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	// Perform operations on all instances
	for i := 0; i < numInstances; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				managers[id].GetPID()
				managers[id].IsRunning()
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Stop all instances
	for i := 0; i < numInstances; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := managers[id].Stop(); err != nil {
				// t.Logf("Failed to stop ProcessManager %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
}

// BenchmarkProcessManagerConcurrent benchmarks concurrent operations
func BenchmarkProcessManagerConcurrent(b *testing.B) {
	pm, err := NewProcessManager(&ProcessManagerOptions{
		Command:      []string{"sleep", "1"},
		RestartDelay: 10 * time.Millisecond,
	})
	if err != nil {
		b.Fatalf("Failed to create ProcessManager: %v", err)
	}

	ctx := context.Background()
	if err := pm.Start(ctx); err != nil {
		b.Fatalf("Failed to start process: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pm.GetPID()
			pm.IsRunning()
		}
	})

	if err := pm.Stop(); err != nil {
		b.Errorf("Failed to stop process: %v", err)
	}
}
