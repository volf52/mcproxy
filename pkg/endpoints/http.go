package endpoints

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mcproxy/pkg/config"
	"mcproxy/pkg/logging"
	"mcproxy/pkg/proxy"
)

// HTTPEndpoint implements the Endpoint interface for HTTP upstream targets
type HTTPEndpoint struct {
	name        string
	url         string
	headers     map[string]string
	timeout     time.Duration
	maxBodySize int64
	httpClient  *http.Client
}

// NewHTTPEndpoint creates a new HTTP endpoint
func NewHTTPEndpoint(name string, cfg config.Endpoint, secrets map[string]string) (Endpoint, error) {
	// Extract HttpEndpoint from the discriminated union
	httpEndpoint, ok := cfg.Value.(config.HttpEndpoint)
	if !ok {
		return nil, fmt.Errorf("endpoint is not an HTTP endpoint")
	}

	// Process secret templates in headers
	processedHeaders := make(map[string]string)
	for headerName, headerValue := range httpEndpoint.Headers {
		resolvedValue, missingVars, err := config.SubstituteTemplate(headerValue, secrets)
		if err != nil {
			logging.Printf("Error processing header '%s' for endpoint '%s': %v", headerName, name, err)
			// Use original value if template processing fails
			resolvedValue = headerValue
		}

		if len(missingVars) > 0 {
			logging.Printf("Warning: Header '%s' for endpoint '%s' has missing secret variables: %v", headerName, name, missingVars)
		}

		processedHeaders[headerName] = resolvedValue
	}

	// Set defaults
	timeout := 60 * time.Second
	if httpEndpoint.Timeout != nil {
		timeout = *httpEndpoint.Timeout
	}

	maxBodySize := int64(10 * 1024 * 1024) // 10MB
	if httpEndpoint.MaxBodySize != nil {
		maxBodySize = *httpEndpoint.MaxBodySize
	}

	return &HTTPEndpoint{
		name:        name,
		url:         httpEndpoint.Url,
		headers:     processedHeaders,
		timeout:     timeout,
		maxBodySize: maxBodySize,
		httpClient:  proxy.CreateHTTPClient(),
	}, nil
}

// HandleRequest handles an HTTP request and forwards it to the upstream HTTP server
func (e *HTTPEndpoint) HandleRequest(ctx context.Context, r *http.Request) (*Response, error) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		return NewErrorResponse(http.StatusMethodNotAllowed, "Only POST method is allowed"), nil
	}

	// Create context with timeout
	if e.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.timeout)
		defer cancel()
	}

	// Check request size against limits
	if err := e.checkRequestSize(r); err != nil {
		return e.handleSizeError(err), nil
	}

	// Create the upstream request
	upstreamReq, err := e.createUpstreamRequest(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	// Execute the upstream request
	resp, err := e.httpClient.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Filter response headers
	filteredHeaders := e.filterHopByHopHeaders(resp.Header)

	return &Response{
		StatusCode: resp.StatusCode,
		Header:     filteredHeaders,
		Body:       body,
	}, nil
}

// Close closes the HTTP endpoint
func (e *HTTPEndpoint) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}

// checkRequestSize validates request size against limits
func (e *HTTPEndpoint) checkRequestSize(r *http.Request) error {
	// Check Content-Length header if present
	if contentLength := r.ContentLength; contentLength > 0 {
		if contentLength > e.maxBodySize {
			return &proxy.RequestTooLargeError{
				Size:     contentLength,
				MaxSize:  e.maxBodySize,
				Endpoint: e.url,
			}
		}
	}

	return nil
}

// handleSizeError creates an error response for size limit violations
func (e *HTTPEndpoint) handleSizeError(err error) *Response {
	if reqErr, ok := err.(*proxy.RequestTooLargeError); ok {
		logging.Printf("Request too large for endpoint '%s': %v", e.name, err)
		return NewErrorResponse(http.StatusRequestEntityTooLarge,
			fmt.Sprintf("Request body size %d exceeds maximum allowed size %d", reqErr.Size, reqErr.MaxSize))
	}

	return NewErrorResponse(http.StatusInternalServerError, "Internal server error")
}

// createUpstreamRequest creates a new HTTP request to forward to the upstream URL
func (e *HTTPEndpoint) createUpstreamRequest(ctx context.Context, r *http.Request) (*http.Request, error) {
	// Apply size limit to request body
	var bodyReader io.Reader = r.Body
	if e.maxBodySize > 0 {
		bodyReader = io.LimitReader(r.Body, e.maxBodySize)
	}

	// Preserve Content-Length if present
	contentLength := r.ContentLength
	if e.maxBodySize > 0 && contentLength > e.maxBodySize {
		contentLength = e.maxBodySize
	}

	// Create request with context and streaming body
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	// Set Content-Length if we have it
	if contentLength >= 0 {
		upstreamReq.ContentLength = contentLength
	}

	// Copy filtered headers from the incoming request
	filteredHeaders := e.filterHopByHopHeaders(r.Header)
	upstreamReq.Header = filteredHeaders

	// Add/override configured headers
	for headerName, headerValue := range e.headers {
		upstreamReq.Header.Set(headerName, headerValue)
		// Log header setting without exposing actual values
		logging.DebugfSanitized("Setting header for upstream request: %s", headerName)
	}

	// Set Content-Type if not present
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/octet-stream")
	}

	// Set proper Host header based on upstream URL
	if parsedURL, err := url.Parse(e.url); err == nil {
		upstreamReq.Host = parsedURL.Host
	}

	return upstreamReq, nil
}

// hopByHopHeaders contains headers that should not be forwarded per RFC 2616
var hopByHopHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailers":            true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"proxy-connection":    true,
}

// filterHopByHopHeaders removes hop-by-hop headers from the request/response
func (e *HTTPEndpoint) filterHopByHopHeaders(header http.Header) http.Header {
	filtered := make(http.Header)

	// Check Connection header for additional hop-by-hop headers
	connectionHeaders := strings.Split(header.Get("Connection"), ",")
	connectionMap := make(map[string]bool)
	for _, h := range connectionHeaders {
		headerName := strings.TrimSpace(strings.ToLower(h))
		if headerName != "" {
			connectionMap[headerName] = true
		}
	}

	for key, values := range header {
		keyLower := strings.ToLower(key)
		// Skip hop-by-hop headers and those listed in Connection header
		if hopByHopHeaders[keyLower] || connectionMap[keyLower] {
			logging.Debugf("Filtering hop-by-hop header: %s", key)
			continue
		}
		for _, value := range values {
			filtered.Add(key, value)
		}
	}

	return filtered
}
