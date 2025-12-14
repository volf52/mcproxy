package mcp

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestJSONRPCRequestMarshalling tests marshalling and unmarshalling of JSON-RPC requests
func TestJSONRPCRequestMarshalling(t *testing.T) {
	tests := []struct {
		name        string
		request     JSONRPCRequest
		expectError bool
	}{
		{
			name: "valid request with params",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion20,
				Method:  "initialize",
				Params:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
				ID:      "req-1",
			},
			expectError: false,
		},
		{
			name: "valid request without params",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion20,
				Method:  "tools/list",
				ID:      123,
			},
			expectError: false,
		},
		{
			name: "notification (no ID)",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion20,
				Method:  "initialized",
				Params:  json.RawMessage(`{}`),
			},
			expectError: false,
		},
		{
			name: "request with null params",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion20,
				Method:  "tools/call",
				Params:  json.RawMessage(`null`),
				ID:      "req-2",
			},
			expectError: false,
		},
		{
			name: "request with empty string method",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion20,
				Method:  "",
				ID:      "req-3",
			},
			expectError: false, // Validation happens at higher level
		},
		{
			name: "request with integer ID zero",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion20,
				Method:  "initialize",
				ID:      0,
			},
			expectError: false,
		},
		{
			name: "request with float ID",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion20,
				Method:  "tools/list",
				ID:      3.14,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshalling
			data, err := json.Marshal(tt.request)
			if err != nil && !tt.expectError {
				t.Errorf("Unexpected marshal error: %v", err)
			} else if err == nil && tt.expectError {
				t.Error("Expected marshal error but got none")
			}

			if err != nil {
				return
			}

			// Test unmarshalling
			var unmarshaled JSONRPCRequest
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Errorf("Unmarshal error: %v", err)
				return
			}

			// Verify basic fields
			if unmarshaled.JSONRPC != tt.request.JSONRPC {
				t.Errorf("Expected JSONRPC version %s, got %s", tt.request.JSONRPC, unmarshaled.JSONRPC)
			}
			if unmarshaled.Method != tt.request.Method {
				t.Errorf("Expected method %s, got %s", tt.request.Method, unmarshaled.Method)
			}

			// Compare IDs (JSON numbers might change type)
			if !jsonIDsEqual(unmarshaled.ID, tt.request.ID) {
				t.Errorf("Expected ID %v, got %v", tt.request.ID, unmarshaled.ID)
			}
		})
	}
}

// TestJSONRPCResponseMarshalling tests marshalling and unmarshalling of JSON-RPC responses
func TestJSONRPCResponseMarshalling(t *testing.T) {
	tests := []struct {
		name        string
		response    JSONRPCResponse
		expectError bool
	}{
		{
			name: "success response",
			response: JSONRPCResponse{
				JSONRPC: JSONRPCVersion20,
				Result:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
				ID:      "resp-1",
			},
			expectError: false,
		},
		{
			name: "error response",
			response: JSONRPCResponse{
				JSONRPC: JSONRPCVersion20,
				Error: &JSONRPCError{
					Code:    -32601,
					Message: "Method not found",
				},
				ID: "resp-2",
			},
			expectError: false,
		},
		{
			name: "response with error data",
			response: JSONRPCResponse{
				JSONRPC: JSONRPCVersion20,
				Error: &JSONRPCError{
					Code:    -32602,
					Message: "Invalid params",
					Data:    json.RawMessage(`{"field":"name","issue":"required"}`),
				},
				ID: 123,
			},
			expectError: false,
		},
		{
			name: "response with null result",
			response: JSONRPCResponse{
				JSONRPC: JSONRPCVersion20,
				Result:  json.RawMessage(`null`),
				ID:      nil,
			},
			expectError: false,
		},
		{
			name: "response with neither result nor error",
			response: JSONRPCResponse{
				JSONRPC: JSONRPCVersion20,
				ID:      "resp-3",
			},
			expectError: false,
		},
		{
			name: "response with both result and error (invalid but testable)",
			response: JSONRPCResponse{
				JSONRPC: JSONRPCVersion20,
				Result:  json.RawMessage(`{}`),
				Error: &JSONRPCError{
					Code:    -32600,
					Message: "Invalid response",
				},
				ID: "resp-4",
			},
			expectError: false, // JSON marshaling won't catch this
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshalling
			data, err := json.Marshal(tt.response)
			if err != nil && !tt.expectError {
				t.Errorf("Unexpected marshal error: %v", err)
			} else if err == nil && tt.expectError {
				t.Error("Expected marshal error but got none")
			}

			if err != nil {
				return
			}

			// Test unmarshalling
			var unmarshaled JSONRPCResponse
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Errorf("Unmarshal error: %v", err)
				return
			}

			// Verify basic fields
			if unmarshaled.JSONRPC != tt.response.JSONRPC {
				t.Errorf("Expected JSONRPC version %s, got %s", tt.response.JSONRPC, unmarshaled.JSONRPC)
			}

			// Compare IDs
			if !jsonIDsEqual(unmarshaled.ID, tt.response.ID) {
				t.Errorf("Expected ID %v, got %v", tt.response.ID, unmarshaled.ID)
			}

			// Check result/error fields
			if (tt.response.Result != nil) != (unmarshaled.Result != nil) {
				t.Error("Result presence mismatch")
			}
			if (tt.response.Error != nil) != (unmarshaled.Error != nil) {
				t.Error("Error presence mismatch")
			}
		})
	}
}

