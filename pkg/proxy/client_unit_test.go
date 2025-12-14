package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestCreateHTTPClient verifies the HTTP client configuration
func TestCreateHTTPClient(t *testing.T) {
	client := CreateHTTPClient()

	// Test timeout configuration
	if client.Timeout != 60*time.Second {
		t.Errorf("Expected client timeout to be 60s, got %v", client.Timeout)
	}

	// Test CheckRedirect policy
	if client.CheckRedirect == nil {
		t.Error("CheckRedirect policy should not be nil")
	} else {
		req, err := http.NewRequest("GET", "http://example.com", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		err = client.CheckRedirect(req, []*http.Request{})
		if err != http.ErrUseLastResponse {
			t.Errorf("Expected CheckRedirect to return http.ErrUseLastResponse, got %v", err)
		}
	}

	// Test transport configuration
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Expected Transport to be *http.Transport, got %T", client.Transport)
	}

	// Test TLS config
	if transport.TLSClientConfig == nil {
		t.Error("TLSClientConfig should not be nil")
	} else if transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false by default")
	}

	// Test dialer configuration
	if transport.DialContext == nil {
		t.Error("DialContext should not be nil")
	}

	// Test connection pooling settings
	if transport.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 10, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", transport.IdleConnTimeout)
	}

	// Test compression is enabled
	if transport.DisableCompression {
		t.Error("DisableCompression should be false to enable compression")
	}

	// Test HTTP/2 is enabled
	if !transport.ForceAttemptHTTP2 {
		t.Error("ForceAttemptHTTP2 should be true to enable HTTP/2")
	}
}

// TestHTTPClientFactory tests the HTTP client factory pattern
func TestHTTPClientFactory(t *testing.T) {
	// Test DefaultHTTPClientFactory
	factory := &DefaultHTTPClientFactory{}
	client := factory.CreateClient()

	// Should implement HTTPClient interface
	var _ HTTPClient = client

	// Should be a wrapper around *http.Client
	wrapper, ok := client.(*httpClientWrapper)
	if !ok {
		t.Fatalf("Expected *httpClientWrapper, got %T", client)
	}

	if wrapper.client == nil {
		t.Error("Wrapped client should not be nil")
	}

	// Test that the wrapper delegates Do() method correctly
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// This will likely fail since we're using a real URL in a unit test,
	// but we're testing that the delegation works
	_, err = wrapper.Do(req)
	// Don't assert an error - network might be available in test environment
}

// TestMockHTTPClient_BasicFunctionality tests basic mock HTTP client functionality
func TestMockHTTPClient_BasicFunctionality(t *testing.T) {
	mock := NewMockHTTPClient()

	// Test default response
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := mock.Do(req)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test request recording
	mock.AssertRequestCount(t, 1)
	mock.AssertRequestCalled(t, "GET", "http://example.com")
}

// TestMockHTTPClient_CustomResponses tests setting custom responses
func TestMockHTTPClient_CustomResponses(t *testing.T) {
	mock := NewMockHTTPClient()
	mock.SetResponse(http.StatusNotFound, `{"error": "not found"}`)

	req, err := http.NewRequest("GET", "http://example.com/resource", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := mock.Do(req)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}

	// Note: We can't easily read the body here since it might have been consumed
	// In practice, tests should set a handler to inspect the response
}

// TestMockHTTPClient_ErrorHandling tests error handling
func TestMockHTTPClient_ErrorHandling(t *testing.T) {
	mock := NewMockHTTPClient()
	mock.SetError(&net.DNSError{
		Err:        "no such host",
		Name:       "example.com",
		IsTimeout:  false,
		IsNotFound: true,
	})

	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = mock.Do(req)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "no such host") {
		t.Errorf("Expected DNS error, got: %v", err)
	}
}

// TestMockHTTPClient_Delay tests delay functionality
func TestMockHTTPClient_Delay(t *testing.T) {
	mock := NewMockHTTPClient()
	mock.SetDelay(100 * time.Millisecond)
	mock.SetResponse(http.StatusOK, "delayed")

	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	start := time.Now()
	_, err = mock.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected delay of at least 100ms, got %v", elapsed)
	}
}

// TestMockHTTPClient_SequenceOfResponses tests multiple responses in sequence
func TestMockHTTPClient_SequenceOfResponses(t *testing.T) {
	mock := NewMockHTTPClient()
	responses := []MockResponse{
		{Status: http.StatusOK, Body: "first"},
		{Status: http.StatusAccepted, Body: "second"},
		{Status: http.StatusCreated, Body: "third"},
	}
	mock.SetResponses(responses)

	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Make three requests
	for i, expected := range responses {
		resp, err := mock.Do(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		if resp.StatusCode != expected.Status {
			t.Errorf("Request %d: expected status %d, got %d", i, expected.Status, resp.StatusCode)
		}
	}

	// Fourth request should use the last response (cycling behavior)
	resp, err := mock.Do(req)
	if err != nil {
		t.Fatalf("Fourth request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Fourth request: expected status 201 (cycle to last), got %d", resp.StatusCode)
	}
}

// TestMockHTTPClient_CustomHandler tests custom handler functionality
func TestMockHTTPClient_CustomHandler(t *testing.T) {
	mock := NewMockHTTPClient()
	calls := 0
	mock.SetHandler(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			Status:     http.StatusText(http.StatusOK),
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(fmt.Sprintf("call %d", calls))),
			Request:    req,
		}, nil
	})

	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Make three requests
	for i := 1; i <= 3; i++ {
		resp, err := mock.Do(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Request %d: expected status 200, got %d", i, resp.StatusCode)
		}

		// Verify handler was called
		if calls != i {
			t.Errorf("Expected handler to be called %d times, got %d", i, calls)
		}
	}
}

// TestMockHTTPClient_RequestVerification tests request recording and verification
func TestMockHTTPClient_RequestVerification(t *testing.T) {
	mock := NewMockHTTPClient()
	mock.SetResponse(http.StatusOK, "OK")

	// Make a request with specific details
	req, err := http.NewRequest("POST", "http://example.com/api", strings.NewReader("test body"))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")

	_, err = mock.Do(req)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}

	// Verify request details
	mock.AssertRequestCount(t, 1)
	mock.AssertRequestCalled(t, "POST", "http://example.com/api")
	mock.AssertBody(t, "POST", "http://example.com/api", "test body")
	mock.AssertHeader(t, "POST", "http://example.com/api", "Content-Type", "application/json")
	mock.AssertHeader(t, "POST", "http://example.com/api", "Authorization", "Bearer token")

	// Test AssertRequestNotCalled
	mock.AssertRequestNotCalled(t, "GET", "http://example.com/api")
	mock.AssertRequestNotCalled(t, "POST", "http://other.com")

	// Test ClearRequests
	mock.ClearRequests()
	mock.AssertRequestCount(t, 0)
}

// BenchmarkMockHTTPClient benchmarks the mock HTTP client
func BenchmarkMockHTTPClient(b *testing.B) {
	mock := NewMockHTTPClient()
	mock.SetResponse(http.StatusOK, `{"result": "success"}`)

	req, err := http.NewRequest("POST", "http://example.com/api", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		b.Fatalf("Failed to create request: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := mock.Do(req)
		if err != nil {
			b.Fatalf("Do() failed: %v", err)
		}
	}
}
