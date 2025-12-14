package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewMCPClient tests creating a new MCP client
func TestNewMCPClient(t *testing.T) {
	opts := &ProcessManagerOptions{
		Command: []string{"/bin/echo", "test"},
	}

	pm, err := NewProcessManager(opts)
	if err != nil {
		t.Fatalf("Failed to create process manager: %v", err)
	}

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
	opts := &ProcessManagerOptions{
		Command: []string{"/bin/echo", "test"},
	}

	pm, err := NewProcessManager(opts)
	if err != nil {
		t.Fatalf("Failed to create process manager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

// MockProcessManager is a mock implementation of ProcessManagerInterface for testing
type MockProcessManager struct {
	running   int32
	pending   map[interface{}]chan *JSONRPCResponse
	mu        sync.RWMutex
	requestID int64
}

func (m *MockProcessManager) Start(ctx context.Context) error {
	atomic.StoreInt32(&m.running, 1)
	return nil
}

func (m *MockProcessManager) Stop() error {
	atomic.StoreInt32(&m.running, 0)
	m.cleanupPendingRequests(fmt.Errorf("process stopped"))
	return nil
}

func (m *MockProcessManager) IsRunning() bool {
	return atomic.LoadInt32(&m.running) == 1
}

func (m *MockProcessManager) SendRequest(method string, params interface{}) (*JSONRPCResponse, error) {
	if !m.IsRunning() {
		return nil, fmt.Errorf("process not running")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate request ID
	id := atomic.AddInt64(&m.requestID, 1)

	// Create request
	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion20,
		Method:  method,
		ID:      id,
	}

	if params != nil {
		paramsBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		request.Params = paramsBytes
	}

	// For testing, return a simple mock response
	result, _ := json.Marshal(map[string]interface{}{"status": "ok"})
	response := &JSONRPCResponse{
		JSONRPC: JSONRPCVersion20,
		ID:      id,
		Result:  result,
	}

	return response, nil
}

func (m *MockProcessManager) SendNotification(method string, params interface{}) error {
	if !m.IsRunning() {
		return fmt.Errorf("process not running")
	}
	return nil
}

func (m *MockProcessManager) cleanupPendingRequests(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear ALL pending requests
	for id, ch := range m.pending {
		// Create error response
		errorResp := &JSONRPCResponse{
			JSONRPC: JSONRPCVersion20,
			ID:      id,
			Error:   NewInternalError("process terminated"),
		}

		// Send error to waiting goroutine (non-blocking)
		select {
		case ch <- errorResp:
		default:
			// Channel is closed or full
		}

		// Close channel only if it's not already closed
		select {
		case <-ch:
		default:
			close(ch)
		}
	}

	// Clear the map
	m.pending = make(map[interface{}]chan *JSONRPCResponse)
}

func NewMockProcessManager() *MockProcessManager {
	return &MockProcessManager{
		pending: make(map[interface{}]chan *JSONRPCResponse),
	}
}
