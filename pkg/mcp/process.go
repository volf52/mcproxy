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
	"syscall"
	"time"

	"mcproxy/pkg/logging"
)

const (
	// MaxTokenSize is the maximum size of a JSON-RPC message (4MB as per MCP spec)
	MaxTokenSize = 4 * 1024 * 1024 // 4MB
	// MaxMessageSize is deprecated, use MaxTokenSize instead
	// Kept for backward compatibility
	MaxMessageSize = MaxTokenSize
	// DefaultRestartDelay is the delay before restarting a failed process
	DefaultRestartDelay = 5 * time.Second
	// MaxRestartAttempts is the maximum number of restart attempts
	MaxRestartAttempts = 5
)

// Commander defines an interface for executing commands, allowing for mocking in tests
type Commander interface {
	Command(ctx context.Context, name string, arg ...string) ExecCommand
}

// ExecCommand defines an interface for a running command
type ExecCommand interface {
	Start() error
	Wait() error
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	GetProcess() *os.Process
}

// RealCommander implements Commander using os/exec
type RealCommander struct{}

func (c *RealCommander) Command(ctx context.Context, name string, arg ...string) ExecCommand {
	return &RealExecCommand{cmd: exec.CommandContext(ctx, name, arg...)}
}

type RealExecCommand struct {
	cmd *exec.Cmd
}

func (c *RealExecCommand) Start() error { return c.cmd.Start() }
func (c *RealExecCommand) Wait() error  { return c.cmd.Wait() }
func (c *RealExecCommand) StdinPipe() (io.WriteCloser, error) {
	return c.cmd.StdinPipe()
}
func (c *RealExecCommand) StdoutPipe() (io.ReadCloser, error) {
	return c.cmd.StdoutPipe()
}
func (c *RealExecCommand) StderrPipe() (io.ReadCloser, error) {
	return c.cmd.StderrPipe()
}
func (c *RealExecCommand) GetProcess() *os.Process { return c.cmd.Process }

// ProcessManager manages MCP server processes
type ProcessManager struct {
	commander  Commander
	cmd        ExecCommand
	process    *os.Process
	stdin      io.WriteCloser
	stdoutPipe io.ReadCloser
	stderr     io.Reader
	cancel     context.CancelFunc
	done       chan struct{}
	doneMu     sync.Mutex

	// Process fields mutex - protects cmd, process, stdin, stdoutPipe, stderr, cancel, restartCount
	processMu sync.RWMutex

	// readerStop signals the readMessages goroutine to stop
	readerStop   chan struct{}
	readerStopMu sync.Mutex
	readerActive int32 // atomic flag to track if reader is active

	// readyChan signals when the first process has started
	readyChan chan error

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
	Commander           Commander // Optional custom commander for testing
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

	commander := opts.Commander
	if commander == nil {
		commander = &RealCommander{}
	}

	pm := &ProcessManager{
		commander:           commander,
		command:             opts.Command,
		env:                 opts.Env,
		restartDelay:        restartDelay,
		pendingRequests:     make(map[interface{}]chan *JSONRPCResponse),
		done:                make(chan struct{}),
		notificationHandler: opts.NotificationHandler,
		readerStop:          make(chan struct{}),
		readyChan:           make(chan error, 1),
	}

	return pm, nil
}

// Start starts the MCP server process
func (pm *ProcessManager) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&pm.running, 0, 1) {
		return fmt.Errorf("process already running")
	}

	// Create context first (outside lock)
	ctx, cancel := context.WithCancel(ctx)

	// Acquire lock to set cancel function
	pm.processMu.Lock()
	pm.cancel = cancel
	pm.restartCount = 0 // Reset restart count on start
	pm.processMu.Unlock()

	// Ensure readyChan is empty
	select {
	case <-pm.readyChan:
	default:
	}

	// Start the run loop
	go pm.run(ctx)

	// Wait for process to be ready or fail
	select {
	case err := <-pm.readyChan:
		if err != nil {
			atomic.StoreInt32(&pm.running, 0)
			return fmt.Errorf("failed to start process: %w", err)
		}
		return nil
	case <-ctx.Done():
		atomic.StoreInt32(&pm.running, 0)
		return fmt.Errorf("context canceled while waiting for process to start: %w", ctx.Err())
	}
}

