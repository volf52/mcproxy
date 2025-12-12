package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"mcproxy/pkg/logging"
)

// client implements the MCPClient interface
type client struct {
	pm       *ProcessManager
	initOnce sync.Once
	initDone chan struct{}
	initErr  error
}

// NewMCPClient creates a new MCP client using the given process manager
func NewMCPClient(pm *ProcessManager) MCPClient {
	return &client{
		pm:       pm,
		initDone: make(chan struct{}),
	}
}

// Initialize sends an initialize request and waits for response
func (c *client) Initialize(params *InitializeParams) (*InitializeResult, error) {
	var result *InitializeResult

	c.initOnce.Do(func() {
		defer close(c.initDone)

		// Send initialize request
		response, err := c.pm.SendRequest(context.Background(), MethodInitialize, params)
		if err != nil {
			c.initErr = fmt.Errorf("initialize request failed: %w", err)
			return
		}

		if response.Error != nil {
			c.initErr = fmt.Errorf("initialize failed: %s", response.Error.Message)
			return
		}

		// Parse result
		if err := json.Unmarshal(response.Result, &result); err != nil {
			c.initErr = fmt.Errorf("failed to parse initialize result: %w", err)
			return
		}

		c.initErr = nil
	})

	// Wait for initialization to complete
	<-c.initDone
	if c.initErr != nil {
		return nil, c.initErr
	}

	// Send initialized notification
	if err := c.Initialized(); err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return result, nil
}

// Initialized sends the initialized notification
func (c *client) Initialized() error {
	params := &InitializedParams{}
	return c.pm.SendNotification(MethodInitialized, params)
}

// ToolsList sends a tools/list request
func (c *client) ToolsList(params *ToolsListParams) (*ToolsListResult, error) {
	response, err := c.pm.SendRequest(context.Background(), MethodToolsList, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("tools/list failed: %s", response.Error.Message)
	}

	var result ToolsListResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/list result: %w", err)
	}

	return &result, nil
}

// ToolsCall sends a tools/call request
func (c *client) ToolsCall(params *ToolsCallParams) (*ToolResult, error) {
	response, err := c.pm.SendRequest(context.Background(), MethodToolsCall, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("tools/call failed: %s", response.Error.Message)
	}

	var result ToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/call result: %w", err)
	}

	return &result, nil
}

// ResourcesList sends a resources/list request
func (c *client) ResourcesList(params *ResourcesListParams) (*ResourcesListResult, error) {
	response, err := c.pm.SendRequest(context.Background(), MethodResourcesList, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("resources/list failed: %s", response.Error.Message)
	}

	var result ResourcesListResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resources/list result: %w", err)
	}

	return &result, nil
}

// ResourcesRead sends a resources/read request
func (c *client) ResourcesRead(params *ResourceReadParams) (*ResourceContents, error) {
	response, err := c.pm.SendRequest(context.Background(), MethodResourcesRead, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("resources/read failed: %s", response.Error.Message)
	}

	var result ResourceContents
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resources/read result: %w", err)
	}

	return &result, nil
}

// PromptsList sends a prompts/list request
func (c *client) PromptsList(params *PromptsListParams) (*PromptsListResult, error) {
	response, err := c.pm.SendRequest(context.Background(), MethodPromptsList, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("prompts/list failed: %s", response.Error.Message)
	}

	var result PromptsListResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompts/list result: %w", err)
	}

	return &result, nil
}

// PromptsGet sends a prompts/get request
func (c *client) PromptsGet(params *PromptsGetParams) (*GetPromptResult, error) {
	response, err := c.pm.SendRequest(context.Background(), MethodPromptsGet, params)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("prompts/get failed: %s", response.Error.Message)
	}

	var result GetPromptResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompts/get result: %w", err)
	}

	return &result, nil
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
}

// NewMCPHTTPBridge creates a new bridge between MCP and HTTP
func NewMCPHTTPBridge(client MCPClient) *MCPHTTPBridge {
	return &MCPHTTPBridge{
		client: client,
	}
}

// Initialize initializes the MCP connection
func (b *MCPHTTPBridge) Initialize(args []string) error {
	params := &InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Experimental: make(map[string]interface{}),
		},
		ClientInfo: ImplementationInfo{
			Name:    "mcproxy",
			Version: "1.0.0",
		},
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