// TestMCPInitializeTypes tests marshalling of MCP initialize types
func TestMCPInitializeTypes(t *testing.T) {
	// Test InitializeParams
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Experimental: map[string]interface{}{
				"feature1": true,
				"feature2": map[string]interface{}{
					"subfeature": "value",
				},
			},
			Root: &RootCapability{
				ListChanged: true,
			},
		},
		ClientInfo: ImplementationInfo{
			Name:    "Test Client",
			Version: "1.0.0",
		},
		Root: &Root{
			URI:  "file:///test/path",
			Name: "Test Project",
			Lang: "en",
			Extra: map[string]interface{}{
				"custom": "data",
			},
		},
		Trace: "debug",
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal InitializeParams: %v", err)
	}

	var unmarshaledParams InitializeParams
	err = json.Unmarshal(data, &unmarshaledParams)
	if err != nil {
		t.Fatalf("Failed to unmarshal InitializeParams: %v", err)
	}

	if unmarshaledParams.ProtocolVersion != params.ProtocolVersion {
		t.Errorf("Expected protocol version %s, got %s", params.ProtocolVersion, unmarshaledParams.ProtocolVersion)
	}

	// Test InitializeResult
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Experimental: map[string]interface{}{
				"serverFeature": true,
			},
			Logging: &LoggingCapability{},
			Tools: &ToolsCapability{
				ListChanged: true,
			},
		},
		ServerInfo: ImplementationInfo{
			Name:    "Test Server",
			Version: "1.0.0",
		},
		Instructions: "Test instructions",
	}

	data, err = json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal InitializeResult: %v", err)
	}

	var unmarshaledResult InitializeResult
	err = json.Unmarshal(data, &unmarshaledResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal InitializeResult: %v", err)
	}

	if unmarshaledResult.ProtocolVersion != result.ProtocolVersion {
		t.Errorf("Expected protocol version %s, got %s", result.ProtocolVersion, unmarshaledResult.ProtocolVersion)
	}
	if unmarshaledResult.Instructions != result.Instructions {
		t.Errorf("Expected instructions %s, got %s", result.Instructions, unmarshaledResult.Instructions)
	}
}

// TestMCPToolsTypes tests marshalling of MCP tools types
func TestMCPToolsTypes(t *testing.T) {
	// Test ToolsListParams
	listParams := ToolsListParams{
		Cursor: &Cursor{
			PageToken: "next-page-token",
		},
	}

	data, err := json.Marshal(listParams)
	if err != nil {
		t.Fatalf("Failed to marshal ToolsListParams: %v", err)
	}

	var unmarshaledListParams ToolsListParams
	err = json.Unmarshal(data, &unmarshaledListParams)
	if err != nil {
		t.Fatalf("Failed to unmarshal ToolsListParams: %v", err)
	}

	if unmarshaledListParams.Cursor.PageToken != listParams.Cursor.PageToken {
		t.Errorf("Expected page token %s, got %s", listParams.Cursor.PageToken, unmarshaledListParams.Cursor.PageToken)
	}

	// Test Tool
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool for testing",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input to process",
				},
			},
			"required": []string{"input"},
		},
		Experimental: map[string]interface{}{
			"experimentalFeature": true,
		},
	}

	data, err = json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal Tool: %v", err)
	}

	var unmarshaledTool Tool
	err = json.Unmarshal(data, &unmarshaledTool)
	if err != nil {
		t.Fatalf("Failed to unmarshal Tool: %v", err)
	}

	if unmarshaledTool.Name != tool.Name {
		t.Errorf("Expected tool name %s, got %s", tool.Name, unmarshaledTool.Name)
	}

	// Test ToolsListResult
	listResult := ToolsListResult{
		Tools: []Tool{tool},
		NextCursor: &Cursor{
			PageToken: "next-page-token-2",
		},
	}

	data, err = json.Marshal(listResult)
	if err != nil {
		t.Fatalf("Failed to marshal ToolsListResult: %v", err)
	}

	var unmarshaledListResult ToolsListResult
	err = json.Unmarshal(data, &unmarshaledListResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal ToolsListResult: %v", err)
	}

	if len(unmarshaledListResult.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(unmarshaledListResult.Tools))
	}

	if unmarshaledListResult.Tools[0].Name != tool.Name {
		t.Errorf("Expected tool name %s, got %s", tool.Name, unmarshaledListResult.Tools[0].Name)
	}
}