// Stop stops the MCP server process
func (pm *ProcessManager) Stop() error {
	if !atomic.CompareAndSwapInt32(&pm.running, 1, 0) {
		return nil // Already stopped
	}

	// Capture all references under single lock
	pm.processMu.Lock()
	var (
		cancel     = pm.cancel
		stdin      = pm.stdin
		stdoutPipe = pm.stdoutPipe
		stderr     = pm.stderr
		process    = pm.process
		hasProc    = pm.process != nil && pm.process.Pid > 0
	)

	// Clear references to prevent further access
	pm.cancel = nil
	pm.stdin = nil
	pm.process = nil
	pm.stderr = nil
	pm.stdoutPipe = nil
	pm.processMu.Unlock()

	// Perform blocking operations outside lock

	// Signal the reader goroutine to stop
	pm.readerStopMu.Lock()
	if pm.readerStop != nil {
		close(pm.readerStop)
		pm.readerStop = nil
	}
	pm.readerStopMu.Unlock()

	// Reset reader active flag
	atomic.StoreInt32(&pm.readerActive, 0)

	// Cancel the context first
	if cancel != nil {
		cancel()
	}

	// Close stdin to signal EOF
	if stdin != nil {
		_ = stdin.Close()
	}

	// Close stdout pipe to signal EOF to reader
	if stdoutPipe != nil {
		_ = stdoutPipe.Close()
	}

	// Clean up any pending requests
	pm.cleanupPendingRequests(fmt.Errorf("process stopped"))

	// Wait for process to finish with proper synchronization
	pm.doneMu.Lock()
	doneChan := pm.done
	pm.doneMu.Unlock()

	if doneChan != nil {
		<-doneChan
	}

	// Terminate process if still running
	if hasProc && process != nil {
		_ = process.Kill()
		_, _ = process.Wait()
	}

	// Close stderr if it exists
	if stderr != nil {
		if closer, ok := stderr.(io.Closer); ok {
			_ = closer.Close()
		}
	}

	return nil
}

// IsRunning returns true if the process is currently running
func (pm *ProcessManager) IsRunning() bool {
	return atomic.LoadInt32(&pm.running) == 1
}

// run manages the process lifecycle
func (pm *ProcessManager) run(ctx context.Context) {
	pm.doneMu.Lock()
	if pm.done == nil {
		pm.done = make(chan struct{})
	}
	pm.doneMu.Unlock()

	defer func() {
		pm.doneMu.Lock()
		if pm.done != nil {
			close(pm.done)
			pm.done = nil
		}
		pm.doneMu.Unlock()
	}()

	for {
		// Check if we should restart
		pm.processMu.Lock()
		currentRestartCount := pm.restartCount
		shouldRestart := currentRestartCount < MaxRestartAttempts
		if shouldRestart {
			pm.restartCount++
		}
		pm.processMu.Unlock()

		if !shouldRestart {
			logging.Printf("Process manager: max restart attempts reached, giving up")
			return
		}

		if err := pm.startProcess(ctx); err != nil {
			logging.Printf("Process manager: failed to start process: %v", err)
			atomic.StoreInt32(&pm.running, 0)

			// Signal error if first attempt
			select {
			case pm.readyChan <- err:
			default:
			}
			return
		}

		// Wait for process to exit or context to be cancelled
		err := pm.waitForProcess()

		// Don't restart if context is cancelled
		if ctx.Err() != nil {
			// Clear running flag when exiting due to context cancellation
			atomic.StoreInt32(&pm.running, 0)
			return
		}

		// Log process exit
		logging.Printf("Process manager: process exited: %v", err)

		// Wait before restarting
		select {
		case <-time.After(pm.restartDelay):
			continue
		case <-ctx.Done():
			return
		}
	}
}

// waitForProcess waits for the process to exit with proper synchronization
func (pm *ProcessManager) waitForProcess() error {
	// Capture process reference under lock
	pm.processMu.RLock()
	process := pm.process
	stdin := pm.stdin
	stdoutPipe := pm.stdoutPipe
	pm.processMu.RUnlock()

	if process == nil {
		return fmt.Errorf("no process to wait for")
	}

	// Wait for process outside lock
	_, err := process.Wait()

	// Clean up pipes after process exits
	if stdin != nil {
		_ = stdin.Close()
	}
	if stdoutPipe != nil {
		_ = stdoutPipe.Close()
	}

	// Clear pipe references after closing
	pm.processMu.Lock()
	if pm.stdin == stdin {
		pm.stdin = nil
	}
	if pm.stdoutPipe == stdoutPipe {
		pm.stdoutPipe = nil
	}
	pm.processMu.Unlock()

	return err
}

