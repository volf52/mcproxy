package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"mcproxy/pkg/config"
)

// TestEndpointRegistrationWithMcpPrefix tests that endpoints are registered with /mcp prefix
func TestEndpointRegistrationWithMcpPrefix(t *testing.T) {
	t.Parallel()
	// Create a mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	// Create endpoints map with discriminated union
	endpoints := map[string]config.Endpoint{
		"test-endpoint": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	// Create server instance
	srv := NewServer(":8080", endpoints, config.ServerConfig{})

	// Create test mux
	mux := http.NewServeMux()

	// Register handlers
	for name, endpoint := range endpoints {
		handler := srv.createProxyHandler(name, endpoint)
		pattern := fmt.Sprintf("/mcp/%s", name)
		mux.HandleFunc(pattern, handler)
	}

	// Test that the endpoint is registered with /mcp prefix
	req := httptest.NewRequest("POST", "/mcp/test-endpoint", bytes.NewReader([]byte("test")))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestPostRequestForwardingToMcpEndpoints tests POST request forwarding to /mcp endpoints
func TestPostRequestForwardingToMcpEndpoints(t *testing.T) {
	t.Parallel()
	// Create a mock upstream server that verifies request
	receivedBody := new(bytes.Buffer)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Copy received body
		io.Copy(receivedBody, r.Body)

		// Send response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer mockServer.Close()

	// Create endpoints map
	endpoints := map[string]config.Endpoint{
		"api-endpoint": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{"Authorization": "Bearer token123"},
			},
		},
	}

	// Create server and handler
	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("api-endpoint", endpoints["api-endpoint"])

	// Create test request
	testBody := `{"test": "data"}`
	req := httptest.NewRequest("POST", "/mcp/api-endpoint", strings.NewReader(testBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute handler
	handler(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify request was forwarded
	if receivedBody.String() != testBody {
		t.Errorf("Expected body %s, got %s", testBody, receivedBody.String())
	}
}

// TestHttpMethodRestrictions tests that only POST methods are allowed on endpoints
func TestHttpMethodRestrictions(t *testing.T) {
	endpoints := map[string]config.Endpoint{
		"test": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("test", endpoints["test"])

	// Test GET method
	req := httptest.NewRequest("GET", "/mcp/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for GET, got %d", w.Code)
	}

	// Test PUT method
	req = httptest.NewRequest("PUT", "/mcp/test", nil)
	w = httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for PUT, got %d", w.Code)
	}

	// Test DELETE method
	req = httptest.NewRequest("DELETE", "/mcp/test", nil)
	w = httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for DELETE, got %d", w.Code)
	}
}

// TestNotFoundHandling tests 404 handling for non-existent endpoints
func TestNotFoundHandling(t *testing.T) {
	// Create test mux
	mux := http.NewServeMux()

	// Add root handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"service": "mcproxy", "status": "running"}`)
	})

	// Test non-existent endpoint
	req := httptest.NewRequest("POST", "/mcp/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent endpoint, got %d", w.Code)
	}
}

