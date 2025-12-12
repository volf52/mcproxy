package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"mcproxy/pkg/logging"
)

const (
	// MaxMessageSize is the maximum size of a JSON-RPC message
	MaxMessageSize = 4 * 1024 * 1024 // 4MB
	// DefaultRestartDelay is the delay before restarting a failed process
	DefaultRestartDelay = 5 * time.Second
	// MaxRestartAttempts is the maximum number of restart attempts
	MaxRestartAttempts = 5
)

// ProcessManager manages MCP server processes
type ProcessManager struct {
	cmd     *exec.Cmd
	process *os.Process
	stdin   io.WriteCloser
	stdout  *bufio.Scanner
	stderr  io.Reader
	cancel  context.CancelFunc
	done    chan struct{}

	// Request/response handling
	pendingRequests map[interface{}]chan *JSONRPCResponse
	reqMu           sync.RWMutex
	requestID       int64

	// State
	running      int32
	restartCount int

	// Configuration
	command      []string
	env          map[string]string
	restartDelay time.Duration

	// Notifications
	notificationHandler func(*JSONRPCNotification)
}

// ProcessManagerOptions configures a ProcessManager
type ProcessManagerOptions struct {
	Command             []string
	Env                 map[string]string
	RestartDelay        time.Duration
	NotificationHandler func(*JSONRPCNotification)
}

// NewProcessManager creates a new MCP process manager
func NewProcessManager(opts *ProcessManagerOptions) (*ProcessManager, error) {
	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	restartDelay := opts.RestartDelay
	if restartDelay == 0 {
		restartDelay = DefaultRestartDelay
	}

	pm := &ProcessManager{
		command:             opts.Command,
		env:                 opts.Env,
		restartDelay:        restartDelay,
		pendingRequests:     make(map[interface{}]chan *JSONRPCResponse),
		done:                make(chan struct{}),
		notificationHandler: opts.NotificationHandler,
	}

	return pm, nil
}

// Start starts the MCP server process
func (pm *ProcessManager) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&pm.running, 0, 1) {
		return fmt.Errorf("process already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	pm.cancel = cancel

	go pm.run(ctx)

	return nil
}

// Stop stops the MCP server process
func (pm *ProcessManager) Stop() error {
	if !atomic.CompareAndSwapInt32(&pm.running, 1, 0) {
		return nil // Already stopped
	}

	if pm.cancel != nil {
		pm.cancel()
	}

	// Close stdin to signal EOF
	if pm.stdin != nil {
		pm.stdin.Close()
	}

	// Wait for process to finish
	<-pm.done

	// Terminate process if still running
	if pm.process != nil && pm.process.Pid > 0 {
		pm.process.Kill()
		_, _ = pm.process.Wait()
	}

	return nil
}

// IsRunning returns true if the process is currently running
func (pm *ProcessManager) IsRunning() bool {
	return atomic.LoadInt32(&pm.running) == 1
}

// run manages the process lifecycle
func (pm *ProcessManager) run(ctx context.Context) {
	defer close(pm.done)

	for {
		if pm.restartCount >= MaxRestartAttempts {
			logging.Printf("Process manager: max restart attempts reached, giving up")
			return
		}

		if err := pm.startProcess(ctx); err != nil {
			logging.Printf("Process manager: failed to start process: %v", err)
			atomic.StoreInt32(&pm.running, 0)
			return
		}

		// Wait for process to exit or context to be cancelled
		done := make(chan error, 1)
		go func() {
			_, err := pm.process.Wait()
			done <- err
		}()

		select {
		case <-ctx.Done():
			// Context cancelled, stop the process
			if pm.stdin != nil {
				pm.stdin.Close()
			}
			<-done // Wait for process to actually exit
			return

		case err := <-done:
			// Process exited, check if we should restart
			logging.Printf("Process manager: process exited: %v", err)

			// Close stdin to prevent further writes
			if pm.stdin != nil {
				pm.stdin.Close()
				pm.stdin = nil
			}

			pm.restartCount++

			// Don't restart if context is cancelled
			if ctx.Err() != nil {
				return
			}

			// Wait before restarting
			select {
			case <-time.After(pm.restartDelay):
				continue
			case <-ctx.Done():
				return
			}
		}
	}
}

// startProcess starts a single instance of the MCP server process
func (pm *ProcessManager) startProcess(ctx context.Context) error {
	// Create command
	cmd := exec.CommandContext(ctx, pm.command[0], pm.command[1:]...)

	// Set up environment
	if len(pm.env) > 0 {
		env := cmd.Env
		for k, v := range pm.env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	// Create pipes for stdin/stdout/stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Store process references
	pm.cmd = cmd
	pm.process = cmd.Process
	pm.stdin = stdin
	pm.stderr = stderr

	// Set up stdout scanner with large buffer
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, MaxMessageSize)
	scanner.Buffer(buf, MaxMessageSize)
	scanner.Split(bufio.ScanLines)
	pm.stdout = scanner

	// Start stderr logger
	go pm.logStderr(stderr)

	// Start message reader
	go pm.readMessages()

	logging.Printf("Process manager: started MCP server process (PID: %d)", cmd.Process.Pid)

	return nil
}

// logStderr logs stderr output from the process
func (pm *ProcessManager) logStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		logging.Printf("Process stderr: %s", scanner.Text())
	}
}