// startProcess starts a single instance of the MCP server process
func (pm *ProcessManager) startProcess(ctx context.Context) error {
	// Create command using the commander
	cmd := pm.commander.Command(ctx, pm.command[0], pm.command[1:]...)

	// Set up environment if it's a RealExecCommand (we can't easily set env on the interface)
	// In a real refactor, Env would be part of the Command interface or the Commander.Command call.
	// For now, we'll keep it simple and only support env for real processes,
	// or assume the mock handles it.
	if realCmd, ok := cmd.(*RealExecCommand); ok && len(pm.env) > 0 {
		env := os.Environ()
		for k, v := range pm.env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		realCmd.cmd.Env = env
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

	// Store process references under lock BEFORE starting goroutines
	pm.processMu.Lock()
	pm.cmd = cmd
	pm.process = cmd.GetProcess()
	pm.stdin = stdin
	pm.stderr = stderr
	pm.stdoutPipe = stdout
	pm.processMu.Unlock()

	// Start stderr logger
	go pm.logStderr(stderr)

	// Start message reader
	pm.readerStopMu.Lock()
	if pm.readerStop != nil {
		close(pm.readerStop)
	}
	pm.readerStop = make(chan struct{})
	pm.readerStopMu.Unlock()
	go pm.readMessages()

	logging.Printf("Process manager: started process with command: %v (PID: %d)", pm.command, cmd.GetProcess().Pid)

	// Signal success to Start
	select {
	case pm.readyChan <- nil:
	default:
	}

	return nil
}

// GetPID returns the PID of the managed process, or 0 if no process is running
func (pm *ProcessManager) GetPID() int {
	pm.processMu.RLock()
	defer pm.processMu.RUnlock()

	if pm.process != nil {
		return pm.process.Pid
	}
	return 0
}

// isProcessAlive checks if the process is still running
func (pm *ProcessManager) isProcessAlive() bool {
	pm.processMu.RLock()
	process := pm.process
	pm.processMu.RUnlock()

	if process == nil {
		return false
	}

	// Send signal 0 to check if process exists
	err := process.Signal(syscall.Signal(0))
	return err == nil
}

// logStderr logs stderr output from the process
func (pm *ProcessManager) logStderr(stderr io.Reader) {
	// Check if stderr is still valid before using it
	pm.processMu.RLock()
	currentStderr := pm.stderr
	pm.processMu.RUnlock()

	// If stderr has been cleared, stop logging
	if currentStderr != stderr {
		return
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		// Check if stderr is still valid on each iteration
		pm.processMu.RLock()
		currentStderr = pm.stderr
		pm.processMu.RUnlock()

		if currentStderr != stderr {
			return
		}

		logging.Printf("Process stderr: %s", scanner.Text())
	}
}

// readMessages reads JSON-RPC messages from stdout
func (pm *ProcessManager) readMessages() {
	// Try to become the active reader
	if !atomic.CompareAndSwapInt32(&pm.readerActive, 0, 1) {
		// Another reader is already active
		return
	}
	defer atomic.StoreInt32(&pm.readerActive, 0)

	// Get the stdout pipe
	pm.processMu.Lock()
	pipe := pm.stdoutPipe
	pm.processMu.Unlock()

	// If no pipe, return
	if pipe == nil {
		return
	}

	// Create a private scanner for this goroutine
	scanner := bufio.NewScanner(pipe)
	buf := make([]byte, 0, MaxTokenSize)
	scanner.Buffer(buf, MaxTokenSize)
	scanner.Split(bufio.ScanLines)

	// Get the stop channel
	pm.readerStopMu.Lock()
	stop := pm.readerStop
	pm.readerStopMu.Unlock()

	for {
		select {
		case <-stop:
			// Stop signal received
			return
		default:
			// Continue scanning with private scanner
			if !scanner.Scan() {
				// stdout closed or error occurred
				if err := scanner.Err(); err != nil {
					logging.Printf("Process manager: stdout read error: %v", err)
					pm.cleanupPendingRequests(fmt.Errorf("stdout read error: %w", err))
				} else {
					pm.cleanupPendingRequests(fmt.Errorf("stdout closed"))
				}
				return
			}

			line := scanner.Text()
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
	}
}

// handleResponse handles a JSON-RPC response
func (pm *ProcessManager) handleResponse(response *JSONRPCResponse) {
	// Normalize ID from float64 to int64 (JSON unmarshals interface{} numbers as float64)
	id := response.ID
	if f, ok := id.(float64); ok {
		id = int64(f)
	}

	pm.reqMu.Lock()
	ch, exists := pm.pendingRequests[id]
	if exists {
		// Delete first to prevent double-cleanup
		delete(pm.pendingRequests, id)
	}
	pm.reqMu.Unlock()

	if !exists {
		logging.Printf("Process manager: received response for unknown request ID: %v (type: %T)", response.ID, response.ID)
		return
	}

	// Try to send response, but don't block if channel is closed
	select {
	case ch <- response:
		// Successfully sent
	default:
		// Channel is full or closed, drop the response
		// Don't log as error - this is expected during shutdown
	}
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
		// Remove and close if still exists (handleResponse might have removed it)
		if ch, exists := pm.pendingRequests[id]; exists {
			delete(pm.pendingRequests, id)
			close(ch)
		}
		pm.reqMu.Unlock()
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

// cleanupPendingRequests closes all pending request channels and clears the map
func (pm *ProcessManager) cleanupPendingRequests(err error) {
	pm.reqMu.Lock()
	defer pm.reqMu.Unlock()

	// Clear ALL pending requests
	for id, ch := range pm.pendingRequests {
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
	pm.pendingRequests = make(map[interface{}]chan *JSONRPCResponse)

	// Log cleanup
	logging.Printf("Process manager: cleaned up pending requests due to: %v", err)
}

// sendMessage sends a JSON-RPC message to the process
func (pm *ProcessManager) sendMessage(msg interface{}) error {
	pm.processMu.RLock()
	stdin := pm.stdin
	pm.processMu.RUnlock()

	if stdin == nil {
		return fmt.Errorf("stdin is not available")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write message with newline
	data = append(data, '\n')
	if _, err := stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}

	return nil
}
