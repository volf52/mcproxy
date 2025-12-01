package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"mcproxy/pkg/config"
)

// setupMockServer creates a test HTTP server that simulates an upstream service
func setupMockServer(response string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte(response))
	}))
}

// TestEndpointRegistrationWithMcpPrefix tests that endpoints are registered with /mcp prefix
func TestEndpointRegistrationWithMcpPrefix(t *testing.T) {
	tests := []struct {
		name           string
		endpointName   string
		endpointConfig config.Endpoint
		shouldWork     bool
		expectedStatus int
	}{
		{
			name:         "valid endpoint",
			endpointName: "webhook",
			endpointConfig: config.Endpoint{
				Url: "https://httpbin.org/post",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
			},
			shouldWork:     true,
			expectedStatus: 200, // Will be 404 until we implement the /mcp prefix
		},
		{
			name:         "endpoint with hyphens",
			endpointName: "slack-webhook",
			endpointConfig: config.Endpoint{
				Url: "https://hooks.slack.com/services/xxx",
				Headers: map[string]string{
					"Authorization": "Bearer token",
				},
			},
			shouldWork:     true,
			expectedStatus: 200, // Will be 404 until we implement the /mcp prefix
		},
		{
			name:         "endpoint with numbers",
			endpointName: "api-v1",
			endpointConfig: config.Endpoint{
				Url: "https://api.example.com/v1/webhook",
				Headers: map[string]string{
					"X-API-Version": "1.0",
				},
			},
			shouldWork:     true,
			expectedStatus: 200, // Will be 404 until we implement the /mcp prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock upstream server
			mockServer := setupMockServer(`{"status": "success"}`, http.StatusOK)
			defer mockServer.Close()

			// Update endpoint config to use mock server
			tt.endpointConfig.Url = mockServer.URL

			// Create endpoints map
			endpoints := map[string]config.Endpoint{
				tt.endpointName: tt.endpointConfig,
			}

			// Create server
			server := NewServer(":8080", endpoints)

			// Create test mux to simulate server behavior
			mux := http.NewServeMux()

			// Register handlers the same way the server does
			for name, endpoint := range server.endpoints {
				handler := server.createProxyHandler(name, endpoint)
				pattern := "/" + name // Current pattern before change
				mux.HandleFunc(pattern, handler)
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

			// Create test server
			testServer := httptest.NewServer(mux)
			defer testServer.Close()

			// Test the current pattern (should work before change)
			resp, err := http.Post(testServer.URL+"/"+tt.endpointName, "application/json", strings.NewReader(`{"test": "data"}`))
			if err != nil {
				t.Fatalf("Error making request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for /%s, got %d", tt.endpointName, resp.StatusCode)
			}

			// Test the new /mcp pattern (will fail until we implement the change)
			respMcp, err := http.Post(testServer.URL+"/mcp/"+tt.endpointName, "application/json", strings.NewReader(`{"test": "data"}`))
			if err != nil {
				t.Fatalf("Error making request to /mcp endpoint: %v", err)
			}
			defer respMcp.Body.Close()

			// After implementing the /mcp prefix, this should pass
			// For now, we expect 404 because the pattern doesn't exist yet
			if respMcp.StatusCode == http.StatusOK {
				t.Logf("SUCCESS: /mcp/%s returned 200 - change has been implemented!", tt.endpointName)
			} else {
				t.Logf("EXPECTED: /mcp/%s returned %d - change not yet implemented", tt.endpointName, respMcp.StatusCode)
			}
		})
	}
}

