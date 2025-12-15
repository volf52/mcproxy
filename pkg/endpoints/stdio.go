package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mcproxy/pkg/config"
	"mcproxy/pkg/logging"
	"mcproxy/pkg/mcp"
	"mcproxy/pkg/proxy"
)

// StdioEndpoint implements the Endpoint interface for MCP stdio processes
type StdioEndpoint struct {
	name        string
	command     []string
	env         map[string]string
	args        []string
	timeout     time.Duration
	maxBodySize int64

	// MCP components
	processManager *mcp.ProcessManager
	bridge         *mcp.MCPHTTPBridge
	mapper         *mcp.RequestMapper
}

// NewStdioEndpoint creates a new stdio endpoint
func NewStdioEndpoint(name string, cfg config.Endpoint, secrets map[string]string) (Endpoint, error) {
	// Extract StdioEndpoint from the discriminated union
	stdioEndpoint, ok := cfg.Value.(config.StdioEndpoint)
	if !ok {
		return nil, fmt.Errorf("endpoint is not a stdio endpoint")
	}

	// Process secret templates in environment variables
	processedEnv := make(map[string]string)
	for envName, envValue := range stdioEndpoint.Env {
		resolvedValue, missingVars, err := config.SubstituteTemplate(envValue, secrets)
		if err != nil {
			logging.Printf("Error processing environment variable '%s' for endpoint '%s': %v", envName, name, err)
			// Use original value if template processing fails
			resolvedValue = envValue
		}

		if len(missingVars) > 0 {
			logging.Printf("Warning: Environment variable '%s' for endpoint '%s' has missing secret variables: %v", envName, name, missingVars)
		}

		processedEnv[envName] = resolvedValue
	}

	// Set defaults
	timeout := 60 * time.Second
	if stdioEndpoint.Timeout != nil {
		timeout = *stdioEndpoint.Timeout
	}

	maxBodySize := int64(10 * 1024 * 1024) // 10MB
	if stdioEndpoint.MaxBodySize != nil {
		maxBodySize = *stdioEndpoint.MaxBodySize
	}

	// Build command: use secure parsing for the Command field, then append Args
	var command []string
	if stdioEndpoint.Command == "" {
		return nil, fmt.Errorf("command is required for stdio endpoint '%s'", name)
	}

	// Use the secure command validator to parse the command
	validator := config.NewSecureCommandValidator()

	// Allow common executable paths
	allowedPaths := []string{
		"/usr/bin",
		"/usr/local/bin",
		"/bin",
		"/sbin",
		"/usr/sbin",
		"/opt",
		"/usr/local/opt",
	}
	validator.WithAllowedPaths(allowedPaths)

	// Parse the command securely
	parsedCommand, err := validator.ValidateAndParseCommand(stdioEndpoint.Command)
	if err != nil {
		return nil, fmt.Errorf("invalid command for stdio endpoint '%s': %w", name, err)
	}

	// Start with the parsed command
	command = parsedCommand

	// Append any additional arguments from Args field
	if len(stdioEndpoint.Args) > 0 {
		// Validate each argument
		for _, arg := range stdioEndpoint.Args {
			if strings.Contains(arg, "\x00") {
				return nil, fmt.Errorf("argument contains null byte in endpoint '%s'", name)
			}
		}
		command = append(command, stdioEndpoint.Args...)
	}

	// Create process manager
	pm, err := mcp.NewProcessManager(&mcp.ProcessManagerOptions{
		Command: command,
		Env:     processedEnv,
		NotificationHandler: func(notification *mcp.JSONRPCNotification) {
			logging.Printf("MCP notification from endpoint '%s': %s", name, notification.Method)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create process manager: %w", err)
	}

	// Create MCP client and bridge
	mcpClient := mcp.NewMCPClient(pm)
	bridge := mcp.NewMCPHTTPBridge(mcpClient)
	mapper := &mcp.RequestMapper{}

	return &StdioEndpoint{
		name:           name,
		command:        command,
		env:            processedEnv,
		args:           stdioEndpoint.Args,
		timeout:        timeout,
		maxBodySize:    maxBodySize,
		processManager: pm,
		bridge:         bridge,
		mapper:         mapper,
	}, nil
}

// Initialize initializes the MCP connection
func (e *StdioEndpoint) Initialize() error {
	// Start the process
	ctx := context.Background()
	if err := e.processManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start MCP process: %w", err)
	}

	// Initialize the MCP connection
	if err := e.bridge.Initialize(e.args); err != nil {
		e.processManager.Stop()
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	logging.Printf("Successfully initialized MCP stdio endpoint '%s' with command: %v", e.name, e.command)
	return nil
}

// HandleRequest handles an HTTP request and translates it to an MCP call
func (e *StdioEndpoint) HandleRequest(ctx context.Context, r *http.Request) (*Response, error) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		return NewErrorResponse(http.StatusMethodNotAllowed, "Only POST method is allowed"), nil
	}

	// Initialize if not already done
	if !e.processManager.IsRunning() {
		if err := e.Initialize(); err != nil {
			return NewErrorResponse(http.StatusServiceUnavailable, "MCP service unavailable", http.StatusInternalServerError), nil
		}
	}

	// Create context with timeout
	if e.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.timeout)
		defer cancel()
	}

	// Read request body with size limit
	body, err := e.readRequestBody(r)
	if err != nil {
		return e.handleSizeError(err), nil
	}

	// Map HTTP request to MCP method and parameters
	method, params, err := e.mapper.MapRequest(r.Method, r.URL.Path, e.getCombinedHeaders(r), body)
	if err != nil {
		return NewErrorResponse(http.StatusBadRequest, fmt.Sprintf("Failed to map request to MCP call: %v", err)), nil
	}

	// Execute MCP call
	result, err := e.bridge.HandleRequest(method, params)
	if err != nil {
		return NewErrorResponse(http.StatusInternalServerError, fmt.Sprintf("MCP call failed: %v", err)), nil
	}

	// Marshal response
	responseBody, err := json.Marshal(result)
	if err != nil {
		return NewErrorResponse(http.StatusInternalServerError, "Failed to marshal MCP response"), nil
	}

	// Set response headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")

	return &Response{
		StatusCode: http.StatusOK,
		Header:     headers,
		Body:       responseBody,
	}, nil
}

