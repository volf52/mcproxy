package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"mcproxy/pkg/logging"
)

// client implements the MCPClient interface with re-initialization support
type client struct {
	pm *ProcessManager

	// Re-initialization support
	initMu      sync.RWMutex
	initArgs    *InitializeParams
	initState   int32 // 0=not init, 1=initializing, 2=initialized, 3=error
	initErr     error
	initResult  *InitializeResult
	initVersion int64 // Increment on each re-init

	// Synchronization for concurrent initialization
	initOnce    *sync.Once    // Per-version once initialization
	initOnceMu  sync.Mutex    // Mutex to protect initOnce recreation
	initWaiters int32         // Count of waiting goroutines
	initDone    chan struct{} // Channel for notifying completion (recreated each time)
	initDoneMu  sync.Mutex    // Mutex to protect initDone recreation
}

// NewMCPClient creates a new MCP client using the given process manager
func NewMCPClient(pm *ProcessManager) MCPClient {
	c := &client{
		pm:       pm,
		initOnce: &sync.Once{},
		initDone: make(chan struct{}),
	}

	// Start re-initialization monitor
	go c.reinitMonitor()

	return c
}

// reinitMonitor monitors for process restarts and triggers re-initialization
func (c *client) reinitMonitor() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastRunningState bool

	for {
		select {
		case <-ticker.C:
			currentlyRunning := c.pm.IsRunning()

			// Detect transition from running to not running
			if lastRunningState && !currentlyRunning {
				logging.Printf("Client: detected process stop, will re-initialize on restart")
				atomic.StoreInt32(&c.initState, 0)

				// Wake up any waiters by recreating the channel
				c.initDoneMu.Lock()
				c.initDone = make(chan struct{})
				c.initDoneMu.Unlock()
			}

			lastRunningState = currentlyRunning

			// Check if we need to re-initialize
			// Only re-initialize if we have initArgs (meaning we were initialized before)
			if currentlyRunning && atomic.LoadInt32(&c.initState) == 0 {
				c.initMu.RLock()
				hasInitArgs := c.initArgs != nil
				c.initMu.RUnlock()

				if hasInitArgs {
					go func() {
						if _, err := c.Initialize(c.initArgs); err != nil {
							logging.Printf("Client: failed to re-initialize: %v", err)
						}
					}()
				}
			}
		}
	}
}

// Initialize sends an initialize request and waits for response
// Can be called multiple times to re-initialize after process restart
func (c *client) Initialize(params *InitializeParams) (*InitializeResult, error) {
	// Store init args if provided
	if params != nil {
		c.initMu.Lock()
		c.initArgs = params
		c.initMu.Unlock()
	}

	for {
		state := atomic.LoadInt32(&c.initState)

		switch state {
		case 2: // Initialized
			// Check if process is still running
			if !c.pm.IsRunning() {
				// Process died, reset state and try again
				c.resetInitState(0)
				continue
			}
			// Return cached result
			c.initMu.RLock()
			result := c.initResult
			c.initMu.RUnlock()
			return result, nil

		case 0: // Not initialized
			if atomic.CompareAndSwapInt32(&c.initState, 0, 1) {
				// We won the race to initialize
				// Create a new done channel for this initialization cycle
				c.initDoneMu.Lock()
				c.initDone = make(chan struct{})
				c.initDoneMu.Unlock()

				// Use sync.Once to ensure initialization happens only once per version
				var initResult *InitializeResult
				var initErr error

				c.initOnceMu.Lock()
				atomic.AddInt64(&c.initVersion, 1)
				c.initOnce = &sync.Once{} // New version, new Once
				c.initOnceMu.Unlock()

				c.initOnce.Do(func() {
					initResult, initErr = c.doInitialize()
					c.initMu.Lock()
					c.initResult = initResult
					c.initErr = initErr
					c.initMu.Unlock()

					if initErr != nil {
						atomic.StoreInt32(&c.initState, 3)
					} else {
						atomic.StoreInt32(&c.initState, 2)
					}

					// Wake up all waiters by closing the channel
					c.initDoneMu.Lock()
					close(c.initDone)
					c.initDoneMu.Unlock()
				})

				return initResult, initErr
			}
			// Lost race, loop again

		case 1: // Another goroutine initializing
			// Get the current done channel to wait on
			c.initDoneMu.Lock()
			currentDone := c.initDone
			atomic.AddInt32(&c.initWaiters, 1)
			c.initDoneMu.Unlock()

			// Wait for initialization to complete or timeout
			select {
			case <-currentDone:
				// Initialization completed, check the result
				state = atomic.LoadInt32(&c.initState)
				if state == 2 {
					c.initMu.RLock()
					result := c.initResult
					c.initMu.RUnlock()
					atomic.AddInt32(&c.initWaiters, -1)
					return result, nil
				} else if state == 3 {
					c.initMu.RLock()
					err := c.initErr
					c.initMu.RUnlock()
					atomic.AddInt32(&c.initWaiters, -1)
					return nil, err
				}
				// State changed unexpectedly, loop again
				atomic.AddInt32(&c.initWaiters, -1)
				continue

			case <-time.After(30 * time.Second):
				atomic.AddInt32(&c.initWaiters, -1)
				return nil, fmt.Errorf("initialization timeout")
			}

		case 3: // Error state
			// Try to re-initialize if process is running
			if c.pm.IsRunning() {
				c.resetInitState(0)
				continue
			}
			c.initMu.RLock()
			err := c.initErr
			c.initMu.RUnlock()
			return nil, fmt.Errorf("initialization failed and process is not running: %w", err)
		}
	}
}