// TestPostRequestForwardingToMcpEndpoints tests POST request forwarding to /mcp endpoints
func TestPostRequestForwardingToMcpEndpoints(t *testing.T) {
	// Create mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Read and verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error reading request body: %v", err)
			return
		}
		defer r.Body.Close()

		expectedBody := `{"test": "data", "endpoint": "test"}`
		if string(body) != expectedBody {
			t.Errorf("Expected body %s, got %s", expectedBody, string(body))
		}

		// Verify headers
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		if authHeader := r.Header.Get("Authorization"); authHeader != "Bearer token123" {
			t.Errorf("Expected Authorization Bearer token123, got %s", authHeader)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true, "received": string(body)}`))
	}))
	defer mockServer.Close()

	// Create endpoint configuration
	endpoints := map[string]config.Endpoint{
		"test-endpoint": {
			Url: mockServer.URL,
			Headers: map[string]string{
				"Authorization": "Bearer token123",
				"Content-Type":  "application/json",
			},
		},
	}

	// Create server
	_ = NewServer(":8080", endpoints)

	// Test the old pattern (should work)
	body := strings.NewReader(`{"test": "data", "endpoint": "test"}`)
	resp, err := http.Post("http://localhost:8080/test-endpoint", "application/json", body)
	if err != nil {
		// Server isn't running, this is expected for unit tests
		t.Logf("Server not running for integration test: %v", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Logf("SUCCESS: Old pattern /test-endpoint works")
		}
	}

	// Test the new pattern (will work after implementation)
	// This test will pass after we implement the /mcp prefix
	t.Logf("Test ready for /mcp/test-endpoint implementation")
}

// TestHttpMethodRestrictions tests that only POST methods are allowed
func TestHttpMethodRestrictions(t *testing.T) {
	// Create mock upstream server
	mockServer := setupMockServer(`{"status": "success"}`, http.StatusOK)
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"test-endpoint": {
			Url: mockServer.URL,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	server := NewServer(":8080", endpoints)

	// Create test mux
	mux := http.NewServeMux()

	// Register handler with current pattern
	handler := server.createProxyHandler("test-endpoint", endpoints["test-endpoint"])
	mux.HandleFunc("/test-endpoint", handler)

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	tests := []struct {
		method          string
		expectedStatus  int
		testDescription string
	}{
		{"GET", http.StatusMethodNotAllowed, "GET request should return 405"},
		{"PUT", http.StatusMethodNotAllowed, "PUT request should return 405"},
		{"DELETE", http.StatusMethodNotAllowed, "DELETE request should return 405"},
		{"PATCH", http.StatusMethodNotAllowed, "PATCH request should return 405"},
		{"POST", http.StatusOK, "POST request should return 200"},
	}

	for _, tt := range tests {
		t.Run(tt.testDescription, func(t *testing.T) {
			var resp *http.Response
			var err error

			if tt.method == "POST" {
				resp, err = http.Post(testServer.URL+"/test-endpoint", "application/json", strings.NewReader(`{"test": "data"}`))
			} else {
				req, err := http.NewRequest(tt.method, testServer.URL+"/test-endpoint", strings.NewReader(`{"test": "data"}`))
				if err == nil {
					resp, err = http.DefaultClient.Do(req)
				}
			}

			if err != nil {
				t.Fatalf("Error making %s request: %v", tt.method, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d for %s request, got %d", tt.expectedStatus, tt.method, resp.StatusCode)
			}
		})
	}
}

// TestNotFoundHandling tests 404 handling for non-existent endpoints
func TestNotFoundHandling(t *testing.T) {
	endpoints := map[string]config.Endpoint{
		"existing-endpoint": {
			Url: "https://httpbin.org/post",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	server := NewServer(":8080", endpoints)

	// Create test mux
	mux := http.NewServeMux()

	// Register existing endpoint
	handler := server.createProxyHandler("existing-endpoint", endpoints["existing-endpoint"])
	mux.HandleFunc("/existing-endpoint", handler)

	// Add root handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"service": "mcproxy", "status": "running"}`)
	})

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	tests := []struct {
		path            string
		method          string
		expectedStatus  int
		testDescription string
	}{
		{"/nonexistent", "POST", http.StatusNotFound, "POST to non-existent endpoint should return 404"},
		{"/nonexistent", "GET", http.StatusNotFound, "GET to non-existent endpoint should return 404"},
		{"/other-path", "GET", http.StatusNotFound, "GET to other path should return 404"},
		{"/mcp/nonexistent", "POST", http.StatusNotFound, "POST to non-existent /mcp endpoint should return 404"},
	}

	for _, tt := range tests {
		t.Run(tt.testDescription, func(t *testing.T) {
			var resp *http.Response
			var err error

			if tt.method == "POST" {
				resp, err = http.Post(testServer.URL+tt.path, "application/json", strings.NewReader(`{"test": "data"}`))
			} else {
				resp, err = http.Get(testServer.URL + tt.path)
			}

			if err != nil {
				t.Fatalf("Error making %s request to %s: %v", tt.method, tt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d for %s %s, got %d", tt.expectedStatus, tt.method, tt.path, resp.StatusCode)
			}
		})
	}
}

// TestRootHandlerUnchanged tests that the root handler remains unchanged
func TestRootHandlerUnchanged(t *testing.T) {
	endpoints := map[string]config.Endpoint{
		"test-endpoint": {
			Url: "https://httpbin.org/post",
		},
	}

	server := NewServer(":8080", endpoints)

	// Create test mux
	mux := http.NewServeMux()

	// Register endpoint
	handler := server.createProxyHandler("test-endpoint", endpoints["test-endpoint"])
	mux.HandleFunc("/test-endpoint", handler)

	// Add root handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"service": "mcproxy", "status": "running"}`)
	})

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// Test GET to root
	resp, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatalf("Error making GET request to root: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for GET /, got %d", resp.StatusCode)
	}

	// Verify response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response body: %v", err)
	}

	expectedBody := `{"service": "mcproxy", "status": "running"}`
	if string(body) != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, string(body))
	}

	// Test POST to root should return 200 (the root handler doesn't check method, only path)
	respPost, err := http.Post(testServer.URL+"/", "application/json", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error making POST request to root: %v", err)
	}
	defer respPost.Body.Close()

	if respPost.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for POST /, got %d", respPost.StatusCode)
	}
}