// TestMCPToolsCallTypes tests marshalling of tools/call types
func TestMCPToolsCallTypes(t *testing.T) {
	// Test ToolsCallParams
	callParams := ToolsCallParams{
		Name: "test_tool",
		Arguments: map[string]interface{}{
			"input": "test input",
			"count": 42,
			"flag":  true,
			"list":  []string{"a", "b", "c"},
		},
		ArgumentsRecord: map[string]interface{}{
			"recordType": "test",
		},
		ArgumentsSchema: map[string]interface{}{
			"schema": "test",
		},
		Experimental: map[string]interface{}{
			"experimental": true,
		},
	}

	data, err := json.Marshal(callParams)
	if err != nil {
		t.Fatalf("Failed to marshal ToolsCallParams: %v", err)
	}

	var unmarshaledCallParams ToolsCallParams
	err = json.Unmarshal(data, &unmarshaledCallParams)
	if err != nil {
		t.Fatalf("Failed to unmarshal ToolsCallParams: %v", err)
	}

	if unmarshaledCallParams.Name != callParams.Name {
		t.Errorf("Expected tool name %s, got %s", callParams.Name, unmarshaledCallParams.Name)
	}

	// Test Content
	content := Content{
		Type: "text",
		Text: "Test response content",
		Annotations: &Annotations{
			Audience: []string{"user", "system"},
			Priority: 1,
		},
		Experimental: map[string]interface{}{
			"contentFeature": true,
		},
	}

	imageContent := Content{
		Type:     "image",
		Data:     "base64-encoded-image-data",
		MimeType: "image/png",
	}

	// Test ToolResult
	result := ToolResult{
		Content: []Content{content, imageContent},
		IsError: false,
		Experimental: map[string]interface{}{
			"resultFeature": true,
		},
	}

	data, err = json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal ToolResult: %v", err)
	}

	var unmarshaledResult ToolResult
	err = json.Unmarshal(data, &unmarshaledResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal ToolResult: %v", err)
	}

	if len(unmarshaledResult.Content) != 2 {
		t.Fatalf("Expected 2 content items, got %d", len(unmarshaledResult.Content))
	}

	if unmarshaledResult.Content[0].Type != "text" {
		t.Errorf("Expected first content type 'text', got '%s'", unmarshaledResult.Content[0].Type)
	}

	if unmarshaledResult.Content[1].Type != "image" {
		t.Errorf("Expected second content type 'image', got '%s'", unmarshaledResult.Content[1].Type)
	}
}

