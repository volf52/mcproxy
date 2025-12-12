package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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
	port           string
	endpoints      map[string]config.Endpoint
	serverConfig   config.ServerConfig
	httpClient     *http.Client
	server         *http.Server
	shutdownWG     sync.WaitGroup
	shutdownOnce   sync.Once
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	isShuttingDown atomicBool
}

// atomicBool provides atomic boolean operations
type atomicBool struct {
	value int32
}

func (b *atomicBool) set(value bool) {
	var i int32
	if value {
		i = 1
	}
	atomic.StoreInt32(&b.value, i)
}

func (b *atomicBool) get() bool {
	return atomic.LoadInt32(&b.value) != 0
}

// NewServer creates a new proxy server instance
func NewServer(port string, endpoints map[string]config.Endpoint, serverConfig config.ServerConfig) *Server {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	return &Server{
		port:           port,
		endpoints:      endpoints,
		serverConfig:   serverConfig,
		httpClient:     createHTTPClient(),
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		isShuttingDown: atomicBool{},
	}
}

// StartWithShutdown starts the proxy server and handles graceful shutdown
func (s *Server) StartWithShutdown() error {
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	s.shutdownWG.Add(1)
	go func() {
		defer s.shutdownWG.Done()
		logging.Printf("Starting mcproxy server on port %s", s.port)
		errChan <- s.listenAndServe()
	}()

	// Wait for either error or signal
	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		logging.Printf("Received signal %v, initiating graceful shutdown", sig)
		return s.gracefulShutdown()
	}
}

// Start starts the proxy server without shutdown handling (backward compatibility)
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

// listenAndServe creates and starts the HTTP server with configured timeouts
func (s *Server) listenAndServe() error {
	mux := http.NewServeMux()

	// Create shutdown check middleware
	shutdownMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if server is shutting down
			select {
			case <-s.shutdownCtx.Done():
				http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
				return
			default:
				// Track in-flight requests for graceful shutdown
				s.shutdownWG.Add(1)
				defer s.shutdownWG.Done()
				next.ServeHTTP(w, r)
			}
		})
	}

	// Apply middleware to all handlers
	handler := shutdownMiddleware(mux)

	// Register endpoint handlers
	for name, endpoint := range s.endpoints {
		handlerFunc := s.createProxyHandler(name, endpoint)
		pattern := fmt.Sprintf("/mcp/%s", name)
		mux.HandleFunc(pattern, handlerFunc)
		logging.LogEndpointRegistration(name, endpoint.Url, "/mcp", endpoint.Headers)
	}

	// Add root handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"service": "mcproxy", "status": "running"}`)
	})

	// Create server with timeouts
	s.server = &http.Server{
		Addr:         s.port,
		Handler:      handler,
		ReadTimeout:  time.Duration(s.serverConfig.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.serverConfig.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(s.serverConfig.IdleTimeout) * time.Second,
	}

	return s.server.ListenAndServe()
}

// gracefulShutdown performs a graceful shutdown of the server
func (s *Server) gracefulShutdown() error {
	var err error
	s.shutdownOnce.Do(func() {
		// Set shutting down flag
		s.isShuttingDown.set(true)

		// Cancel the shutdown context to signal handlers
		s.shutdownCancel()

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(),
			time.Duration(s.serverConfig.ShutdownTimeout)*time.Second)
		defer cancel()

		logging.Printf("Shutting down server gracefully (timeout: %v)",
			time.Duration(s.serverConfig.ShutdownTimeout)*time.Second)

		if s.server != nil {
			// Shutdown the server
			if shutdownErr := s.server.Shutdown(shutdownCtx); shutdownErr != nil {
				logging.Printf("Error during server shutdown: %v", shutdownErr)
				err = shutdownErr
			}
		}

		// Wait for all handlers to complete
		done := make(chan struct{})
		go func() {
			s.shutdownWG.Wait()
			close(done)
		}()

		// Wait for either completion or timeout
		select {
		case <-done:
			logging.Printf("Server shutdown complete")
		case <-shutdownCtx.Done():
			logging.Printf("Server shutdown timeout, forcing exit")
			if s.server != nil {
				// Force close remaining connections
				s.server.Close()
			}
		}
	})

	return err
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

		// Check if server is shutting down
		select {
		case <-s.shutdownCtx.Done():
			http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
			return
		default:
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
