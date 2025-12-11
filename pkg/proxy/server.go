package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mcproxy/pkg/config"
	"mcproxy/pkg/logging"
)

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

// RequestTooLargeError is returned when request body exceeds size limit
type RequestTooLargeError struct {
	Size     int64
	MaxSize  int64
	Endpoint string
}

func (e *RequestTooLargeError) Error() string {
	return fmt.Sprintf("request body %d bytes exceeds limit of %d bytes for endpoint %s",
		e.Size, e.MaxSize, e.Endpoint)
}

// filterHopByHopHeaders removes hop-by-hop headers from the request
func filterHopByHopHeaders(header http.Header) http.Header {
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
			logging.Debugf("Filtering hop-by-hop header: %s -> %t (exists in hopByHop: %t, in connection: %t)",
				key, hopByHopHeaders[keyLower], hopByHopHeaders[keyLower], connectionMap[keyLower])
			continue
		}
		for _, value := range values {
			filtered.Add(key, value)
		}
	}

	// The above should have filtered out all hop-by-hop headers including Proxy-Connection

	return filtered
}

// Server represents the proxy server
type Server struct {
	port       string
	endpoints  map[string]config.Endpoint
	httpClient *http.Client
}

// NewServer creates a new proxy server instance
func NewServer(port string, endpoints map[string]config.Endpoint) *Server {
	return &Server{
		port:       port,
		endpoints:  endpoints,
		httpClient: createHTTPClient(),
	}
}

// Start starts the proxy server and registers all endpoints
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register handlers for each endpoint
	for name, endpoint := range s.endpoints {
		handler := s.createProxyHandler(name, endpoint)
		pattern := fmt.Sprintf("/mcp/%s", name)
		mux.HandleFunc(pattern, handler)
		logging.LogEndpointRegistration(name, endpoint.Url, "/mcp", endpoint.Headers)
	}

	// Add a root handler for basic info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"service": "mcproxy", "status": "running"}`)
	})

	logging.Printf("Starting mcproxy server on port %s", s.port)
	return http.ListenAndServe(s.port, mux)
}

// createProxyHandler creates an HTTP handler for a specific endpoint
func (s *Server) createProxyHandler(name string, endpoint config.Endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Create context with timeout
		ctx := r.Context()
		timeout := s.getEndpointTimeout(endpoint)
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		// Check request size against limits
		if err := s.checkRequestSize(r, endpoint); err != nil {
			s.handleRequestError(err, name, endpoint.Url, w)
			return
		}

		// Get max body size for streaming
		maxSize := s.getEndpointMaxBodySize(endpoint)

		// Create the upstream request with streaming
		upstreamReq, err := s.createUpstreamRequest(ctx, r, endpoint, maxSize)
		if err != nil {
			s.handleRequestError(err, name, endpoint.Url, w)
			return
		}

		// Execute the upstream request
		resp, err := s.httpClient.Do(upstreamReq)
		if err != nil {
			s.handleRequestError(err, name, endpoint.Url, w)
			return
		}
		defer resp.Body.Close()

		// Stream response back to client
		s.streamResponse(w, resp)
	}
}

// createUpstreamRequest creates a new HTTP request to forward to the upstream URL
func (s *Server) createUpstreamRequest(ctx context.Context, r *http.Request, endpoint config.Endpoint, maxSize int64) (*http.Request, error) {
	// Apply size limit to request body
	var bodyReader io.Reader = r.Body
	if maxSize > 0 {
		bodyReader = io.LimitReader(r.Body, maxSize)
	}

	// Preserve Content-Length if present
	contentLength := r.ContentLength
	if maxSize > 0 && contentLength > maxSize {
		contentLength = maxSize
	}

	// Create request with context and streaming body
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.Url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	// Set Content-Length if we have it
	if contentLength >= 0 {
		upstreamReq.ContentLength = contentLength
	}

	// Copy filtered headers from the incoming request
	// Important: We must filter BEFORE setting on the request to avoid adding hop-by-hop headers
	filteredHeaders := filterHopByHopHeaders(r.Header)
	upstreamReq.Header = filteredHeaders

	// Add/override configured headers
	for headerName, headerValue := range endpoint.Headers {
		upstreamReq.Header.Set(headerName, headerValue)
		// Log header setting without exposing actual values
		logging.DebugfSanitized("Setting header for upstream request: %s", headerName)
	}

	// Set Content-Type if not present
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/octet-stream")
	}

	// Set proper Host header based on upstream URL
	if parsedURL, err := url.Parse(endpoint.Url); err == nil {
		upstreamReq.Host = parsedURL.Host
	}

	return upstreamReq, nil
}

// getEndpointTimeout returns the timeout for an endpoint
func (s *Server) getEndpointTimeout(endpoint config.Endpoint) time.Duration {
	if endpoint.Timeout != nil && *endpoint.Timeout > 0 {
		return *endpoint.Timeout
	}
	// Return default timeout
	return 60 * time.Second
}

// getEndpointMaxBodySize returns the max body size for an endpoint
func (s *Server) getEndpointMaxBodySize(endpoint config.Endpoint) int64 {
	if endpoint.MaxBodySize != nil && *endpoint.MaxBodySize > 0 {
		return *endpoint.MaxBodySize
	}
	// Return default max size (10MB)
	return 10 * 1024 * 1024
}

// checkRequestSize validates request size against limits
func (s *Server) checkRequestSize(r *http.Request, endpoint config.Endpoint) error {
	maxSize := s.getEndpointMaxBodySize(endpoint)

	// Check Content-Length header if present
	if contentLength := r.ContentLength; contentLength > 0 {
		if contentLength > maxSize {
			return &RequestTooLargeError{
				Size:     contentLength,
				MaxSize:  maxSize,
				Endpoint: endpoint.Url,
			}
		}
	}

	return nil
}

// handleRequestError handles upstream request errors
func (s *Server) handleRequestError(err error, endpointName, upstreamURL string, w http.ResponseWriter) {
	if errors.Is(err, context.DeadlineExceeded) {
		logging.Printf("Request timeout for endpoint '%s'", endpointName)
		http.Error(w, "Request timeout", http.StatusGatewayTimeout)
	} else if errors.Is(err, context.Canceled) {
		logging.Printf("Request canceled for endpoint '%s'", endpointName)
		http.Error(w, "Request canceled", http.StatusRequestTimeout)
	} else if _, ok := err.(*RequestTooLargeError); ok {
		logging.Printf("Request too large for endpoint '%s': %v", endpointName, err)
		http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
	} else {
		logging.Printf("Error forwarding request for endpoint '%s': %v", endpointName, err)
		logging.Debugf("Request forwarding error: endpoint='%s', upstream='%s', error=%v",
			endpointName, upstreamURL, err)
		http.Error(w, "Bad gateway", http.StatusBadGateway)
	}
}

// streamResponse streams the upstream response back to the client
func (s *Server) streamResponse(w http.ResponseWriter, resp *http.Response) {
	// Copy response headers (filtering hop-by-hop)
	for key, values := range resp.Header {
		keyLower := strings.ToLower(key)
		if !hopByHopHeaders[keyLower] {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// Stream response body
	_, err := io.Copy(w, resp.Body)
	if err != nil {
		logging.Printf("Error streaming response body: %v", err)
		// Don't write to response here as headers are already sent
	}
}