// TestMCPEdgeCases tests edge cases and error conditions
func TestMCPEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
	}{
		{
			name:        "valid empty object",
			jsonData:    `{}`,
			expectError: false,
		},
		{
			name:        "valid null",
			jsonData:    `null`,
			expectError: false,
		},
		{
			name:        "invalid JSON",
			jsonData:    `{invalid json}`,
			expectError: true,
		},
		{
			name:        "empty array",
			jsonData:    `[]`,
			expectError: true, // Empty array is not a valid JSON-RPC request
		},
		{
			name:        "non-object in params",
			jsonData:    `{"params": "string instead of object"}`,
			expectError: false, // json.RawMessage can hold anything
		},
		{
			name:        "invalid JSON-RPC version",
			jsonData:    `{"jsonrpc": "1.0", "method": "test", "id": 1}`,
			expectError: false, // Validation happens at higher level
		},
		{
			name:        "missing method",
			jsonData:    `{"jsonrpc": "2.0", "id": 1}`,
			expectError: false, // json.Unmarshal will use zero value
		},
		{
			name:        "extra fields",
			jsonData:    `{"jsonrpc": "2.0", "method": "test", "id": 1, "extra": "field"}`,
			expectError: false, // Extra fields are ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request JSONRPCRequest
			err := json.Unmarshal([]byte(tt.jsonData), &request)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestMCPUnicodeHandling tests Unicode and special character handling
func TestMCPUnicodeHandling(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "ASCII text",
			value: "Hello, World!",
		},
		{
			name:  "Unicode characters",
			value: "Hello, 世界! 🌍",
		},
		{
			name:  "Emoji",
			value: "🚀 MCP Server 🤖",
		},
		{
			name:  "Special characters",
			value: "Special: \n\t\r\\\"'",
		},
		{
			name:  "JSON escaped characters",
			value: "JSON: \b\f\n\r\t",
		},
		{
			name:  "Unicode surrogate pairs",
			value: "\U0001F600", // Grinning face emoji
		},
		{
			name:  "Zero-width characters",
			value: "Invisible\u200BText",
		},
		{
			name:  "Control characters",
			value: string([]byte{0x00, 0x01, 0x02}), // Should be escaped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test in text content
			content := Content{
				Type: "text",
				Text: tt.value,
			}

			data, err := json.Marshal(content)
			if err != nil {
				t.Errorf("Marshal error: %v", err)
				return
			}

			var unmarshaled Content
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Errorf("Unmarshal error: %v", err)
				return
			}

			if unmarshaled.Text != tt.value {
				t.Errorf("Expected text '%s', got '%s'", tt.value, unmarshaled.Text)
			}

			// Test in tool name
			tool := Tool{
				Name:        tt.value,
				Description: "Test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			}

			data, err = json.Marshal(tool)
			if err != nil {
				t.Errorf("Tool marshal error: %v", err)
				return
			}

			var unmarshaledTool Tool
			err = json.Unmarshal(data, &unmarshaledTool)
			if err != nil {
				t.Errorf("Tool unmarshal error: %v", err)
				return
			}

			if unmarshaledTool.Name != tt.value {
				t.Errorf("Expected tool name '%s', got '%s'", tt.value, unmarshaledTool.Name)
			}
		})
	}
}

// TestMCPLargePayloads tests handling of large payloads
func TestMCPLargePayloads(t *testing.T) {
	// Generate a large text content (approximately 1MB)
	largeText := make([]byte, 1024*1024)
	for i := range largeText {
		largeText[i] = byte('A' + (i % 26))
	}

	content := Content{
		Type: "text",
		Text: string(largeText),
	}

	// Test marshaling large content
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal large content: %v", err)
	}

	// Verify it's reasonably large
	if len(data) < 1024*1020 {
		t.Errorf("Expected marshaled data to be at least 1MB, got %d bytes", len(data))
	}

	// Test unmarshaling large content
	var unmarshaled Content
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal large content: %v", err)
	}

	if len(unmarshaled.Text) != len(largeText) {
		t.Errorf("Expected text length %d, got %d", len(largeText), len(unmarshaled.Text))
	}

	// Test with very large array of tools
	tools := make([]Tool, 10000)
	for i := range tools {
		tools[i] = Tool{
			Name:        "tool_" + string(rune(i)),
			Description: "Description for tool " + string(rune(i)),
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"index": map[string]interface{}{
						"type": "integer",
					},
				},
			},
		}
	}

	listResult := ToolsListResult{
		Tools: tools,
	}

	data, err = json.Marshal(listResult)
	if err != nil {
		t.Fatalf("Failed to marshal large tools list: %v", err)
	}

	// Should be at least 1MB
	if len(data) < 1*1024*1024 {
		t.Errorf("Expected large tools list to be at least 1MB, got %d bytes", len(data))
	}

	var unmarshaledList ToolsListResult
	err = json.Unmarshal(data, &unmarshaledList)
	if err != nil {
		t.Fatalf("Failed to unmarshal large tools list: %v", err)
	}

	if len(unmarshaledList.Tools) != 10000 {
		t.Errorf("Expected 10000 tools, got %d", len(unmarshaledList.Tools))
	}
}

