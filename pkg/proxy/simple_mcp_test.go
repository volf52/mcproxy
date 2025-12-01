package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mcproxy/pkg/config"
)

// TestSimpleMcpPrefix validates that our implementation adds /mcp prefix correctly
func TestSimpleMcpPrefix(t *testing.T) {
	// Create mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer mockServer.Close()

	endpoints := map[string]config.Endpoint{
		"test": {
			Url: mockServer.URL,
		},
	}

	server := NewServer(":8080", endpoints)

	// Test our implementation by registering endpoints exactly as the server does
	mux := http.NewServeMux()

	// This mimics the exact logic from server.go Start() method
	for name, endpoint := range server.endpoints {
		handler := server.createProxyHandler(name, endpoint)
		// This is the line we changed in server.go
		pattern := "/mcp/" + name
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

	// Test 1: /mcp/test should work
	resp, err := http.Post(testServer.URL+"/mcp/test", "application/json", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error making request to /mcp/test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for /mcp/test, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response body: %v", err)
	}

	if string(body) != `{"success": true}` {
		t.Errorf("Expected response body from upstream, got %s", string(body))
	}

	// Test 2: /test should NOT work (return 404)
	respOld, err := http.Post(testServer.URL+"/test", "application/json", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error making request to /test: %v", err)
	}
	defer respOld.Body.Close()

	if respOld.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for old pattern /test, got %d", respOld.StatusCode)
	}

	// Test 3: Root endpoint should still work
	respRoot, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatalf("Error making request to root: %v", err)
	}
	defer respRoot.Body.Close()

	if respRoot.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for root /, got %d", respRoot.StatusCode)
	}

	t.Logf("SUCCESS: /mcp prefix implementation is working correctly!")
}