// resetInitState safely resets the initialization state
func (c *client) resetInitState(newState int32) {
	atomic.StoreInt32(&c.initState, newState)
	// Wake up any waiters by recreating the channel
	c.initDoneMu.Lock()
	c.initDone = make(chan struct{})
	c.initDoneMu.Unlock()
}

// doInitialize performs the actual initialization work
func (c *client) doInitialize() (*InitializeResult, error) {
	c.initMu.RLock()
	params := c.initArgs
	c.initMu.RUnlock()

	if params == nil {
		return nil, fmt.Errorf("no initialization parameters provided")
	}

	logging.Printf("Client: initializing MCP connection")

	// Send initialize request
	response, err := c.pm.SendRequest(context.Background(), MethodInitialize, params)
	if err != nil {
		return nil, fmt.Errorf("initialize request failed: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("initialize failed: %s", response.Error.Message)
	}

	// Parse result using mapToStruct
	var result InitializeResult
	if err := mapToStruct(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse initialize result: %w", err)
	}

	// Send initialized notification
	if err := c.Initialized(); err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	logging.Printf("Client: successfully initialized MCP connection (protocol version: %s)", result.ProtocolVersion)
	return &result, nil
}

// Initialized sends the initialized notification
func (c *client) Initialized() error {
	params := &InitializedParams{}
	return c.pm.SendNotification(MethodInitialized, params)
}

// EnsureInitialized ensures the client is initialized, re-initializing if necessary
func (c *client) EnsureInitialized() error {
	_, err := c.Initialize(nil)
	return err
}

// ToolsList sends a tools/list request
func (c *client) ToolsList(params *ToolsListParams) (*ToolsListResult, error) {
	if err := c.EnsureInitialized(); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	response, err := c.pm.SendRequest(context.Background(), MethodToolsList, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("tools/list failed: %s", response.Error.Message)
	}

	var result ToolsListResult
	if err := mapToStruct(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/list result: %w", err)
	}

	return &result, nil
}

// ToolsCall sends a tools/call request
func (c *client) ToolsCall(params *ToolsCallParams) (*ToolResult, error) {
	if err := c.EnsureInitialized(); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	response, err := c.pm.SendRequest(context.Background(), MethodToolsCall, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("tools/call failed: %s", response.Error.Message)
	}

	var result ToolResult
	if err := mapToStruct(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/call result: %w", err)
	}

	return &result, nil
}

// ResourcesList sends a resources/list request
func (c *client) ResourcesList(params *ResourcesListParams) (*ResourcesListResult, error) {
	if err := c.EnsureInitialized(); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	response, err := c.pm.SendRequest(context.Background(), MethodResourcesList, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("resources/list failed: %s", response.Error.Message)
	}

	var result ResourcesListResult
	if err := mapToStruct(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resources/list result: %w", err)
	}

	return &result, nil
}

// ResourcesRead sends a resources/read request
func (c *client) ResourcesRead(params *ResourceReadParams) (*ResourceContents, error) {
	if err := c.EnsureInitialized(); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	response, err := c.pm.SendRequest(context.Background(), MethodResourcesRead, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("resources/read failed: %s", response.Error.Message)
	}

	var result ResourceContents
	if err := mapToStruct(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resources/read result: %w", err)
	}

	return &result, nil
}

// PromptsList sends a prompts/list request
func (c *client) PromptsList(params *PromptsListParams) (*PromptsListResult, error) {
	if err := c.EnsureInitialized(); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	response, err := c.pm.SendRequest(context.Background(), MethodPromptsList, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("prompts/list failed: %s", response.Error.Message)
	}

	var result PromptsListResult
	if err := mapToStruct(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompts/list result: %w", err)
	}

	return &result, nil
}

// PromptsGet sends a prompts/get request
func (c *client) PromptsGet(params *PromptsGetParams) (*GetPromptResult, error) {
	if err := c.EnsureInitialized(); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	response, err := c.pm.SendRequest(context.Background(), MethodPromptsGet, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("prompts/get failed: %s", response.Error.Message)
	}

	var result GetPromptResult
	if err := mapToStruct(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompts/get result: %w", err)
	}

	return &result, nil
}

// HealthCheck performs a health check on the MCP connection
func (c *client) HealthCheck() error {
	// Check if process manager is running
	if !c.pm.IsRunning() {
		return fmt.Errorf("process is not running")
	}

	// Try to send a lightweight request to verify the connection is working
	// Using tools/list with nil params as a simple health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := c.pm.SendRequest(ctx, MethodToolsList, nil)
	if err != nil {
		// Trigger re-initialization on failure
		c.resetInitState(0)
		return fmt.Errorf("health check failed: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("health check failed: %s", response.Error.Message)
	}

	return nil
}

// IsHealthy returns the current health status of the client
func (c *client) IsHealthy() bool {
	// Check if process is running and initialized
	if !c.pm.IsRunning() {
		return false
	}

	state := atomic.LoadInt32(&c.initState)
	return state == 2 // Initialized state
}

// Close closes the connection
func (c *client) Close() error {
	return c.pm.Stop()
}

// MCPHTTPBridge adapts an MCP client to handle HTTP requests
type MCPHTTPBridge struct {
	client MCPClient
	// Cached tool list to avoid repeated requests
	tools []Tool
	// Store init args for re-initialization
	initArgs []string
}

// NewMCPHTTPBridge creates a new bridge between MCP and HTTP
func NewMCPHTTPBridge(client MCPClient) *MCPHTTPBridge {
	return &MCPHTTPBridge{
		client: client,
	}
}

// Initialize initializes the MCP connection
func (b *MCPHTTPBridge) Initialize(args []string) error {
	// Store args for potential re-initialization
	b.initArgs = args

	params := &InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Experimental: make(map[string]interface{}),
		},
		ClientInfo: ImplementationInfo{
			Name:    "mcproxy",
			Version: "1.0.0",
		},
		Experimental: make(map[string]interface{}),
	}

	// Add args to experimental if provided
	if len(args) > 0 {
		params.Experimental["args"] = args
	}

	_, err := b.client.Initialize(params)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	// Fetch tool list
	toolsResult, err := b.client.ToolsList(nil)
	if err != nil {
		logging.Printf("Warning: Failed to fetch tool list: %v", err)
	} else {
		b.tools = toolsResult.Tools
		logging.Printf("Loaded %d tools from MCP server", len(b.tools))
	}

	return nil
}