// TestMCPNumericValues tests handling of various numeric values
func TestMCPNumericValues(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		valid bool
	}{
		{
			name:  "zero",
			value: 0,
			valid: true,
		},
		{
			name:  "positive integer",
			value: 42,
			valid: true,
		},
		{
			name:  "negative integer",
			value: -42,
			valid: true,
		},
		{
			name:  "large integer",
			value: 9223372036854775807, // Max int64
			valid: true,
		},
		{
			name:  "float",
			value: 3.14159,
			valid: true,
		},
		{
			name:  "negative float",
			value: -2.71828,
			valid: true,
		},
		{
			name:  "scientific notation",
			value: 1.23e-4,
			valid: true,
		},
		{
			name:  "infinity",
			value: json.Number("Inf"),
			valid: false, // JSON doesn't support Infinity
		},
		{
			name:  "NaN",
			value: json.Number("NaN"),
			valid: false, // JSON doesn't support NaN
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := JSONRPCRequest{
				JSONRPC: JSONRPCVersion20,
				Method:  "test",
				ID:      tt.value,
			}

			data, err := json.Marshal(request)
			if err != nil && tt.valid {
				t.Errorf("Unexpected marshal error: %v", err)
			}
			if err == nil && !tt.valid {
				t.Error("Expected marshal error but got none")
			}

			if err != nil {
				return
			}

			var unmarshaled JSONRPCRequest
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Errorf("Unmarshal error: %v", err)
				return
			}

			// IDs might change type (e.g., int to float64)
			if unmarshaled.ID == nil && tt.value != nil {
				t.Error("Expected non-nil ID")
			}
		})
	}
}

// TestMCPMapToStruct tests the mapToStruct helper function
func TestMCPMapToStruct(t *testing.T) {
	tests := []struct {
		name        string
		data        json.RawMessage
		target      interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid object to struct",
			data: json.RawMessage(`{"name":"test","value":42}`),
			target: &struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{},
			expectError: false,
		},
		{
			name:        "valid object to map",
			data:        json.RawMessage(`{"key":"value","number":123}`),
			target:      &map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "valid array to slice",
			data:        json.RawMessage(`[1,2,3]`),
			target:      &[]int{},
			expectError: false,
		},
		{
			name:        "empty data",
			data:        json.RawMessage([]byte{}),
			target:      &map[string]interface{}{},
			expectError: true,
			errorMsg:    "empty result data",
		},
		{
			name:        "null data",
			data:        json.RawMessage(`null`),
			target:      &map[string]interface{}{},
			expectError: true,
			errorMsg:    "empty result data",
		},
		{
			name:        "invalid JSON",
			data:        json.RawMessage(`{invalid json}`),
			target:      &map[string]interface{}{},
			expectError: true,
			errorMsg:    "invalid JSON data",
		},
		{
			name:        "mismatched types",
			data:        json.RawMessage(`{"name":"test"}`),
			target:      &[]string{},
			expectError: true,
			errorMsg:    "failed to unmarshal result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapToStruct(tt.data, tt.target)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					// Check if error contains expected message (not exact match for some cases)
					if !contains(err.Error(), tt.errorMsg) {
						t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestMCPErrorCreation tests the error creation helper functions
func TestMCPErrorCreation(t *testing.T) {
	tests := []struct {
		name  string
		error *JSONRPCError
		code  int
		msg   string
		data  interface{}
	}{
		{
			name:  "parse error",
			error: NewParseError("unexpected token"),
			code:  ParseError,
			msg:   "Parse error",
		},
		{
			name:  "invalid request error",
			error: NewInvalidRequestError(map[string]string{"field": "id"}),
			code:  InvalidRequest,
			msg:   "Invalid Request",
		},
		{
			name:  "method not found error",
			error: NewMethodNotFoundError("unknown_method"),
			code:  MethodNotFound,
			msg:   "Method not found",
		},
		{
			name:  "invalid params error",
			error: NewInvalidParamsError("missing field 'name'"),
			code:  InvalidParams,
			msg:   "Invalid params",
		},
		{
			name:  "internal error",
			error: NewInternalError("database connection failed"),
			code:  InternalError,
			msg:   "Internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.error.Code != tt.code {
				t.Errorf("Expected error code %d, got %d", tt.code, tt.error.Code)
			}
			if tt.error.Message != tt.msg {
				t.Errorf("Expected error message '%s', got '%s'", tt.msg, tt.error.Message)
			}
			if tt.error.Data == nil && tt.data != nil {
				t.Error("Expected error data but got nil")
			}
			if tt.error.Data != nil && len(tt.error.Data) == 0 {
				t.Errorf("Expected non-empty error data, got %v", tt.error.Data)
			}
		})
	}
}

// Helper functions for testing
func jsonIDsEqual(a, b interface{}) bool {
	// Convert both to JSON and compare
	aBytes, _ := json.Marshal(a)
	bBytes, _ := json.Marshal(b)
	return bytes.Equal(aBytes, bBytes)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