// readMessages reads JSON-RPC messages from stdout
func (pm *ProcessManager) readMessages() {
	for pm.stdout.Scan() {
		line := pm.stdout.Text()
		if line == "" {
			continue
		}

		// Try to parse as JSON-RPC response first
		var response JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &response); err == nil && response.ID != nil {
			pm.handleResponse(&response)
			continue
		}

		// Try to parse as JSON-RPC notification
		var notification JSONRPCNotification
		if err := json.Unmarshal([]byte(line), &notification); err == nil {
			pm.handleNotification(&notification)
			continue
		}

		// Try to parse as generic JSON-RPC request (for unexpected requests from server)
		var request JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &request); err == nil {
			logging.Printf("Process manager: received unexpected request from server: %s", request.Method)
			continue
		}

		logging.Printf("Process manager: failed to parse JSON-RPC message: %s", line)
	}

	if err := pm.stdout.Err(); err != nil {
		logging.Printf("Process manager: stdout read error: %v", err)
	}
}

// handleResponse handles a JSON-RPC response
func (pm *ProcessManager) handleResponse(response *JSONRPCResponse) {
	pm.reqMu.RLock()
	ch, exists := pm.pendingRequests[response.ID]
	pm.reqMu.RUnlock()

	if !exists {
		logging.Printf("Process manager: received response for unknown request ID: %v", response.ID)
		return
	}

	select {
	case ch <- response:
	default:
		// Channel is full or closed, drop the response
		logging.Printf("Process manager: dropped response for request ID: %v", response.ID)
	}

	pm.reqMu.Lock()
	delete(pm.pendingRequests, response.ID)
	pm.reqMu.Unlock()
}

// handleNotification handles a JSON-RPC notification
func (pm *ProcessManager) handleNotification(notification *JSONRPCNotification) {
	if pm.notificationHandler != nil {
		go pm.notificationHandler(notification)
	}
}

// SendRequest sends a JSON-RPC request and waits for the response
func (pm *ProcessManager) SendRequest(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	if !pm.IsRunning() {
		return nil, fmt.Errorf("process is not running")
	}

	// Generate request ID
	id := atomic.AddInt64(&pm.requestID, 1)

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

	// Create response channel
	ch := make(chan *JSONRPCResponse, 1)
	pm.reqMu.Lock()
	pm.pendingRequests[id] = ch
	pm.reqMu.Unlock()

	// Clean up pending request when done
	defer func() {
		pm.reqMu.Lock()
		delete(pm.pendingRequests, id)
		pm.reqMu.Unlock()
		close(ch)
	}()

	// Send request
	if err := pm.sendMessage(&request); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	select {
	case response := <-ch:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timed out")
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (pm *ProcessManager) SendNotification(method string, params interface{}) error {
	if !pm.IsRunning() {
		return fmt.Errorf("process is not running")
	}

	notification := JSONRPCNotification{
		JSONRPC: JSONRPCVersion20,
		Method:  method,
	}

	if params != nil {
		paramsBytes, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		notification.Params = paramsBytes
	}

	return pm.sendMessage(&notification)
}

// sendMessage sends a JSON-RPC message to the process
func (pm *ProcessManager) sendMessage(msg interface{}) error {
	if pm.stdin == nil {
		return fmt.Errorf("stdin is not available")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write message with newline
	data = append(data, '\n')
	if _, err := pm.stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}

	return nil
}