// HandleRequest handles an HTTP request and translates it to an MCP call
func (b *MCPHTTPBridge) HandleRequest(method string, params interface{}) (interface{}, error) {
	switch method {
	case MethodToolsList:
		if p, ok := params.(*ToolsListParams); ok {
			return b.client.ToolsList(p)
		}
		return b.client.ToolsList(nil)

	case MethodToolsCall:
		if p, ok := params.(*ToolsCallParams); ok {
			return b.client.ToolsCall(p)
		}
		return nil, fmt.Errorf("invalid params for tools/call")

	case MethodResourcesList:
		if p, ok := params.(*ResourcesListParams); ok {
			return b.client.ResourcesList(p)
		}
		return b.client.ResourcesList(nil)

	case MethodResourcesRead:
		if p, ok := params.(*ResourceReadParams); ok {
			return b.client.ResourcesRead(p)
		}
		return nil, fmt.Errorf("invalid params for resources/read")

	case MethodPromptsList:
		if p, ok := params.(*PromptsListParams); ok {
			return b.client.PromptsList(p)
		}
		return b.client.PromptsList(nil)

	case MethodPromptsGet:
		if p, ok := params.(*PromptsGetParams); ok {
			return b.client.PromptsGet(p)
		}
		return nil, fmt.Errorf("invalid params for prompts/get")

	default:
		return nil, fmt.Errorf("unsupported MCP method: %s", method)
	}
}