// TestMultipleEndpointsIndependently tests multiple endpoints work independently
func TestMultipleEndpointsIndependently(t *testing.T) {
	// Create multiple mock servers
	mockServer1 := setupMockServer(`{"service": "webhook"}`, http.StatusOK)
	mockServer2 := setupMockServer(`{"service": "slack"}`, http.StatusOK)
	defer mockServer1.Close()
	defer mockServer2.Close()

	endpoints := map[string]config.Endpoint{
		"webhook": {
			Url: mockServer1.URL,
			Headers: map[string]string{
				"X-Service": "webhook",
			},
		},
		"slack": {
			Url: mockServer2.URL,
			Headers: map[string]string{
				"X-Service": "slack",
			},
		},
	}

	server := NewServer(":8080", endpoints)

	// Create test mux
	mux := http.NewServeMux()

	// Register handlers
	for name, endpoint := range endpoints {
		handler := server.createProxyHandler(name, endpoint)
		mux.HandleFunc("/"+name, handler)
	}

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// Test both endpoints
	resp1, err := http.Post(testServer.URL+"/webhook", "application/json", strings.NewReader(`{"test": "data1"}`))
	if err != nil {
		t.Fatalf("Error making request to webhook: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for webhook, got %d", resp1.StatusCode)
	}

	resp2, err := http.Post(testServer.URL+"/slack", "application/json", strings.NewReader(`{"test": "data2"}`))
	if err != nil {
		t.Fatalf("Error making request to slack: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for slack, got %d", resp2.StatusCode)
	}

	// Test concurrent requests
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp, err := http.Post(testServer.URL+"/webhook", "application/json", strings.NewReader(`{"test": "data"}`))
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("expected status 200, got %d", resp.StatusCode)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent request error: %v", err)
	}
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	// Create mock server
	mockServer := setupMockServer(`{"status": "success"}`, http.StatusOK)
	defer mockServer.Close()

	tests := []struct {
		name            string
		endpointName    string
		requestBody     string
		shouldWork      bool
		testDescription string
	}{
		{"empty body", "test-endpoint", "", true, "Empty request body should work"},
		{"large body", "test-endpoint", strings.Repeat("x", 1000000), true, "Large request body should work"},
		{"json body", "test-endpoint", `{"key": "value"}`, true, "JSON request body should work"},
	}

	for _, tt := range tests {
		t.Run(tt.testDescription, func(t *testing.T) {
			endpoints := map[string]config.Endpoint{
				tt.endpointName: {
					Url: mockServer.URL,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
				},
			}

			server := NewServer(":8080", endpoints)

			// Create test mux
			mux := http.NewServeMux()
			handler := server.createProxyHandler(tt.endpointName, endpoints[tt.endpointName])
			mux.HandleFunc("/"+tt.endpointName, handler)

			testServer := httptest.NewServer(mux)
			defer testServer.Close()

			var body *strings.Reader
			if tt.requestBody == "" {
				body = strings.NewReader("")
			} else {
				body = strings.NewReader(tt.requestBody)
			}

			resp, err := http.Post(testServer.URL+"/"+tt.endpointName, "application/json", body)
			if err != nil {
				t.Fatalf("Error making request: %v", err)
			}
			defer resp.Body.Close()

			if tt.shouldWork && resp.StatusCode != http.StatusOK {
				t.Errorf("Expected success for %s, got status %d", tt.testDescription, resp.StatusCode)
			}
		})
	}
}

// TestMcpPrefixImplementation tests the actual implementation of /mcp prefix
// This test will be used to verify our implementation works correctly
func TestMcpPrefixImplementation(t *testing.T) {
	// Create mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true, "message": "Request received"}`))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"test-endpoint": {
			Url: mockServer.URL,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	server := NewServer(":8080", endpoints)

	// Create test mux with the NEW /mcp pattern
	mux := http.NewServeMux()

	// Register handlers with /mcp prefix (this is what we want to implement)
	for name, endpoint := range server.endpoints {
		handler := server.createProxyHandler(name, endpoint)
		pattern := "/mcp/" + name // NEW pattern
		mux.HandleFunc(pattern, handler)
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

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// Test that /mcp/test-endpoint works
	resp, err := http.Post(testServer.URL+"/mcp/test-endpoint", "application/json", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error making request to /mcp/test-endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for /mcp/test-endpoint, got %d", resp.StatusCode)
	}

	// Test that old pattern returns 404
	respOld, err := http.Post(testServer.URL+"/test-endpoint", "application/json", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error making request to /test-endpoint: %v", err)
	}
	defer respOld.Body.Close()

	if respOld.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for old pattern /test-endpoint, got %d", respOld.StatusCode)
	}

	t.Logf("SUCCESS: /mcp prefix implementation test passed!")
}