// Close closes the stdio endpoint
func (e *StdioEndpoint) Close() error {
	if e.bridge != nil {
		e.bridge.Close()
	}
	return e.processManager.Stop()
}

// GetPID returns the PID of the managed process, or 0 if no process is running
func (e *StdioEndpoint) GetPID() int {
	return e.processManager.GetPID()
}

// readRequestBody reads the request body with size limit
func (e *StdioEndpoint) readRequestBody(r *http.Request) ([]byte, error) {
	if r.ContentLength > e.maxBodySize {
		return nil, &proxy.RequestTooLargeError{
			Size:     r.ContentLength,
			MaxSize:  e.maxBodySize,
			Endpoint: e.name,
		}
	}

	// Limit the reader to prevent excessive memory usage
	limitedReader := http.MaxBytesReader(nil, r.Body, e.maxBodySize)
	defer r.Body.Close()

	body := make([]byte, r.ContentLength)
	_, err := limitedReader.Read(body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// handleSizeError creates an error response for size limit violations
func (e *StdioEndpoint) handleSizeError(err error) *Response {
	if reqErr, ok := err.(*proxy.RequestTooLargeError); ok {
		logging.Printf("Request too large for endpoint '%s': %v", e.name, err)
		return NewErrorResponse(http.StatusRequestEntityTooLarge,
			fmt.Sprintf("Request body size %d exceeds maximum allowed size %d", reqErr.Size, reqErr.MaxSize))
	}

	return NewErrorResponse(http.StatusInternalServerError, "Internal server error")
}

// getCombinedHeaders combines incoming headers with configured headers
func (e *StdioEndpoint) getCombinedHeaders(r *http.Request) map[string]string {
	combined := make(map[string]string)

	// Add incoming headers
	for key, values := range r.Header {
		if len(values) > 0 {
			combined[key] = values[0] // Take first value
		}
	}

	// Stdio endpoints don't have custom headers to override

	return combined
}