// GetTools returns the cached list of tools
func (b *MCPHTTPBridge) GetTools() []Tool {
	return b.tools
}

// Close closes the underlying MCP client
func (b *MCPHTTPBridge) Close() error {
	return b.client.Close()
}

// RequestMapper maps HTTP requests to MCP calls
type RequestMapper struct{}

// MapRequest maps an HTTP request to an MCP method and parameters
func (rm *RequestMapper) MapRequest(method, path string, headers map[string]string, body []byte) (string, interface{}, error) {
	// Check for explicit method in header
	if mcpMethod := headers["X-MCP-Method"]; mcpMethod != "" {
		return rm.parseMethodAndParams(mcpMethod, body)
	}

	// Map based on HTTP method and path
	switch method {
	case "POST":
		switch path {
		case "/tools/list":
			return MethodToolsList, nil, nil
		case "/tools/call":
			var params ToolsCallParams
			if len(body) > 0 {
				if err := json.Unmarshal(body, &params); err != nil {
					return "", nil, fmt.Errorf("failed to parse tools/call params: %w", err)
				}
			}
			return MethodToolsCall, &params, nil
		case "/resources/list":
			return MethodResourcesList, nil, nil
		case "/resources/read":
			var params ResourceReadParams
			if len(body) > 0 {
				if err := json.Unmarshal(body, &params); err != nil {
					return "", nil, fmt.Errorf("failed to parse resources/read params: %w", err)
				}
			}
			return MethodResourcesRead, &params, nil
		case "/prompts/list":
			return MethodPromptsList, nil, nil
		case "/prompts/get":
			var params PromptsGetParams
			if len(body) > 0 {
				if err := json.Unmarshal(body, &params); err != nil {
					return "", nil, fmt.Errorf("failed to parse prompts/get params: %w", err)
				}
			}
			return MethodPromptsGet, &params, nil
		default:
			// Default to treating the body as a generic JSON-RPC request
			var request JSONRPCRequest
			if err := json.Unmarshal(body, &request); err == nil {
				return request.Method, request.Params, nil
			}
			return "", nil, fmt.Errorf("unable to map POST request to MCP method")
		}
	default:
		return "", nil, fmt.Errorf("HTTP method %s not supported for MCP", method)
	}
}

// parseMethodAndParams parses an explicit MCP method from header
func (rm *RequestMapper) parseMethodAndParams(method string, body []byte) (string, interface{}, error) {
	switch method {
	case MethodToolsList:
		if len(body) == 0 {
			return MethodToolsList, nil, nil
		}
		var params ToolsListParams
		if err := json.Unmarshal(body, &params); err != nil {
			return "", nil, fmt.Errorf("failed to parse tools/list params: %w", err)
		}
		return MethodToolsList, &params, nil

	case MethodToolsCall:
		var params ToolsCallParams
		if err := json.Unmarshal(body, &params); err != nil {
			return "", nil, fmt.Errorf("failed to parse tools/call params: %w", err)
		}
		return MethodToolsCall, &params, nil

	case MethodResourcesList:
		if len(body) == 0 {
			return MethodResourcesList, nil, nil
		}
		var params ResourcesListParams
		if err := json.Unmarshal(body, &params); err != nil {
			return "", nil, fmt.Errorf("failed to parse resources/list params: %w", err)
		}
		return MethodResourcesList, &params, nil

	case MethodResourcesRead:
		var params ResourceReadParams
		if err := json.Unmarshal(body, &params); err != nil {
			return "", nil, fmt.Errorf("failed to parse resources/read params: %w", err)
		}
		return MethodResourcesRead, &params, nil

	case MethodPromptsList:
		if len(body) == 0 {
			return MethodPromptsList, nil, nil
		}
		var params PromptsListParams
		if err := json.Unmarshal(body, &params); err != nil {
			return "", nil, fmt.Errorf("failed to parse prompts/list params: %w", err)
		}
		return MethodPromptsList, &params, nil

	case MethodPromptsGet:
		var params PromptsGetParams
		if err := json.Unmarshal(body, &params); err != nil {
			return "", nil, fmt.Errorf("failed to parse prompts/get params: %w", err)
		}
		return MethodPromptsGet, &params, nil

	default:
		// Try to parse as a generic method
		var params json.RawMessage
		if len(body) > 0 {
			params = body
		}
		return method, params, nil
	}
}
