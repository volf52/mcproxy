package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"mcproxy/pkg/config"
)

// TestGetBodyImplementation tests that the Request.GetBody function
// is properly implemented to support HTTP/2 retries
func TestGetBodyImplementation(t *testing.T) {
	// Create a test server that simulates an upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the body to ensure it's properly sent
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		// Echo back the body
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(body)
	}))
	defer upstream.Close()

	// Create test data
	testBody := "test request body for HTTP/2 retry"

	// Create a mock incoming request
	incomingReq := httptest.NewRequest(http.MethodPost, "/mcp/test", bytes.NewReader([]byte(testBody)))
	incomingReq.Header.Set("Content-Type", "application/octet-stream")

	// Create server instance
	s := &Server{}

	// Create endpoint configuration
	endpoint := config.Endpoint{
		Value: config.HttpEndpoint{
			Type:    "http",
			Url:     upstream.URL,
			Headers: make(map[string]string),
		},
	}

	// Create upstream request using our implementation
	ctx := context.Background()
	upstreamReq, err := s.createUpstreamRequest(ctx, incomingReq, endpoint, int64(len(testBody)*2))
	if err != nil {
		t.Fatalf("Failed to create upstream request: %v", err)
	}

	// Verify that GetBody is set
	if upstreamReq.GetBody == nil {
		t.Fatal("GetBody function is not set on the upstream request")
	}

	// Test that GetBody returns a new reader
	firstBody, err := upstreamReq.GetBody()
	if err != nil {
		t.Fatalf("GetBody failed: %v", err)
	}

	// Read from the first body
	firstBodyContent, err := io.ReadAll(firstBody)
	if err != nil {
		t.Fatalf("Failed to read from first GetBody result: %v", err)
	}

	// Test that GetBody can be called again and returns a new reader
	secondBody, err := upstreamReq.GetBody()
	if err != nil {
		t.Fatalf("Second GetBody call failed: %v", err)
	}

	// Read from the second body
	secondBodyContent, err := io.ReadAll(secondBody)
	if err != nil {
		t.Fatalf("Failed to read from second GetBody result: %v", err)
	}

	// Verify both bodies contain the same content
	if string(firstBodyContent) != testBody {
		t.Errorf("First body content mismatch: got %q, want %q", string(firstBodyContent), testBody)
	}

	if string(secondBodyContent) != testBody {
		t.Errorf("Second body content mismatch: got %q, want %q", string(secondBodyContent), testBody)
	}

	// Verify Content-Length is properly set
	if upstreamReq.ContentLength != int64(len(testBody)) {
		t.Errorf("Content-Length mismatch: got %d, want %d", upstreamReq.ContentLength, len(testBody))
	}
}

// TestGetBodyWithLargeBody tests GetBody with larger request bodies
func TestGetBodyWithLargeBody(t *testing.T) {
	// Create a 1MB test body
	bodySize := 1024 * 1024
	testBody := make([]byte, bodySize)
	for i := range testBody {
		testBody[i] = byte(i % 256)
	}

	// Create a mock incoming request
	incomingReq := httptest.NewRequest(http.MethodPost, "/mcp/test", bytes.NewReader(testBody))
	incomingReq.Header.Set("Content-Type", "application/octet-stream")

	// Create server instance
	s := &Server{}

	// Create endpoint configuration
	endpoint := config.Endpoint{
		Value: config.HttpEndpoint{
			Type:    "http",
			Url:     "http://example.com",
			Headers: make(map[string]string),
		},
	}

	// Create upstream request with a reasonable size limit
	ctx := context.Background()
	maxSize := int64(bodySize * 2) // Allow larger than the body
	upstreamReq, err := s.createUpstreamRequest(ctx, incomingReq, endpoint, maxSize)
	if err != nil {
		t.Fatalf("Failed to create upstream request: %v", err)
	}

	// Verify GetBody works with large bodies
	body, err := upstreamReq.GetBody()
	if err != nil {
		t.Fatalf("GetBody failed with large body: %v", err)
	}

	// Read and verify the content
	readBody, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read large body: %v", err)
	}

	if len(readBody) != bodySize {
		t.Errorf("Body size mismatch: got %d, want %d", len(readBody), bodySize)
	}

	// Verify first few bytes
	if !bytes.Equal(readBody[:10], testBody[:10]) {
		t.Error("Body content mismatch in first 10 bytes")
	}
}

// TestGetBodyWithSizeLimit tests that GetBody respects size limits
func TestGetBodyWithSizeLimit(t *testing.T) {
	// Create a test body larger than our limit
	bodySize := 200
	limit := 100
	testBody := make([]byte, bodySize)
	for i := range testBody {
		testBody[i] = byte(i)
	}

	// Create a mock incoming request
	incomingReq := httptest.NewRequest(http.MethodPost, "/mcp/test", bytes.NewReader(testBody))
	incomingReq.Header.Set("Content-Type", "application/octet-stream")

	// Create server instance
	s := &Server{}

	// Create endpoint configuration
	endpoint := config.Endpoint{
		Value: config.HttpEndpoint{
			Type:    "http",
			Url:     "http://example.com",
			Headers: make(map[string]string),
		},
	}

	// Create upstream request with size limit
	ctx := context.Background()
	upstreamReq, err := s.createUpstreamRequest(ctx, incomingReq, endpoint, int64(limit))
	if err != nil {
		t.Fatalf("Failed to create upstream request: %v", err)
	}

	// Verify GetBody returns limited body
	body, err := upstreamReq.GetBody()
	if err != nil {
		t.Fatalf("GetBody failed: %v", err)
	}

	// Read the limited body
	readBody, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read limited body: %v", err)
	}

	// Verify size is limited
	if len(readBody) != limit {
		t.Errorf("Body size not limited correctly: got %d, want %d", len(readBody), limit)
	}

	// Verify Content-Length reflects the limited size
	if upstreamReq.ContentLength != int64(limit) {
		t.Errorf("Content-Length not set correctly: got %d, want %d", upstreamReq.ContentLength, limit)
	}
}

// TestGetBodyWithEmptyBody tests GetBody with empty request body
func TestGetBodyWithEmptyBody(t *testing.T) {
	// Create a mock incoming request with empty body
	incomingReq := httptest.NewRequest(http.MethodPost, "/mcp/test", nil)

	// Create server instance
	s := &Server{}

	// Create endpoint configuration
	endpoint := config.Endpoint{
		Value: config.HttpEndpoint{
			Type:    "http",
			Url:     "http://example.com",
			Headers: make(map[string]string),
		},
	}

	// Create upstream request
	ctx := context.Background()
	upstreamReq, err := s.createUpstreamRequest(ctx, incomingReq, endpoint, 0)
	if err != nil {
		t.Fatalf("Failed to create upstream request: %v", err)
	}

	// Verify GetBody handles empty body correctly
	body, err := upstreamReq.GetBody()
	if err != nil {
		t.Fatalf("GetBody failed with empty body: %v", err)
	}

	// Read the empty body
	readBody, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read empty body: %v", err)
	}

	if len(readBody) != 0 {
		t.Errorf("Expected empty body, got %d bytes", len(readBody))
	}

	// Verify Content-Length is 0
	if upstreamReq.ContentLength != 0 {
		t.Errorf("Content-Length should be 0 for empty body, got %d", upstreamReq.ContentLength)
	}
}