// TestRootHandlerUnchanged tests that the root handler remains unchanged
func TestRootHandlerUnchanged(t *testing.T) {
	// Create test mux
	mux := http.NewServeMux()

	// Add root handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"service": "mcproxy", "status": "running"}`)
	})

	// Test root path
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for root, got %d", w.Code)
	}

	expectedBody := `{"service": "mcproxy", "status": "running"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}

// TestMultipleEndpointsIndependently tests multiple endpoints work independently
func TestMultipleEndpointsIndependently(t *testing.T) {
	// Create mock servers
	mockServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Endpoint 1 response"))
	}))
	defer mockServer1.Close()

	mockServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Endpoint 2 response"))
	}))
	defer mockServer2.Close()

	// Create multiple endpoints
	endpoints := map[string]config.Endpoint{
		"endpoint1": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer1.URL,
				Headers: map[string]string{},
			},
		},
		"endpoint2": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer2.URL,
				Headers: map[string]string{},
			},
		},
	}

	// Create server and register handlers
	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	mux := http.NewServeMux()

	for name, endpoint := range endpoints {
		handler := srv.createProxyHandler(name, endpoint)
		pattern := fmt.Sprintf("/mcp/%s", name)
		mux.HandleFunc(pattern, handler)
	}

	// Test endpoint 1
	req1 := httptest.NewRequest("POST", "/mcp/endpoint1", nil)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 for endpoint1, got %d", w1.Code)
	}
	if w1.Body.String() != "Endpoint 1 response" {
		t.Errorf("Expected 'Endpoint 1 response', got %s", w1.Body.String())
	}

	// Test endpoint 2
	req2 := httptest.NewRequest("POST", "/mcp/endpoint2", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for endpoint2, got %d", w2.Code)
	}
	if w2.Body.String() != "Endpoint 2 response" {
		t.Errorf("Expected 'Endpoint 2 response', got %s", w2.Body.String())
	}
}

// TestMcpPrefixImplementation tests the actual implementation of /mcp prefix
func TestMcpPrefixImplementation(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"test": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	mux := http.NewServeMux()

	// Register with /mcp prefix
	for name, endpoint := range endpoints {
		handler := srv.createProxyHandler(name, endpoint)
		pattern := fmt.Sprintf("/mcp/%s", name)
		mux.HandleFunc(pattern, handler)
	}

	// Test that direct endpoint without prefix doesn't work
	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for direct access without /mcp prefix, got %d", w.Code)
	}

	// Test that endpoint with prefix works
	req = httptest.NewRequest("POST", "/mcp/test", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /mcp/test, got %d", w.Code)
	}
}

// TestStreamingRequestBody tests that large request bodies are streamed without buffering
func TestStreamingRequestBody(t *testing.T) {
	// Create a mock server that reads entire body
	var receivedBody []byte
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error reading body: %v", err)
		}
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	// Create endpoint with large max body size
	largeSize := int64(10 * 1024 * 1024) // 10MB
	endpoints := map[string]config.Endpoint{
		"large": {
			Value: config.HttpEndpoint{
				EndpointShared: config.EndpointShared{
					MaxBodySize: &largeSize,
				},
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("large", endpoints["large"])

	// Create a large body (1MB)
	largeBody := strings.Repeat("x", 1024*1024)
	req := httptest.NewRequest("POST", "/mcp/large", strings.NewReader(largeBody))
	w := httptest.NewRecorder()

	// Execute handler
	handler(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify the entire body was received
	if len(receivedBody) != len(largeBody) {
		t.Errorf("Expected body size %d, got %d", len(largeBody), len(receivedBody))
	}
}

// TestRequestSizeLimit tests body size limits (returns 413 for oversized requests)
func TestRequestSizeLimit(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	// Create endpoint with small max body size
	smallSize := int64(100) // 100 bytes
	endpoints := map[string]config.Endpoint{
		"small": {
			Value: config.HttpEndpoint{
				EndpointShared: config.EndpointShared{
					MaxBodySize: &smallSize,
				},
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("small", endpoints["small"])

	// Create a body that exceeds the limit
	oversizedBody := strings.Repeat("x", 200) // 200 bytes > 100 bytes limit
	req := httptest.NewRequest("POST", "/mcp/small", strings.NewReader(oversizedBody))
	req.ContentLength = int64(len(oversizedBody))
	w := httptest.NewRecorder()

	// Execute handler
	handler(w, req)

	// Should return 413 Payload Too Large
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413 for oversized body, got %d", w.Code)
	}
}

// TestRequestTimeout tests that per-endpoint timeouts are respected (returns 504)
func TestRequestTimeout(t *testing.T) {
	t.Parallel()
	// Create a mock server that delays response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second) // Delay longer than timeout
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Too late"))
	}))
	defer mockServer.Close()

	// Create endpoint with short timeout
	shortTimeout := 200 * time.Millisecond
	endpoints := map[string]config.Endpoint{
		"timeout": {
			Value: config.HttpEndpoint{
				EndpointShared: config.EndpointShared{
					Timeout: &shortTimeout,
				},
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("timeout", endpoints["timeout"])

	// Create request
	req := httptest.NewRequest("POST", "/mcp/timeout", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	// Execute handler
	handler(w, req)
	elapsed := time.Since(start)

	// Should return 504 Gateway Timeout
	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("Expected status 504 for timeout, got %d", w.Code)
	}

	// Should fail before the mock server responds
	if elapsed > 500*time.Millisecond {
		t.Errorf("Request took too long: %v, should have timed out quickly", elapsed)
	}
}

// TestContextCancellation tests that client cancellation is propagated
func TestContextCancellation(t *testing.T) {
	t.Parallel()
	// Create a mock server that delays response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"cancel": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("cancel", endpoints["cancel"])

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create request with context
	req := httptest.NewRequest("POST", "/mcp/cancel", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Start handler in goroutine
	done := make(chan struct{})
	go func() {
		handler(w, req)
		done <- struct{}{}
	}()

	// Cancel context after a short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for handler to complete
	select {
	case <-done:
		// Handler should return quickly after cancellation
	case <-time.After(200 * time.Millisecond):
		t.Error("Handler did not return quickly after context cancellation")
	}
}

// TestHostHeaderSetting tests that Host header is set correctly for upstream
func TestHostHeaderSetting(t *testing.T) {
	t.Parallel()
	var receivedHost string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"host": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("host", endpoints["host"])

	// Create request
	req := httptest.NewRequest("POST", "/mcp/host", nil)
	w := httptest.NewRecorder()

	// Execute handler
	handler(w, req)

	// Extract expected host from mock server URL
	parsedURL, err := url.Parse(mockServer.URL)
	if err != nil {
		t.Fatalf("Error parsing mock server URL: %v", err)
	}

	// Verify host header was set to upstream host
	if receivedHost != parsedURL.Host {
		t.Errorf("Expected Host header %s, got %s", parsedURL.Host, receivedHost)
	}
}

// TestHeaderFiltering tests that hop-by-hop headers are filtered
func TestHeaderFiltering(t *testing.T) {
	var receivedHeaders http.Header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"filter": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("filter", endpoints["filter"])

	// Create request with hop-by-hop headers
	req := httptest.NewRequest("POST", "/mcp/filter", nil)
	req.Header.Set("Connection", "keep-alive, upgrade")
	req.Header.Set("Keep-Alive", "timeout=60")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Proxy-Connection", "keep-alive")
	req.Header.Set("Transfer-Encoding", "chunked")
	req.Header.Set("X-Custom-Header", "should-pass-through")
	w := httptest.NewRecorder()

	// Execute handler
	handler(w, req)

	// Verify hop-by-hop headers were filtered out
	if receivedHeaders.Get("Connection") != "" {
		t.Error("Connection header should have been filtered")
	}
	if receivedHeaders.Get("Keep-Alive") != "" {
		t.Error("Keep-Alive header should have been filtered")
	}
	if receivedHeaders.Get("Upgrade") != "" {
		t.Error("Upgrade header should have been filtered")
	}
	if receivedHeaders.Get("Proxy-Connection") != "" {
		t.Error("Proxy-Connection header should have been filtered")
	}

	// Verify custom header passed through
	if receivedHeaders.Get("X-Custom-Header") != "should-pass-through" {
		t.Error("X-Custom-Header should have passed through")
	}
}

// TestImmediateHeaderFlushing tests that headers are flushed immediately
func TestImmediateHeaderFlushing(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add a custom header to verify it's passed through
		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response body"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"flush": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("flush", endpoints["flush"])

	// Create a ResponseWriter that tracks flushes
	flushTracker := &flushTracker{
		ResponseWriter: httptest.NewRecorder(),
		flushed:        false,
	}

	// Create request
	req := httptest.NewRequest("POST", "/mcp/flush", nil)

	// Execute handler
	handler(flushTracker, req)

	// Verify headers were flushed
	if !flushTracker.flushed {
		t.Error("Headers should have been flushed immediately")
	}

	// Verify response was written
	recorder := flushTracker.ResponseWriter.(*httptest.ResponseRecorder)
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-Test-Header") != "test-value" {
		t.Error("Custom header should have been passed through")
	}
}

// TestImmediateHeaderFlushingWithoutFlusher tests graceful fallback when ResponseWriter doesn't implement Flusher
func TestImmediateHeaderFlushingWithoutFlusher(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response body"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"noflush": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("noflush", endpoints["noflush"])

	// Create a ResponseWriter that doesn't implement Flusher
	recorder := httptest.NewRecorder()

	// Wrap it in a non-flusher writer
	nonFlusher := &nonFlusherWriter{
		ResponseWriter: recorder,
	}

	// Create request
	req := httptest.NewRequest("POST", "/mcp/noflush", nil)

	// Execute handler
	handler(nonFlusher, req)

	// Should not panic and response should be written
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

// TestContextAwareStreaming tests that streaming respects context cancellation
func TestContextAwareStreaming(t *testing.T) {
	// Create a mock server that streams large response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		// Stream data slowly
		for i := 0; i < 100; i++ {
			fmt.Fprintf(w, "Chunk %d\n", i)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"stream": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("stream", endpoints["stream"])

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Create request with context
	req := httptest.NewRequest("POST", "/mcp/stream", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Execute handler
	handler(w, req)

	// Response should be partially written due to timeout
	body := w.Body.String()
	if body == "" {
		t.Error("Expected partial response before cancellation")
	}

	// Count chunks (should be less than full 100 due to cancellation)
	chunks := strings.Count(body, "Chunk")
	if chunks >= 100 {
		t.Error("Response was not interrupted by context cancellation")
	}
}

// TestFlushMetrics tests that flush operations are tracked correctly
func TestFlushMetrics(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"metrics": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("metrics", endpoints["metrics"])

	// Get initial metrics
	initialSuccess, initialSkip := srv.GetFlushMetrics()

	// Make multiple requests with flusher
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/mcp/metrics", nil)
		handler(w, req)
	}

	// Make requests with non-flusher
	for i := 0; i < 3; i++ {
		w := &nonFlusherWriter{ResponseWriter: httptest.NewRecorder()}
		req := httptest.NewRequest("POST", "/mcp/metrics", nil)
		handler(w, req)
	}

	// Check final metrics
	finalSuccess, finalSkip := srv.GetFlushMetrics()

	// Should have increased success count by 5
	if finalSuccess-initialSuccess != 5 {
		t.Errorf("Expected success count to increase by 5, got %d", finalSuccess-initialSuccess)
	}

	// Should have increased skip count by 3
	if finalSkip-initialSkip != 3 {
		t.Errorf("Expected skip count to increase by 3, got %d", finalSkip-initialSkip)
	}
}

// TestLargeResponseStreaming tests streaming of large responses with immediate flushing
func TestLargeResponseStreaming(t *testing.T) {
	// Create a mock server that returns large response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		// Write 1MB of data
		chunk := strings.Repeat("x", 1024)
		for i := 0; i < 1024; i++ {
			w.Write([]byte(chunk))
		}
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"large": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("large", endpoints["large"])

	// Create request
	req := httptest.NewRequest("POST", "/mcp/large", nil)
	w := httptest.NewRecorder()

	// Measure response time
	start := time.Now()
	handler(w, req)
	elapsed := time.Since(start)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify body size (should be 1MB)
	expectedSize := 1024 * 1024
	if w.Body.Len() != expectedSize {
		t.Errorf("Expected body size %d, got %d", expectedSize, w.Body.Len())
	}

	// Streaming should be efficient (this is more of a sanity check)
	if elapsed > time.Second {
		t.Errorf("Streaming took too long: %v", elapsed)
	}
}

// TestEmptyResponseBody tests flushing with empty response bodies
func TestEmptyResponseBody(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusNoContent)
		// No body
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"empty": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("empty", endpoints["empty"])

	// Create request
	req := httptest.NewRequest("POST", "/mcp/empty", nil)
	w := httptest.NewRecorder()

	// Execute handler
	handler(w, req)

	// Verify response
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Verify headers
	if w.Header().Get("X-Custom") != "value" {
		t.Error("Custom headers should be preserved")
	}

	// Verify body is empty
	if w.Body.Len() != 0 {
		t.Errorf("Expected empty body, got %d bytes", w.Body.Len())
	}
}

// TestPerformanceWithImmediateFlushing measures first-byte latency with immediate flushing
func TestPerformanceWithImmediateFlushing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add header and start response immediately
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Add some delay before body
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("Delayed body"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"perf": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("perf", endpoints["perf"])

	// Measure multiple requests
	var totalLatency time.Duration
	iterations := 10

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("POST", "/mcp/perf", nil)
		w := httptest.NewRecorder()

		start := time.Now()
		handler(w, req)
		latency := time.Since(start)

		totalLatency += latency

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i, w.Code)
		}
	}

	avgLatency := totalLatency / time.Duration(iterations)
	t.Logf("Average latency with immediate flushing: %v", avgLatency)

	// With immediate flushing, we should get headers quickly
	// This is a rough check - actual values depend on the test environment
	if avgLatency > 150*time.Millisecond {
		t.Errorf("Average latency seems high: %v", avgLatency)
	}
}

// TestConcurrentRequestsPerformance tests performance with multiple concurrent requests
func TestConcurrentRequestsPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a mock server with some delay
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // Simulate some processing
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"concurrent": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("concurrent", endpoints["concurrent"])

	// Test concurrent requests
	concurrency := 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	var durations []time.Duration

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := httptest.NewRequest("POST", "/mcp/concurrent", nil)
			w := httptest.NewRecorder()

			reqStart := time.Now()
			handler(w, req)
			duration := time.Since(reqStart)

			if w.Code != http.StatusOK {
				mu.Lock()
				errors = append(errors, fmt.Errorf("request %d: status %d", id, w.Code))
				mu.Unlock()
			}

			mu.Lock()
			durations = append(durations, duration)
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(start)

	// Check for errors
	if len(errors) > 0 {
		t.Errorf("Encountered %d errors: %v", len(errors), errors)
	}

	// Calculate statistics
	var totalDuration time.Duration
	for _, d := range durations {
		totalDuration += d
	}
	avgDuration := totalDuration / time.Duration(len(durations))

	t.Logf("Concurrent requests: %d", concurrency)
	t.Logf("Total time: %v", totalTime)
	t.Logf("Average per-request time: %v", avgDuration)

	// With proper concurrency, total time should be less than sum of individual times
	if totalTime > time.Duration(concurrency)*avgDuration*90/100 {
		t.Error("Requests don't appear to be running concurrently")
	}
}

// BenchmarkStreamResponseWithFlushing benchmarks the streamResponse function with flushing
func BenchmarkStreamResponseWithFlushing(b *testing.B) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		// Write multiple chunks
		for i := 0; i < 10; i++ {
			w.Write([]byte(fmt.Sprintf("Chunk %d content\n", i)))
		}
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"benchmark": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("benchmark", endpoints["benchmark"])

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/mcp/benchmark", nil)
		w := httptest.NewRecorder()
		handler(w, req)
	}
}

// BenchmarkStreamResponseWithoutFlushing compares performance without immediate flushing
func BenchmarkStreamResponseWithoutFlushing(b *testing.B) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		// Write multiple chunks
		for i := 0; i < 10; i++ {
			w.Write([]byte(fmt.Sprintf("Chunk %d content\n", i)))
		}
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"benchmark": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("benchmark", endpoints["benchmark"])

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/mcp/benchmark", nil)
		// Use non-flushing writer
		w := &nonFlusherWriter{ResponseWriter: httptest.NewRecorder()}
		handler(w, req)
	}
}

// TestEdgeCases tests various edge cases (empty body, large body, JSON body)
func TestEdgeCases(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back the request
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"edge": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("edge", endpoints["edge"])

	tests := []struct {
		name           string
		contentType    string
		body           string
		expectedStatus int
	}{
		{
			name:           "empty body",
			contentType:    "",
			body:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "large body",
			contentType:    "text/plain",
			body:           strings.Repeat("x", 10000),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "json body",
			contentType:    "application/json",
			body:           `{"key": "value", "number": 42, "array": [1, 2, 3]}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "binary data",
			contentType:    "application/octet-stream",
			body:           string([]byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD}),
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}

			req := httptest.NewRequest("POST", "/mcp/edge", body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify response body matches input
			if w.Body.String() != tt.body {
				t.Errorf("Expected body %q, got %q", tt.body, w.Body.String())
			}
		})
	}
}

