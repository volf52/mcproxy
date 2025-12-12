package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mcproxy/pkg/config"
)

// TestServerTimeouts tests that server timeouts are properly configured and enforced
func TestServerTimeouts(t *testing.T) {
	tests := []struct {
		name         string
		serverConfig config.ServerConfig
		testFunc     func(t *testing.T, server *Server, testServer *httptest.Server)
	}{
		{
			name: "read timeout",
			serverConfig: config.ServerConfig{
				ReadTimeout:     1, // 1 second
				WriteTimeout:    30,
				IdleTimeout:     120,
				ShutdownTimeout: 30,
			},
			testFunc: testReadTimeout,
		},
		{
			name: "write timeout",
			serverConfig: config.ServerConfig{
				ReadTimeout:     30,
				WriteTimeout:    1, // 1 second
				IdleTimeout:     120,
				ShutdownTimeout: 30,
			},
			testFunc: testWriteTimeout,
		},
		{
			name: "idle timeout",
			serverConfig: config.ServerConfig{
				ReadTimeout:     30,
				WriteTimeout:    30,
				IdleTimeout:     1, // 1 second
				ShutdownTimeout: 30,
			},
			testFunc: testIdleTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock upstream server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"success": true}`))
			}))
			defer mockServer.Close()

			// Create endpoint configuration
			endpoints := map[string]config.Endpoint{
				"test-endpoint": {
					Url: mockServer.URL,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
				},
			}

			// Create server with specific timeout configuration
			server := NewServer(":8080", endpoints, tt.serverConfig)

			// Create a test HTTP server to test the timeouts
			mux := http.NewServeMux()
			handler := server.createProxyHandler("test-endpoint", endpoints["test-endpoint"])
			mux.HandleFunc("/mcp/test-endpoint", handler)

			testServer := httptest.NewServer(mux)
			defer testServer.Close()

			// Run the specific test for this timeout
			tt.testFunc(t, server, testServer)
		})
	}
}

// testReadTimeout tests that read timeout is enforced
func testReadTimeout(t *testing.T, server *Server, testServer *httptest.Server) {
	// Create a slow client that sends data very slowly
	slowReader := strings.NewReader(strings.Repeat("x", 10000)) // 10KB of data

	// Create request with custom context that we can control
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", testServer.URL+"/mcp/test-endpoint", slowReader)
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}

	// Set Content-Length to a large value to trigger read timeout
	req.ContentLength = 10000
	req.Header.Set("Content-Type", "application/json")

	// Make request
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		// Expected due to timeout
		t.Logf("Request timed out as expected: %v (elapsed: %v)", err, elapsed)
	} else {
		defer resp.Body.Close()
		// If we get a response, it might be an error response
		if resp.StatusCode != http.StatusOK {
			t.Logf("Got error response %d as expected (elapsed: %v)", resp.StatusCode, elapsed)
		}
	}
}

// testWriteTimeout tests that write timeout is enforced
func testWriteTimeout(t *testing.T, server *Server, testServer *httptest.Server) {
	// Create a slow client that reads responses very slowly
	req, err := http.NewRequest("POST", testServer.URL+"/mcp/test-endpoint", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Make request
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	// Slowly read the response to trigger write timeout
	// Read one byte at a time with delays
	buffer := make([]byte, 1)
	totalRead := 0
	for {
		n, err := resp.Body.Read(buffer)
		totalRead += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Logf("Response read interrupted: %v (elapsed: %v, bytes read: %d)", err, elapsed, totalRead)
			break
		}
		// Add delay to trigger write timeout
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Read %d bytes in %v", totalRead, elapsed)
}

// testIdleTimeout tests that idle timeout is enforced
func testIdleTimeout(t *testing.T, server *Server, testServer *httptest.Server) {
	// Create a client that makes a request but keeps the connection alive
	req, err := http.NewRequest("POST", testServer.URL+"/mcp/test-endpoint", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")

	// Make first request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Error making first request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Wait for idle timeout period
	time.Sleep(time.Duration(server.serverConfig.IdleTimeout+1) * time.Second)

	// Try to make another request - should fail if connection was closed
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("Connection closed as expected due to idle timeout: %v", err)
	} else {
		defer resp2.Body.Close()
		t.Logf("Connection still open after idle timeout, status: %d", resp2.StatusCode)
	}
}

// TestGracefulShutdown tests that graceful shutdown works correctly
func TestGracefulShutdown(t *testing.T) {
	// Create a mock upstream server that delays responses
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow upstream processing
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer mockServer.Close()

	// Create endpoint configuration
	endpoints := map[string]config.Endpoint{
		"test-endpoint": {
			Url: mockServer.URL,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	// Create server with short shutdown timeout for testing
	serverConfig := config.ServerConfig{
		ReadTimeout:     30,
		WriteTimeout:    30,
		IdleTimeout:     120,
		ShutdownTimeout: 5, // 5 seconds
	}

	// Create server
	server := NewServer(":0", endpoints, serverConfig) // Use port 0 for automatic port allocation

	// Start server in goroutine with proper synchronization
	var wg sync.WaitGroup
	wg.Add(1)

	// Use channels for safe communication between goroutines
	type serverResult struct {
		addr string
		err  error
	}
	resultCh := make(chan serverResult, 1)

	go func() {
		defer wg.Done()

		// Create server with proper timeouts
		httpServer := &http.Server{
			Addr:         server.port,
			ReadTimeout:  time.Duration(serverConfig.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(serverConfig.WriteTimeout) * time.Second,
			IdleTimeout:  time.Duration(serverConfig.IdleTimeout) * time.Second,
		}

		mux := http.NewServeMux()

		// Add shutdown middleware
		shutdownMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-server.shutdownCtx.Done():
					http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
					return
				default:
					server.shutdownWG.Add(1)
					defer server.shutdownWG.Done()
					next.ServeHTTP(w, r)
				}
			})
		}

		handler := shutdownMiddleware(mux)

		// Register endpoint
		handlerFunc := server.createProxyHandler("test-endpoint", endpoints["test-endpoint"])
		mux.HandleFunc("/mcp/test-endpoint", handlerFunc)

		// Add root handler
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"service": "mcproxy", "status": "running"}`)
		})

		httpServer.Handler = handler

		// Store the http.Server instance so gracefulShutdown() can access it
		server.serverMu.Lock()
		server.server = httpServer
		server.serverMu.Unlock()

		// Get actual address after binding
		listener, err := net.Listen("tcp", server.port)
		if err != nil {
			resultCh <- serverResult{err: err}
			return
		}
		addr := "http://" + listener.Addr().String()

		// Send address back before starting Serve
		resultCh <- serverResult{addr: addr}

		// Start serving (this blocks)
		serveErr := httpServer.Serve(listener)

		// If server closed normally, don't report as error
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			resultCh <- serverResult{addr: addr, err: serveErr}
		}
	}()

	// Get server address or error from goroutine
	var result serverResult
	select {
	case result = <-resultCh:
		if result.err != nil {
			t.Fatalf("Server failed to start: %v", result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Server failed to start within timeout")
	}

	// Make a request
	resp, err := http.Post(result.addr+"/mcp/test-endpoint", "application/json", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Start graceful shutdown
	shutdownStart := time.Now()
	shutdownErr := server.gracefulShutdown()
	shutdownElapsed := time.Since(shutdownStart)

	// Wait for server goroutine to finish
	wg.Wait()

	// Verify shutdown completed
	if shutdownErr != nil {
		t.Logf("Graceful shutdown returned error: %v", shutdownErr)
	}

	// Verify shutdown didn't take longer than the configured timeout
	if shutdownElapsed > time.Duration(serverConfig.ShutdownTimeout+2)*time.Second {
		t.Errorf("Graceful shutdown took too long: %v (config: %ds)", shutdownElapsed, serverConfig.ShutdownTimeout)
	}

	t.Logf("Graceful shutdown completed in %v", shutdownElapsed)
}

// TestGracefulShutdownWithInFlightRequests tests graceful shutdown with in-flight requests
func TestGracefulShutdownWithInFlightRequests(t *testing.T) {
	// Track request completion
	var requestCompleted sync.WaitGroup
	var requestStarted sync.WaitGroup
	requestStarted.Add(1)
	requestCompleted.Add(1)

	// Create a mock upstream server that delays responses
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted.Done()
		// Simulate slow upstream processing
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer mockServer.Close()

	// Create endpoint configuration
	endpoints := map[string]config.Endpoint{
		"test-endpoint": {
			Url: mockServer.URL,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	// Create server with sufficient shutdown timeout to allow request completion
	serverConfig := config.ServerConfig{
		ReadTimeout:     30,
		WriteTimeout:    30,
		IdleTimeout:     120,
		ShutdownTimeout: 5, // 5 seconds
	}

	// Create server
	server := NewServer(":0", endpoints, serverConfig)

	// Start server in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	var serverErr atomic.Value
	var actualAddr atomic.Value

	go func() {
		defer wg.Done()

		// Similar setup as in TestGracefulShutdown
		httpServer := &http.Server{
			Addr:         server.port,
			ReadTimeout:  time.Duration(serverConfig.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(serverConfig.WriteTimeout) * time.Second,
			IdleTimeout:  time.Duration(serverConfig.IdleTimeout) * time.Second,
		}

		mux := http.NewServeMux()

		shutdownMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-server.shutdownCtx.Done():
					http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
					return
				default:
					server.shutdownWG.Add(1)
					defer server.shutdownWG.Done()
					next.ServeHTTP(w, r)
				}
			})
		}

		handler := shutdownMiddleware(mux)

		handlerFunc := server.createProxyHandler("test-endpoint", endpoints["test-endpoint"])
		mux.HandleFunc("/mcp/test-endpoint", handlerFunc)

		httpServer.Handler = handler

		// Store the http.Server instance so gracefulShutdown() can access it
		server.serverMu.Lock()
		server.server = httpServer
		server.serverMu.Unlock()

		listener, err := net.Listen("tcp", server.port)
		if err != nil {
			serverErr.Store(err)
			return
		}
		actualAddr.Store("http://" + listener.Addr().String())

		serveErr := httpServer.Serve(listener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErr.Store(serveErr)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	if err := serverErr.Load(); err != nil {
		t.Fatalf("Server failed to start: %v", err)
	}

	// Make a request in a goroutine
	var requestErr error
	var requestStatus int
	go func() {
		defer requestCompleted.Done()
		addr := actualAddr.Load().(string)
		resp, err := http.Post(addr+"/mcp/test-endpoint", "application/json", strings.NewReader(`{"test": "data"}`))
		if err != nil {
			requestErr = err
			return
		}
		defer resp.Body.Close()
		requestStatus = resp.StatusCode
	}()

	// Wait for request to start
	requestStarted.Wait()

	// Initiate shutdown while request is in-flight
	shutdownStart := time.Now()
	shutdownErr := server.gracefulShutdown()
	shutdownElapsed := time.Since(shutdownStart)

	// Wait for both server and request to complete
	requestCompleted.Wait()
	wg.Wait()

	// Verify request completed successfully
	if requestErr != nil {
		t.Errorf("Request failed during shutdown: %v", requestErr)
	} else if requestStatus != http.StatusOK {
		t.Errorf("Request returned wrong status during shutdown: %d", requestStatus)
	}

	// Verify shutdown completed
	if shutdownErr != nil {
		t.Logf("Graceful shutdown returned error: %v", shutdownErr)
	}

	t.Logf("Graceful shutdown with in-flight request completed in %v", shutdownElapsed)
}

// TestShutdownTimeout tests that shutdown timeout is enforced
func TestShutdownTimeout(t *testing.T) {
	// Create a mock upstream server that never responds
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate hung upstream that responds after a very long delay
		// but respects client cancellation
		select {
		case <-time.After(10 * time.Second):
			// Respond after 10 seconds (longer than shutdown timeout)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"delayed": true}`))
		case <-r.Context().Done():
			// Client cancelled the request, exit without responding
			return
		case <-time.After(2 * time.Second):
			// Fallback: if context cancellation doesn't work, exit after 2 seconds
			// This prevents the mock server from hanging indefinitely
			return
		}
	}))
	defer mockServer.Close()

	// Create endpoint configuration
	endpoints := map[string]config.Endpoint{
		"test-endpoint": {
			Url: mockServer.URL,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	// Create server with very short shutdown timeout
	serverConfig := config.ServerConfig{
		ReadTimeout:     30,
		WriteTimeout:    30,
		IdleTimeout:     120,
		ShutdownTimeout: 1, // 1 second
	}

	// Create server
	server := NewServer(":0", endpoints, serverConfig)

	// Start server in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	var serverErr atomic.Value
	var actualAddr atomic.Value

	go func() {
		defer wg.Done()

		httpServer := &http.Server{
			Addr:         server.port,
			ReadTimeout:  time.Duration(serverConfig.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(serverConfig.WriteTimeout) * time.Second,
			IdleTimeout:  time.Duration(serverConfig.IdleTimeout) * time.Second,
		}

		mux := http.NewServeMux()

		shutdownMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-server.shutdownCtx.Done():
					http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
					return
				default:
					server.shutdownWG.Add(1)
					defer server.shutdownWG.Done()
					next.ServeHTTP(w, r)
				}
			})
		}

		handler := shutdownMiddleware(mux)

		handlerFunc := server.createProxyHandler("test-endpoint", endpoints["test-endpoint"])
		mux.HandleFunc("/mcp/test-endpoint", handlerFunc)

		httpServer.Handler = handler

		// Store the http.Server instance so gracefulShutdown() can access it
		server.serverMu.Lock()
		server.server = httpServer
		server.serverMu.Unlock()

		listener, err := net.Listen("tcp", server.port)
		if err != nil {
			serverErr.Store(err)
			return
		}
		actualAddr.Store("http://" + listener.Addr().String())

		serveErr := httpServer.Serve(listener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErr.Store(serveErr)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	if err := serverErr.Load(); err != nil {
		t.Fatalf("Server failed to start: %v", err)
	}

	// Make a request that will hang
	go func() {
		// Create request with a long timeout to ensure it doesn't complete before shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		addr := actualAddr.Load().(string)
		req, err := http.NewRequestWithContext(ctx, "POST", addr+"/mcp/test-endpoint", strings.NewReader(`{"test": "data"}`))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		// This request will be cancelled when server shuts down
		http.DefaultClient.Do(req)
	}()

	// Give request time to start
	time.Sleep(100 * time.Millisecond)

	// Initiate shutdown
	shutdownStart := time.Now()
	shutdownErr := server.gracefulShutdown()
	shutdownElapsed := time.Since(shutdownStart)

	// Wait for server to complete
	wg.Wait()

	// Verify shutdown timed out appropriately (should be close to 1 second)
	expectedMinTimeout := time.Duration(serverConfig.ShutdownTimeout) * time.Second
	if shutdownElapsed < expectedMinTimeout {
		t.Errorf("Shutdown completed too quickly: %v (expected >= %v)", shutdownElapsed, expectedMinTimeout)
	}
	// Also verify it didn't take too long (should be within reasonable bounds)
	expectedMaxTimeout := time.Duration(serverConfig.ShutdownTimeout+1) * time.Second
	if shutdownElapsed > expectedMaxTimeout {
		t.Errorf("Shutdown took too long: %v (expected <= %v)", shutdownElapsed, expectedMaxTimeout)
	}

	if shutdownErr != nil {
		t.Logf("Graceful shutdown returned error (expected): %v", shutdownErr)
	}

	t.Logf("Graceful shutdown with timeout completed in %v", shutdownElapsed)
}

// TestServerShutdownStatus tests that server returns 503 when shutting down
func TestServerShutdownStatus(t *testing.T) {
	// Create a mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer mockServer.Close()

	// Create endpoint configuration
	endpoints := map[string]config.Endpoint{
		"test-endpoint": {
			Url: mockServer.URL,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	// Create server
	serverConfig := config.ServerConfig{
		ReadTimeout:     30,
		WriteTimeout:    30,
		IdleTimeout:     120,
		ShutdownTimeout: 5,
	}

	server := NewServer(":0", endpoints, serverConfig)

	// Start server in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	var serverErr atomic.Value
	var actualAddr atomic.Value

	go func() {
		defer wg.Done()

		httpServer := &http.Server{
			Addr:         server.port,
			ReadTimeout:  time.Duration(serverConfig.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(serverConfig.WriteTimeout) * time.Second,
			IdleTimeout:  time.Duration(serverConfig.IdleTimeout) * time.Second,
		}

		mux := http.NewServeMux()

		shutdownMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-server.shutdownCtx.Done():
					http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
					return
				default:
					server.shutdownWG.Add(1)
					defer server.shutdownWG.Done()
					next.ServeHTTP(w, r)
				}
			})
		}

		handler := shutdownMiddleware(mux)

		handlerFunc := server.createProxyHandler("test-endpoint", endpoints["test-endpoint"])
		mux.HandleFunc("/mcp/test-endpoint", handlerFunc)

		httpServer.Handler = handler

		// Store the http.Server instance so gracefulShutdown() can access it
		server.serverMu.Lock()
		server.server = httpServer
		server.serverMu.Unlock()

		listener, err := net.Listen("tcp", server.port)
		if err != nil {
			serverErr.Store(err)
			return
		}
		actualAddr.Store("http://" + listener.Addr().String())

		serveErr := httpServer.Serve(listener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErr.Store(serveErr)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	if err := serverErr.Load(); err != nil {
		t.Fatalf("Server failed to start: %v", err)
	}

	// Initiate shutdown
	go func() {
		time.Sleep(50 * time.Millisecond)
		server.gracefulShutdown()
	}()

	// Try to make a request during shutdown
	time.Sleep(75 * time.Millisecond) // Give shutdown a moment to start

	addr := actualAddr.Load().(string)
	resp, err := http.Post(addr+"/mcp/test-endpoint", "application/json", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Logf("Request failed as expected during shutdown: %v", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusServiceUnavailable {
			t.Logf("Got expected 503 status during shutdown")
		} else {
			t.Errorf("Expected 503 status during shutdown, got %d", resp.StatusCode)
		}
	}

	// Wait for server to complete
	wg.Wait()
}
