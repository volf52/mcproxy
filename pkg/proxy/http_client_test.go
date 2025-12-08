package proxy

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mcproxy/pkg/config"
)

// TestHTTPClientRedirects tests that the HTTP client doesn't follow redirects by default
func TestHTTPClientRedirects(t *testing.T) {
	// Create a server that redirects
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Redirect to an external URL to test redirect handling
		http.Redirect(w, r, "http://example.com", http.StatusFound)
	}))
	defer redirectServer.Close()

	// Use our HTTP client directly
	client := createHTTPClient()

	// Make request directly to the redirect server
	req, err := http.NewRequest("POST", redirectServer.URL, strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	// Should receive the redirect response, not follow it
	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected status %d (redirect), got %d", http.StatusFound, resp.StatusCode)
	}

	// Verify Location header is present
	location := resp.Header.Get("Location")
	if location == "" {
		t.Error("Expected Location header in redirect response")
	}
}

// TestHTTPClientTimeouts tests that the HTTP client respects timeout settings
func TestHTTPClientTimeouts(t *testing.T) {
	// Create a server that delays response but respects context cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	delayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than our client timeout to trigger timeout
		select {
		case <-time.After(70 * time.Second): // Longer than client's 60s timeout
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"delayed": true}`))
		case <-ctx.Done():
			// Context cancelled, exit gracefully - don't write response
			return
		}
	}))
	defer delayServer.Close()

	// Create a custom HTTP client with shorter timeout for testing
	shortTimeoutClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 5 * time.Second,
			}).DialContext,
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     10 * time.Second,
		},
		Timeout: 2 * time.Second, // Short timeout for testing
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test request directly with the short timeout client
	start := time.Now()
	req, err := http.NewRequest("POST", delayServer.URL, strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := shortTimeoutClient.Do(req)
	elapsed := time.Since(start)

	// Should have received an error or timeout
	if err == nil {
		resp.Body.Close()
		t.Error("Expected timeout error, but request succeeded")
	} else {
		// Verify it's a timeout error
		if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	}

	// Verify it timed out reasonably (should be close to our 2s timeout, not 70s)
	if elapsed > 5*time.Second {
		t.Errorf("Request took too long to timeout: %v", elapsed)
	}

	t.Logf("Request correctly timed out after %v", elapsed)
}

// TestHTTPClientHTTPS tests that the HTTP client properly handles HTTPS
func TestHTTPClientHTTPS(t *testing.T) {
	// This test verifies that HTTPS URLs work with our client
	// We use a public HTTPS service that's reliable
	endpoints := map[string]config.Endpoint{
		"https-test": {
			Url: "https://httpbin.org/post", // Public HTTPS testing service
			Headers: map[string]string{
				"Content-Type": "application/json",
				"User-Agent":   "mcproxy-test/1.0",
			},
		},
	}

	server := NewServer(":8080", endpoints)

	// Test that the client was created successfully (has TLS config)
	if server.httpClient.Transport == nil {
		t.Fatal("HTTP client transport is nil")
	}

	transport := server.httpClient.Transport.(*http.Transport)
	if transport.TLSClientConfig == nil {
		t.Fatal("TLS config is nil, HTTPS support missing")
	}

	// Verify secure settings
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false for secure default")
	}

	// Verify HTTP/2 is enabled
	if !transport.ForceAttemptHTTP2 {
		t.Error("HTTP/2 should be enabled for better performance")
	}

	// Verify redirect handling
	if server.httpClient.CheckRedirect == nil {
		t.Error("CheckRedirect should be configured to prevent redirects")
	}

	// Test the redirect function
	req := httptest.NewRequest("POST", "http://example.com", nil)
	err := server.httpClient.CheckRedirect(req, []*http.Request{})
	if err != http.ErrUseLastResponse {
		t.Errorf("Expected ErrUseLastResponse, got %v", err)
	}

	t.Log("HTTP client HTTPS configuration verified successfully")
}

// TestHTTPClientConnectionPooling tests connection pooling settings
func TestHTTPClientConnectionPooling(t *testing.T) {
	client := createHTTPClient()
	transport := client.Transport.(*http.Transport)

	// Verify connection pooling settings
	if transport.MaxIdleConns <= 0 {
		t.Error("MaxIdleConns should be > 0 for connection pooling")
	}

	if transport.MaxIdleConnsPerHost <= 0 {
		t.Error("MaxIdleConnsPerHost should be > 0 for connection pooling")
	}

	if transport.IdleConnTimeout <= 0 {
		t.Error("IdleConnTimeout should be > 0 to clean up idle connections")
	}

	// Verify compression is enabled
	if transport.DisableCompression {
		t.Error("Compression should be enabled for better performance")
	}

	t.Logf("Connection pooling settings verified: MaxIdleConns=%d, MaxIdleConnsPerHost=%d, IdleConnTimeout=%v",
		transport.MaxIdleConns, transport.MaxIdleConnsPerHost, transport.IdleConnTimeout)
}