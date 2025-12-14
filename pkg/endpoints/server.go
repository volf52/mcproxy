package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"mcproxy/pkg/config"
	"mcproxy/pkg/logging"
	"mcproxy/pkg/version"
)

// Server represents the proxy server using the endpoint abstraction
type Server struct {
	port           string
	endpointMap    map[string]Endpoint
	serverConfig   config.ServerConfig
	server         *http.Server
	serverMu       sync.RWMutex // Protects access to the server field
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

// IsShuttingDown returns whether the server is currently shutting down
func (s *Server) IsShuttingDown() bool {
	return s.isShuttingDown.get()
}

// NewServer creates a new proxy server instance with endpoint factory
func NewServer(port string, endpoints map[string]config.Endpoint, secrets config.Secrets, serverConfig config.ServerConfig) (*Server, error) {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	// Create endpoint factory
	factory := NewDefaultFactory()

	// Create endpoints and track successful initializations
	endpointMap := make(map[string]Endpoint)
	initErrors := make(map[string]error)

	for name, cfg := range endpoints {
		endpoint, err := factory.CreateEndpoint(name, cfg, secrets)
		if err != nil {
			initErrors[name] = fmt.Errorf("failed to create endpoint '%s': %w", name, err)
			continue
		}

		// Initialize stdio endpoints
		if stdioEp, ok := endpoint.(*StdioEndpoint); ok {
			if err := stdioEp.Initialize(); err != nil {
				initErrors[name] = fmt.Errorf("failed to initialize stdio endpoint '%s': %w", name, err)
				endpoint.Close() // Clean up resources
				continue
			}
		}

		endpointMap[name] = endpoint
	}

	// Require at least one successful endpoint
	if len(endpointMap) == 0 {
		// Cancel context first
		shutdownCancel()

		var errorMsg strings.Builder
		errorMsg.WriteString("no endpoints could be initialized:\n")
		for name, err := range initErrors {
			errorMsg.WriteString(fmt.Sprintf("  - %s: %v\n", name, err))
		}
		return nil, fmt.Errorf("%s", errorMsg.String())
	}

	// Log any initialization failures
	for _, err := range initErrors {
		logging.Printf("Warning: %v", err)
	}

	return &Server{
		port:           port,
		endpointMap:    endpointMap,
		serverConfig:   serverConfig,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		isShuttingDown: atomicBool{},
	}, nil
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

	// Register all handlers
	s.registerHandlers(mux)

	logging.Printf("Starting mcproxy server on port %s", s.port)
	return http.ListenAndServe(s.port, mux)
}

// registerHandlers registers all HTTP handlers on the provided mux
func (s *Server) registerHandlers(mux *http.ServeMux) {
	// Register handlers for each endpoint (with and without trailing slash)
	for name := range s.endpointMap {
		handler := s.createProxyHandler(name)
		pattern1 := fmt.Sprintf("/mcp/%s", name)
		pattern2 := fmt.Sprintf("/mcp/%s/", name)
		mux.HandleFunc(pattern1, handler)
		mux.HandleFunc(pattern2, handler)
		logging.Printf("Registered endpoint handler: /mcp/%s", name)
	}

	// Add a root handler for basic info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Collect endpoint types
		endpointTypes := make(map[string]int)
		for _, endpoint := range s.endpointMap {
			switch endpoint.(type) {
			case *HTTPEndpoint:
				endpointTypes["http"]++
			case *StdioEndpoint:
				endpointTypes["stdio"]++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		response := fmt.Sprintf(`{
        "service": "mcproxy",
        "version": "%s",
        "status": "running",
        "endpoints": {
            "total": %d,
            "types": %v
        }
    }`, version.Version, len(s.endpointMap), endpointTypes)

		io.WriteString(w, response)
	})

	// Add health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type EndpointStatus struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}

		endpoints := make(map[string]EndpointStatus)
		for name, endpoint := range s.endpointMap {
			status := EndpointStatus{}

			switch ep := endpoint.(type) {
			case *HTTPEndpoint:
				status.Type = "http"
				status.Status = "healthy" // HTTP endpoints don't have persistent state
			case *StdioEndpoint:
				status.Type = "stdio"
				if ep.processManager != nil && ep.processManager.IsRunning() {
					status.Status = "healthy"
				} else {
					status.Status = "unhealthy"
					status.Error = "Process not running"
				}
			}

			endpoints[name] = status
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"endpoints": endpoints,
		})
	})
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

	// Register all handlers
	s.registerHandlers(mux)

	// Apply middleware to all handlers
	handler := shutdownMiddleware(mux)

	// Create server with timeouts
	s.serverMu.Lock()
	s.server = &http.Server{
		Addr:         s.port,
		Handler:      handler,
		ReadTimeout:  time.Duration(s.serverConfig.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.serverConfig.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(s.serverConfig.IdleTimeout) * time.Second,
	}
	s.serverMu.Unlock()

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

		// Safely access the server field
		s.serverMu.RLock()
		httpServer := s.server
		s.serverMu.RUnlock()

		if httpServer != nil {
			// Shutdown the server
			if shutdownErr := httpServer.Shutdown(shutdownCtx); shutdownErr != nil {
				logging.Printf("Error during server shutdown: %v", shutdownErr)
				err = shutdownErr
			}
		}

		// Close all endpoints
		for name, endpoint := range s.endpointMap {
			if closeErr := endpoint.Close(); closeErr != nil {
				logging.Printf("Error closing endpoint '%s': %v", name, closeErr)
				if err == nil {
					err = closeErr
				}
			}
		}

		// Wait for all in-flight requests to complete
		done := make(chan struct{})
		go func() {
			s.shutdownWG.Wait()
			close(done)
		}()

		select {
		case <-done:
			logging.Printf("All in-flight requests completed")
		case <-shutdownCtx.Done():
			logging.Printf("Shutdown timeout reached, forcing exit")
			if err == nil {
				err = shutdownCtx.Err()
			}
		}

		logging.Printf("Server shutdown complete")
	})

	return err
}

// createProxyHandler creates an HTTP handler for a specific endpoint using the endpoint abstraction
func (s *Server) createProxyHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the endpoint
		endpoint, exists := s.endpointMap[name]
		if !exists {
			http.Error(w, "Endpoint not found", http.StatusNotFound)
			return
		}

		// Check if server is shutting down
		select {
		case <-s.shutdownCtx.Done():
			http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
			return
		default:
		}

		// Handle the request using the endpoint
		response, err := endpoint.HandleRequest(r.Context(), r)
		if err != nil {
			logging.Printf("Error handling request for endpoint '%s': %v", name, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Copy response headers
		for key, values := range response.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Set status code and write body
		w.WriteHeader(response.StatusCode)
		if response.Body != nil {
			w.Write(response.Body)
		}
	}
}