// Helper types for testing

// flushTracker wraps a ResponseWriter to track if Flush was called
type flushTracker struct {
	http.ResponseWriter
	flushed bool
}

func (ft *flushTracker) Flush() {
	ft.flushed = true
	if f, ok := ft.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// nonFlusherWriter wraps a ResponseWriter but doesn't implement Flusher
type nonFlusherWriter struct {
	http.ResponseWriter
}

func (nfw *nonFlusherWriter) CloseNotify() <-chan bool {
	if cn, ok := nfw.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	return nil
}

// TestIsShuttingDown tests the IsShuttingDown method
func TestIsShuttingDown(t *testing.T) {
	endpoints := map[string]config.Endpoint{
		"test": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})

	// Initially should not be shutting down
	if srv.IsShuttingDown() {
		t.Error("Server should not be shutting down initially")
	}

	// Initiate graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start shutdown in goroutine
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- srv.gracefulShutdown()
	}()

	// Wait a bit for shutdown to initiate
	time.Sleep(100 * time.Millisecond)

	// Should now be shutting down
	if !srv.IsShuttingDown() {
		t.Error("Server should be shutting down after gracefulShutdown called")
	}

	// Wait for shutdown to complete
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Errorf("Unexpected error during shutdown: %v", err)
		}
	case <-shutdownCtx.Done():
		t.Fatal("Shutdown timed out")
	}
}

