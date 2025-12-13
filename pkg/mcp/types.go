package mcp

import (
	"encoding/json"
	"fmt"
)

// JSON-RPC 2.0 base types
type JSONRPCVersion string

const JSONRPCVersion20 JSONRPCVersion = "2.0"

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC JSONRPCVersion  `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC JSONRPCVersion  `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// JSONRPCNotification represents a JSON-RPC 2.0 notification (no ID)
type JSONRPCNotification struct {
	JSONRPC JSONRPCVersion  `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// MCP-specific method names
const (
	MethodInitialize         = "initialize"
	MethodInitialized        = "initialized"
	MethodToolsList          = "tools/list"
	MethodToolsCall          = "tools/call"
	MethodResourcesList      = "resources/list"
	MethodResourcesRead      = "resources/read"
	MethodPromptsList        = "prompts/list"
	MethodPromptsGet         = "prompts/get"
	MethodCompletionComplete = "completion/complete"
	MethodSetLevel           = "logging/setLevel"
	MethodGetPrompt          = "logging/getPrompt"
)

// InitializeParams represents parameters for the initialize method
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ClientCapabilities     `json:"capabilities"`
	ClientInfo      ImplementationInfo     `json:"clientInfo"`
	Root            *Root                  `json:"root,omitempty"`
	Trace           string                 `json:"trace,omitempty"`
	Experimental    map[string]interface{} `json:"experimental,omitempty"`
}

// ClientCapabilities represents client capabilities
type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Root         *RootCapability        `json:"root,omitempty"`
	Sampling     *SamplingCapability    `json:"sampling,omitempty"`
}

// RootCapability represents root capability
type RootCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability represents sampling capability
type SamplingCapability struct{}

// Root represents the root URI
type Root struct {
	URI   string                 `json:"uri"`
	Name  string                 `json:"name,omitempty"`
	Lang  string                 `json:"lang,omitempty"`
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// ImplementationInfo represents client or server implementation info
type ImplementationInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult represents the result of the initialize method
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ServerCapabilities     `json:"capabilities"`
	ServerInfo      ImplementationInfo     `json:"serverInfo"`
	Instructions    string                 `json:"instructions,omitempty"`
	Experimental    map[string]interface{} `json:"experimental,omitempty"`
}

// ServerCapabilities represents server capabilities
type ServerCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Logging      *LoggingCapability     `json:"logging,omitempty"`
	Prompts      *PromptsCapability     `json:"prompts,omitempty"`
	Resources    *ResourcesCapability   `json:"resources,omitempty"`
	Tools        *ToolsCapability       `json:"tools,omitempty"`
}

// LoggingCapability represents logging capability
type LoggingCapability struct{}

// PromptsCapability represents prompts capability
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability represents resources capability
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolsCapability represents tools capability
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializedParams represents parameters for the initialized notification
type InitializedParams struct{}

// ToolsListParams represents parameters for tools/list
type ToolsListParams struct {
	Cursor *Cursor `json:"cursor,omitempty"`
}

// Cursor represents pagination cursor
type Cursor struct {
	PageToken string `json:"pageToken,omitempty"`
}

// ToolsListResult represents the result of tools/list
type ToolsListResult struct {
	Tools      []Tool  `json:"tools"`
	NextCursor *Cursor `json:"nextCursor,omitempty"`
}

// Tool represents a tool definition
type Tool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	InputSchema  map[string]interface{} `json:"inputSchema"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// ToolsCallParams represents parameters for tools/call
type ToolsCallParams struct {
	Name            string                 `json:"name"`
	Arguments       map[string]interface{} `json:"arguments,omitempty"`
	ArgumentsRecord map[string]interface{} `json:"argumentsRecord,omitempty"`
	ArgumentsSchema map[string]interface{} `json:"argumentsSchema,omitempty"`
	Experimental    map[string]interface{} `json:"experimental,omitempty"`
	// Legacy field for compatibility
	Arguments__deprecated map[string]interface{} `json:"_arguments,omitempty"`
}

// ToolResult represents a tool call result
type ToolResult struct {
	Content      []Content              `json:"content"`
	IsError      bool                   `json:"isError,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// Content represents text or image content
type Content struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	Data         string                 `json:"data,omitempty"`
	MimeType     string                 `json:"mimeType,omitempty"`
	Annotations  *Annotations           `json:"annotations,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// Annotations represents content annotations
type Annotations struct {
	Audience []string `json:"audience,omitempty"`
	Priority int      `json:"priority,omitempty"`
}

// ResourcesListParams represents parameters for resources/list
type ResourcesListParams struct {
	Cursor *Cursor `json:"cursor,omitempty"`
}

// ResourcesListResult represents the result of resources/list
type ResourcesListResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor *Cursor    `json:"nextCursor,omitempty"`
}

// Resource represents a resource
type Resource struct {
	URI          string                 `json:"uri"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	MimeType     string                 `json:"mimeType,omitempty"`
	Annotations  *Annotations           `json:"annotations,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// ResourceContents represents the contents of a resource
type ResourceContents struct {
	URI         string       `json:"uri"`
	MimeType    string       `json:"mimeType,omitempty"`
	Text        string       `json:"text,omitempty"`
	Blob        string       `json:"blob,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
}

// PromptsListParams represents parameters for prompts/list
type PromptsListParams struct {
	Cursor *Cursor `json:"cursor,omitempty"`
}

// PromptsListResult represents the result of prompts/list
type PromptsListResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor *Cursor  `json:"nextCursor,omitempty"`
}

// Prompt represents a prompt template
type Prompt struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	Arguments    []PromptArgument       `json:"arguments,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// PromptArgument represents a prompt argument
type PromptArgument struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	Required     bool                   `json:"required,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// PromptsGetParams represents parameters for prompts/get
type PromptsGetParams struct {
	Name         string                 `json:"name"`
	Arguments    map[string]interface{} `json:"arguments,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// GetPromptResult represents the result of prompts/get
type GetPromptResult struct {
	Description  string                 `json:"description,omitempty"`
	Messages     []PromptMessage        `json:"messages"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// PromptMessage represents a prompt message
type PromptMessage struct {
	Role         string                 `json:"role"`
	Content      Content                `json:"content"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// SetLevelParams represents parameters for logging/setLevel
type SetLevelParams struct {
	Level string `json:"level"`
}

// LoggingGetPromptParams represents parameters for logging/getPrompt
type LoggingGetPromptParams struct {
	Name         string                 `json:"name"`
	Arguments    map[string]interface{} `json:"arguments,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// LoggingGetPromptResult represents the result of logging/getPrompt
type LoggingGetPromptResult struct {
	Prompt       string                 `json:"prompt"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// ProgressToken represents a progress token
type ProgressToken struct {
	String string `json:"string,omitempty"`
	Number int64  `json:"number,omitempty"`
}

// ProgressNotification represents a progress notification
type ProgressNotification struct {
	ProgressToken ProgressToken          `json:"progressToken"`
	Progress      float64                `json:"progress"`
	Total         float64                `json:"total,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Percentage    float64                `json:"percentage,omitempty"`
	Experimental  map[string]interface{} `json:"experimental,omitempty"`
}

// CompletionParams represents parameters for completion/complete
type CompletionParams struct {
	Ref          *CompletionReference   `json:"ref"`
	Argument     *CompletionArgument    `json:"argument,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// CompletionReference represents a completion reference
type CompletionReference struct {
	Type string `json:"type"`
	URI  string `json:"uri,omitempty"`
	Name string `json:"name,omitempty"`
}

// CompletionArgument represents a completion argument
type CompletionArgument struct {
	Name         string                 `json:"name"`
	Value        string                 `json:"value"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// CompletionResult represents the result of completion/complete
type CompletionResult struct {
	Completion   CompletionItem         `json:"Completion,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// CompletionItem represents a completion item
type CompletionItem struct {
	Values  []CompletionValue `json:"values"`
	Total   int64             `json:"total,omitempty"`
	HasMore bool              `json:"hasMore,omitempty"`
}

// CompletionValue represents a completion value
type CompletionValue struct {
	Value        string                 `json:"value"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// MCPClient represents an MCP client connection
type MCPClient interface {
	// Initialize sends an initialize request and waits for response
	Initialize(params *InitializeParams) (*InitializeResult, error)

	// Initialized sends the initialized notification
	Initialized() error

	// ToolsList sends a tools/list request
	ToolsList(params *ToolsListParams) (*ToolsListResult, error)

	// ToolsCall sends a tools/call request
	ToolsCall(params *ToolsCallParams) (*ToolResult, error)

	// ResourcesList sends a resources/list request
	ResourcesList(params *ResourcesListParams) (*ResourcesListResult, error)

	// ResourcesRead sends a resources/read request
	ResourcesRead(params *ResourceReadParams) (*ResourceContents, error)

	// PromptsList sends a prompts/list request
	PromptsList(params *PromptsListParams) (*PromptsListResult, error)

	// PromptsGet sends a prompts/get request
	PromptsGet(params *PromptsGetParams) (*GetPromptResult, error)

	// HealthCheck performs a health check on the MCP connection
	HealthCheck() error

	// IsHealthy returns the current health status of the client
	IsHealthy() bool

	// Close closes the connection
	Close() error
}

// ResourceReadParams represents parameters for resources/read
type ResourceReadParams struct {
	URI string `json:"uri"`
}

// Helper functions for error handling
func NewJSONRPCError(code int, message string, data interface{}) *JSONRPCError {
	result := &JSONRPCError{
		Code:    code,
		Message: message,
	}
	if data != nil {
		if bytes, err := json.Marshal(data); err == nil {
			result.Data = bytes
		}
	}
	return result
}

func NewParseError(data interface{}) *JSONRPCError {
	return NewJSONRPCError(ParseError, "Parse error", data)
}

func NewInvalidRequestError(data interface{}) *JSONRPCError {
	return NewJSONRPCError(InvalidRequest, "Invalid Request", data)
}

func NewMethodNotFoundError(method string) *JSONRPCError {
	return NewJSONRPCError(MethodNotFound, "Method not found", map[string]string{"method": method})
}

func NewInvalidParamsError(data interface{}) *JSONRPCError {
	return NewJSONRPCError(InvalidParams, "Invalid params", data)
}

func NewInternalError(data interface{}) *JSONRPCError {
	return NewJSONRPCError(InternalError, "Internal error", data)
}

// mapToStruct safely unmarshals JSON-RPC result data to a target struct.
// It handles edge cases like nil data, empty responses, and malformed JSON.
func mapToStruct(data json.RawMessage, target interface{}) error {
	// Handle nil or empty data
	if len(data) == 0 || string(data) == "null" {
		return fmt.Errorf("empty result data")
	}

	// Check if data is valid JSON
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON data: %s", string(data))
	}

	// Unmarshal to target
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return nil
}
