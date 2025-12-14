package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

// Server represents a test MCP server
type Server struct {
	initialized bool
	mu          sync.Mutex
	crashAfter  int
	slowMode    bool
	malformed   bool
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// InitializeParams represents initialize parameters
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]interface{} `json:"clientInfo"`
}

// InitializeResult represents initialize result
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      map[string]interface{} `json:"serverInfo"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolsListResult represents tools/list result
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams represents tools/call parameters
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolResult represents tool call result
type ToolResult struct {
	Content []map[string]interface{} `json:"content"`
	IsError bool                     `json:"isError,omitempty"`
}

// NewServer creates a new test MCP server
func NewServer() *Server {
	return &Server{}
}

// handleRequest processes a JSON-RPC request
func (s *Server) handleRequest(req *JSONRPCRequest) *JSONRPCResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for crash condition
	if s.crashAfter > 0 {
		s.crashAfter--
		if s.crashAfter == 0 {
			log.Fatal("Intentional crash for testing")
		}
	}

	// Simulate slow response
	if s.slowMode {
		time.Sleep(100 * time.Millisecond)
	}

	// Malformed response mode
	if s.malformed {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"invalid": json}`),
		}
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req, req.Params)
	case "tools/list":
		return s.handleToolsList(req, req.Params)
	case "tools/call":
		return s.handleToolsCall(req, req.Params)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

// handleInitialize handles the initialize method
func (s *Server) handleInitialize(req *JSONRPCRequest, params json.RawMessage) *JSONRPCResponse {
	if s.initialized {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32000,
				Message: "Already initialized",
			},
		}
	}

	var initParams InitializeParams
	if err := json.Unmarshal(params, &initParams); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    err.Error(),
			},
		}
	}

	s.initialized = true

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		ServerInfo: map[string]interface{}{
			"name":    "Test MCP Server",
			"version": "1.0.0",
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolsList handles the tools/list method
func (s *Server) handleToolsList(req *JSONRPCRequest, params json.RawMessage) *JSONRPCResponse {
	if !s.initialized {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32000,
				Message: "Not initialized",
			},
		}
	}

	tools := []Tool{
		{
			Name:        "echo",
			Description: "Echoes back the input arguments",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "The message to echo back",
					},
				},
				"required": []string{"message"},
			},
		},
		{
			Name:        "add",
			Description: "Adds two numbers together",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]interface{}{
						"type":        "number",
						"description": "First number",
					},
					"b": map[string]interface{}{
						"type":        "number",
						"description": "Second number",
					},
				},
				"required": []string{"a", "b"},
			},
		},
		{
			Name:        "large_payload",
			Description: "Handles large payloads efficiently",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type":        "string",
						"description": "Large data payload",
					},
				},
			},
		},
	}

	result := ToolsListResult{
		Tools: tools,
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolsCall handles the tools/call method
func (s *Server) handleToolsCall(req *JSONRPCRequest, params json.RawMessage) *JSONRPCResponse {
	if !s.initialized {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32000,
				Message: "Not initialized",
			},
		}
	}

	var callParams ToolCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    err.Error(),
			},
		}
	}

	var result ToolResult

	switch callParams.Name {
	case "echo":
		message, ok := callParams.Arguments["message"].(string)
		if !ok {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32602,
					Message: "Missing or invalid message parameter",
				},
			}
		}
		result = ToolResult{
			Content: []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("Echo: %s", message),
				},
			},
		}

	case "add":
		a, aOk := callParams.Arguments["a"].(float64)
		b, bOk := callParams.Arguments["b"].(float64)
		if !aOk || !bOk {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32602,
					Message: "Missing or invalid number parameters",
				},
			}
		}
		sum := a + b
		result = ToolResult{
			Content: []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("%.2f + %.2f = %.2f", a, b, sum),
				},
			},
		}

	case "large_payload":
		data, ok := callParams.Arguments["data"].(string)
		if !ok {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32602,
					Message: "Missing or invalid data parameter",
				},
			}
		}
		size := len(data)
		result = ToolResult{
			Content: []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("Received payload of %d bytes", size),
				},
			},
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: fmt.Sprintf("Unknown tool: %s", callParams.Name),
			},
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// Run starts the server and handles JSON-RPC communication
func (s *Server) Run() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req JSONRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Failed to decode request: %v", err)
			continue
		}

		resp := s.handleRequest(&req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Failed to encode response: %v", err)
			break
		}
	}
}

// parseFlags parses command line flags
func parseFlags() (crashAfter int, slowMode bool, malformed bool) {
	flag.IntVar(&crashAfter, "crash-after", 0, "Crash after N requests")
	flag.BoolVar(&slowMode, "slow", false, "Respond slowly (100ms delay)")
	flag.BoolVar(&malformed, "malformed", false, "Return malformed responses")
	flag.Parse()
	return
}

// generateRandomID generates a random request ID for testing
func generateRandomID() string {
	return strconv.Itoa(rand.Intn(1000000))
}

// Main function for the test server
func Main() {
	crashAfter, slowMode, malformed := parseFlags()

	server := NewServer()
	server.crashAfter = crashAfter
	server.slowMode = slowMode
	server.malformed = malformed

	// Seed random generator for any random behavior
	rand.Seed(time.Now().UnixNano())

	server.Run()
}