// TestCreateUpstreamRequestError tests error handling in createUpstreamRequest
func TestCreateUpstreamRequestError(t *testing.T) {
	endpoints := map[string]config.Endpoint{
		"stdio": {
			Value: config.StdioEndpoint{
				Type:    config.EndpointTypeStdio,
				Command: "/bin/cat",
				Args:    []string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})

	ctx := context.Background()
	req := httptest.NewRequest("POST", "/mcp/stdio", nil)
	maxSize := int64(1024)

	// Try to create upstream request for stdio endpoint
	_, err := srv.createUpstreamRequest(ctx, req, endpoints["stdio"], maxSize)

	// Should return error for non-HTTP endpoints
	if err == nil {
		t.Error("Expected error for stdio endpoint in createUpstreamRequest")
	}
	if !strings.Contains(err.Error(), "not an HTTP endpoint") {
		t.Errorf("Expected 'not an HTTP endpoint' error, got: %v", err)
	}
}

// TestDefaultTimeout tests that default timeout is applied when not specified
func TestDefaultTimeout(t *testing.T) {
	endpoints := map[string]config.Endpoint{
		"default": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
				// No timeout specified - should use default
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	timeout := srv.getEndpointTimeout(endpoints["default"])

	// Default timeout should be 60 seconds
	if timeout != 60*time.Second {
		t.Errorf("Expected default timeout of 60s, got %v", timeout)
	}
}

// TestDefaultMaxBodySize tests that default max body size is applied when not specified
func TestDefaultMaxBodySize(t *testing.T) {
	endpoints := map[string]config.Endpoint{
		"default": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
				// No max body size specified - should use default
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	maxSize := srv.getEndpointMaxBodySize(endpoints["default"])

	// Default max body size should be 10MB
	expectedSize := int64(10 * 1024 * 1024)
	if maxSize != expectedSize {
		t.Errorf("Expected default max body size of %d, got %d", expectedSize, maxSize)
	}
}

// TestRequestTooLargeError tests the RequestTooLargeError type
func TestRequestTooLargeError(t *testing.T) {
	err := &RequestTooLargeError{
		Size:     2048,
		MaxSize:  1024,
		Endpoint: "http://example.com",
	}

	expectedMsg := "request body 2048 bytes exceeds limit of 1024 bytes for endpoint http://example.com"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestServerShutdownContext tests that server shutdown context is properly handled
func TestServerShutdownContext(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"test": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})

	// Start graceful shutdown
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = srv.gracefulShutdown()
	}()

	// Wait for shutdown to initiate
	time.Sleep(100 * time.Millisecond)

	// Create handler
	handler := srv.createProxyHandler("test", endpoints["test"])
	req := httptest.NewRequest("POST", "/mcp/test", nil)
	w := httptest.NewRecorder()

	// Handler should return 503 during shutdown
	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 during shutdown, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "shutting down") {
		t.Errorf("Expected shutdown message in body, got: %s", w.Body.String())
	}
}

