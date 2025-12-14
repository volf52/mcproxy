package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mcproxy/pkg/config"
)

// TestProxyHandler_HTTPMethodRestrictions tests that only POST methods are allowed
func TestProxyHandler_HTTPMethodRestrictions(t *testing.T) {
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

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{"GET method", "GET", http.StatusMethodNotAllowed},
		{"PUT method", "PUT", http.StatusMethodNotAllowed},
		{"DELETE method", "DELETE", http.StatusMethodNotAllowed},
		{"PATCH method", "PATCH", http.StatusMethodNotAllowed},
		{"HEAD method", "HEAD", http.StatusMethodNotAllowed},
		{"OPTIONS method", "OPTIONS", http.StatusMethodNotAllowed},
		{"TRACE method", "TRACE", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/mcp/test", nil)
			w := httptest.NewRecorder()
			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d for %s, got %d", tt.expectedStatus, tt.method, w.Code)
			}
		})
	}
}

// TestProxyHandler_RequestForwarding tests basic request forwarding with mock HTTP client
func TestProxyHandler_RequestForwarding(t *testing.T) {
	// Create mock HTTP client
	mockClient := NewMockHTTPClient()
	mockClient.SetResponse(http.StatusOK, `{"status": "success"}`)

	// Create server with mock client
	endpoints := map[string]config.Endpoint{
		"api": {
			Value: config.HttpEndpoint{
				Type: config.EndpointTypeHTTP,
				Url:  "http://api.example.com/webhook",
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"X-Custom":      "custom-value",
				},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient // Inject mock client

	handler := srv.createProxyHandler("api", endpoints["api"])

	// Create test request
	testBody := `{"test": "data"}`
	req := httptest.NewRequest("POST", "/mcp/api", strings.NewReader(testBody))
	// Don't set Content-Type to let the server set the default
	req.Header.Set("X-Incoming", "incoming-value")
	w := httptest.NewRecorder()

	// Execute handler
	handler(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != `{"status": "success"}` {
		t.Errorf("Expected body %q, got %q", `{"status": "success"}`, w.Body.String())
	}

	// Verify request was forwarded correctly
	mockClient.AssertRequestCount(t, 1)
	mockClient.AssertRequestCalled(t, "POST", "http://api.example.com/webhook")
	mockClient.AssertBody(t, "POST", "http://api.example.com/webhook", testBody)
	mockClient.AssertHeader(t, "POST", "http://api.example.com/webhook", "Authorization", "Bearer token123")
	mockClient.AssertHeader(t, "POST", "http://api.example.com/webhook", "X-Custom", "custom-value")
	mockClient.AssertHeader(t, "POST", "http://api.example.com/webhook", "Content-Type", "application/octet-stream")
}

// TestProxyHandler_RequestTooLarge tests body size limits
func TestProxyHandler_RequestTooLarge(t *testing.T) {
	mockClient := NewMockHTTPClient()
	mockClient.SetResponse(http.StatusOK, "OK")

	smallSize := int64(100)
	endpoints := map[string]config.Endpoint{
		"small": {
			Value: config.HttpEndpoint{
				EndpointShared: config.EndpointShared{
					MaxBodySize: &smallSize,
				},
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient

	handler := srv.createProxyHandler("small", endpoints["small"])

	// Create oversized request
	oversizedBody := strings.Repeat("x", 200) // 200 bytes > 100 bytes limit
	req := httptest.NewRequest("POST", "/mcp/small", strings.NewReader(oversizedBody))
	req.ContentLength = int64(len(oversizedBody))
	w := httptest.NewRecorder()

	handler(w, req)

	// Should return 413 Payload Too Large
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413 for oversized body, got %d", w.Code)
	}

	// Verify no request was made to upstream
	mockClient.AssertRequestCount(t, 0)
}

// TestProxyHandler_Timeout tests timeout handling
func TestProxyHandler_Timeout(t *testing.T) {
	mockClient := NewMockHTTPClient()
	// Simulate timeout by setting a delay and returning an error
	mockClient.SetDelay(100 * time.Millisecond)
	mockClient.SetError(context.DeadlineExceeded)

	shortTimeout := 50 * time.Millisecond
	endpoints := map[string]config.Endpoint{
		"timeout": {
			Value: config.HttpEndpoint{
				EndpointShared: config.EndpointShared{
					Timeout: &shortTimeout,
				},
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient

	handler := srv.createProxyHandler("timeout", endpoints["timeout"])

	req := httptest.NewRequest("POST", "/mcp/timeout", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	handler(w, req)
	elapsed := time.Since(start)

	// Should return 504 Gateway Timeout
	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("Expected status 504 for timeout, got %d", w.Code)
	}

	// Should fail quickly
	if elapsed > 200*time.Millisecond {
		t.Errorf("Request took too long: %v, should have timed out quickly", elapsed)
	}

	mockClient.AssertRequestCount(t, 1)
}

// TestProxyHandler_ContextCancellation tests context cancellation
func TestProxyHandler_ContextCancellation(t *testing.T) {
	mockClient := NewMockHTTPClient()
	mockClient.SetDelay(500 * time.Millisecond)
	mockClient.SetError(context.Canceled)

	endpoints := map[string]config.Endpoint{
		"cancel": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient

	// Create context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := srv.createProxyHandler("cancel", endpoints["cancel"])
	req := httptest.NewRequest("POST", "/mcp/cancel", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Cancel context immediately
	cancel()
	handler(w, req)

	// Should handle cancellation gracefully
	if w.Code == http.StatusOK {
		t.Error("Expected non-200 status due to cancellation")
	}
}

// TestProxyHandler_ShutdownStatus tests behavior during shutdown
func TestProxyHandler_ShutdownStatus(t *testing.T) {
	mockClient := NewMockHTTPClient()
	mockClient.SetResponse(http.StatusOK, "OK")

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
	srv.client = mockClient

	// Initiate shutdown
	srv.isShuttingDown.set(true)
	srv.shutdownCancel()

	handler := srv.createProxyHandler("test", endpoints["test"])

	req := httptest.NewRequest("POST", "/mcp/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Should return 503 Service Unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 during shutdown, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "shutting down") {
		t.Errorf("Expected shutdown message in body, got: %s", w.Body.String())
	}

	// Verify no request was made to upstream
	mockClient.AssertRequestCount(t, 0)
}

// TestProxyHandler_HeaderFiltering tests that hop-by-hop headers are filtered
func TestProxyHandler_HeaderFiltering(t *testing.T) {
	var capturedHeaders http.Header
	mockClient := NewMockHTTPClient()
	mockClient.SetHandler(func(req *http.Request) (*http.Response, error) {
		capturedHeaders = make(http.Header)
		for k, v := range req.Header {
			capturedHeaders[k] = v
		}
		return &http.Response{
			Status:     http.StatusText(http.StatusOK),
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("OK")),
			Request:    req,
		}, nil
	})

	endpoints := map[string]config.Endpoint{
		"filter": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient

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

	handler(w, req)

	// Verify hop-by-hop headers were filtered
	if capturedHeaders.Get("Connection") != "" {
		t.Error("Connection header should have been filtered")
	}
	if capturedHeaders.Get("Keep-Alive") != "" {
		t.Error("Keep-Alive header should have been filtered")
	}
	if capturedHeaders.Get("Upgrade") != "" {
		t.Error("Upgrade header should have been filtered")
	}
	if capturedHeaders.Get("Proxy-Connection") != "" {
		t.Error("Proxy-Connection header should have been filtered")
	}
	if capturedHeaders.Get("Transfer-Encoding") != "" {
		t.Error("Transfer-Encoding header should have been filtered")
	}

	// Verify custom header passed through
	if capturedHeaders.Get("X-Custom-Header") != "should-pass-through" {
		t.Error("X-Custom-Header should have passed through")
	}
}

// TestProxyHandler_CustomHeaders tests that custom headers override incoming headers
func TestProxyHandler_CustomHeaders(t *testing.T) {
	mockClient := NewMockHTTPClient()
	mockClient.SetResponse(http.StatusOK, "OK")

	endpoints := map[string]config.Endpoint{
		"test": {
			Value: config.HttpEndpoint{
				Type: config.EndpointTypeHTTP,
				Url:  "http://example.com",
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"X-Custom":      "custom-value",
					"X-Only":        "only-in-config",
				},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient

	handler := srv.createProxyHandler("test", endpoints["test"])

	req := httptest.NewRequest("POST", "/mcp/test", nil)
	// Set conflicting headers
	req.Header.Set("Authorization", "Bearer wrong-token")
	req.Header.Set("X-Custom", "wrong-value")
	req.Header.Set("X-Incoming", "incoming-value")
	w := httptest.NewRecorder()

	handler(w, req)

	// Custom headers should override incoming
	mockClient.AssertHeader(t, "POST", "http://example.com", "Authorization", "Bearer token123")
	mockClient.AssertHeader(t, "POST", "http://example.com", "X-Custom", "custom-value")
	mockClient.AssertHeader(t, "POST", "http://example.com", "X-Only", "only-in-config")

	// Incoming headers should still pass through if not overridden
	mockClient.AssertHeader(t, "POST", "http://example.com", "X-Incoming", "incoming-value")
}

// TestProxyHandler_DefaultValues tests that default timeout and max body size are applied
func TestProxyHandler_DefaultValues(t *testing.T) {
	endpoints := map[string]config.Endpoint{
		"defaults": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
				// No timeout or max body size specified
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})

	// Test default timeout
	timeout := srv.getEndpointTimeout(endpoints["defaults"])
	if timeout != 60*time.Second {
		t.Errorf("Expected default timeout of 60s, got %v", timeout)
	}

	// Test default max body size
	maxSize := srv.getEndpointMaxBodySize(endpoints["defaults"])
	expectedSize := int64(10 * 1024 * 1024) // 10MB
	if maxSize != expectedSize {
		t.Errorf("Expected default max body size of %d, got %d", expectedSize, maxSize)
	}
}

// TestProxyHandler_StreamingRequestBody tests that large request bodies are handled correctly
func TestProxyHandler_StreamingRequestBody(t *testing.T) {
	mockClient := NewMockHTTPClient()

	// Capture the request body to verify it was sent correctly
	var capturedBody []byte
	mockClient.SetHandler(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err == nil {
			capturedBody = body
		}
		return &http.Response{
			Status:     http.StatusText(http.StatusOK),
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("OK")),
			Request:    req,
		}, nil
	})

	largeSize := int64(10 * 1024 * 1024) // 10MB
	endpoints := map[string]config.Endpoint{
		"large": {
			Value: config.HttpEndpoint{
				EndpointShared: config.EndpointShared{
					MaxBodySize: &largeSize,
				},
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient

	handler := srv.createProxyHandler("large", endpoints["large"])

	// Create a large body (1MB)
	largeBody := strings.Repeat("x", 1024*1024)
	req := httptest.NewRequest("POST", "/mcp/large", strings.NewReader(largeBody))
	req.ContentLength = int64(len(largeBody))
	w := httptest.NewRecorder()

	handler(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify the entire body was captured
	if len(capturedBody) != len(largeBody) {
		t.Errorf("Expected body size %d, got %d", len(largeBody), len(capturedBody))
	}

	if !bytes.Equal(capturedBody, []byte(largeBody)) {
		t.Error("Large body content does not match")
	}
}

// TestProxyHandler_EmptyBody tests handling of empty request bodies
func TestProxyHandler_EmptyBody(t *testing.T) {
	mockClient := NewMockHTTPClient()
	mockClient.SetResponse(http.StatusNoContent, "")

	endpoints := map[string]config.Endpoint{
		"empty": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient

	handler := srv.createProxyHandler("empty", endpoints["empty"])

	// Create request with empty body
	req := httptest.NewRequest("POST", "/mcp/empty", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Verify response
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Verify request was made
	mockClient.AssertRequestCalled(t, "POST", "http://example.com")
	mockClient.AssertBody(t, "POST", "http://example.com", "")
}

// TestProxyHandler_ErrorResponse tests handling of error responses from upstream
func TestProxyHandler_ErrorResponse(t *testing.T) {
	mockClient := NewMockHTTPClient()
	mockClient.SetResponse(http.StatusInternalServerError, `{"error": "internal server error"}`)

	endpoints := map[string]config.Endpoint{
		"error": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient

	handler := srv.createProxyHandler("error", endpoints["error"])

	req := httptest.NewRequest("POST", "/mcp/error", strings.NewReader("test"))
	w := httptest.NewRecorder()

	handler(w, req)

	// Should forward the error response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	if w.Body.String() != `{"error": "internal server error"}` {
		t.Errorf("Expected error body, got %s", w.Body.String())
	}
}

// BenchmarkProxyHandler benchmarks the proxy handler without network I/O
func BenchmarkProxyHandler(b *testing.B) {
	mockClient := NewMockHTTPClient()
	mockClient.SetResponse(http.StatusOK, `{"result": "success"}`)

	endpoints := map[string]config.Endpoint{
		"bench": {
			Value: config.HttpEndpoint{
				Type:    config.EndpointTypeHTTP,
				Url:     "http://example.com",
				Headers: map[string]string{"X-Test": "benchmark"},
			},
		},
	}

	srv := NewServer(":8080", endpoints, config.ServerConfig{})
	srv.client = mockClient

	handler := srv.createProxyHandler("bench", endpoints["bench"])
	reqBody := strings.NewReader(`{"test": "data"}`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/mcp/bench", reqBody)
		w := httptest.NewRecorder()
		handler(w, req)
	}
}