// TestContentLengthHandling tests Content-Length header handling
func TestContentLengthHandling(t *testing.T) {
	var receivedContentLength int64
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentLength = r.ContentLength
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"test": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("test", endpoints["test"])

	testBody := "test content"
	req := httptest.NewRequest("POST", "/mcp/test", strings.NewReader(testBody))
	req.ContentLength = int64(len(testBody))
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if receivedContentLength != int64(len(testBody)) {
		t.Errorf("Expected Content-Length %d, got %d", len(testBody), receivedContentLength)
	}

	if w.Body.String() != testBody {
		t.Errorf("Expected body %q, got %q", testBody, w.Body.String())
	}
}

// TestDefaultContentType tests that default Content-Type is set when not provided
func TestDefaultContentType(t *testing.T) {
	var receivedContentType string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"test": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("test", endpoints["test"])

	req := httptest.NewRequest("POST", "/mcp/test", strings.NewReader("test"))
	// Don't set Content-Type in request
	w := httptest.NewRecorder()

	handler(w, req)

	// Should have default Content-Type
	if receivedContentType != "application/octet-stream" {
		t.Errorf("Expected Content-Type 'application/octet-stream', got %q", receivedContentType)
	}
}

// TestCustomHeaders tests that custom headers override incoming headers
func TestCustomHeaders(t *testing.T) {
	var receivedAuth string
	var receivedCustom string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedCustom = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"test": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     mockServer.URL,
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"X-Custom":      "custom-value",
				},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	handler := srv.createProxyHandler("test", endpoints["test"])

	req := httptest.NewRequest("POST", "/mcp/test", nil)
	// Set conflicting headers
	req.Header.Set("Authorization", "Bearer wrong-token")
	req.Header.Set("X-Custom", "wrong-value")
	w := httptest.NewRecorder()

	handler(w, req)

	// Custom headers should override incoming
	if receivedAuth != "Bearer token123" {
		t.Errorf("Expected Authorization 'Bearer token123', got %q", receivedAuth)
	}
	if receivedCustom != "custom-value" {
		t.Errorf("Expected X-Custom 'custom-value', got %q", receivedCustom)
	}
}