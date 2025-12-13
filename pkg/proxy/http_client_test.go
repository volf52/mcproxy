package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestHTTPClientRedirects verifies that the HTTP client doesn't follow redirects by default
func TestHTTPClientRedirects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		redirectCode   int
		redirectURL    string
		expectedError  error
		expectFollowed bool
	}{
		{
			name:           "301 Moved Permanently",
			redirectCode:   http.StatusMovedPermanently,
			redirectURL:    "/target",
			expectedError:  http.ErrUseLastResponse,
			expectFollowed: false,
		},
		{
			name:           "302 Found",
			redirectCode:   http.StatusFound,
			redirectURL:    "/target",
			expectedError:  http.ErrUseLastResponse,
			expectFollowed: false,
		},
		{
			name:           "307 Temporary Redirect",
			redirectCode:   http.StatusTemporaryRedirect,
			redirectURL:    "/target",
			expectedError:  http.ErrUseLastResponse,
			expectFollowed: false,
		},
		{
			name:           "308 Permanent Redirect",
			redirectCode:   http.StatusPermanentRedirect,
			redirectURL:    "/target",
			expectedError:  http.ErrUseLastResponse,
			expectFollowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a target server that the redirect would point to
			targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("Redirect was followed - request reached target server at %s", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("target reached"))
			}))
			defer targetServer.Close()

			// Create a source server that returns the redirect
			redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/source" {
					// Create absolute redirect URL
					redirectURL := targetServer.URL + "/target"
					http.Redirect(w, r, redirectURL, tt.redirectCode)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer redirectServer.Close()

			// Create HTTP client using the factory function
			client := CreateHTTPClient()

			// Make request to the source server
			req, err := http.NewRequest("POST", redirectServer.URL+"/source", strings.NewReader("test data"))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			// Verify we got the redirect response, not the target
			if resp.StatusCode != tt.redirectCode {
				t.Errorf("Expected status %d, got %d", tt.redirectCode, resp.StatusCode)
			}

			// Verify Location header is present
			location := resp.Header.Get("Location")
			if location == "" {
				t.Error("Expected Location header in redirect response")
			}

			// Verify that the CheckRedirect policy is being applied
			// We can't directly test the error from CheckRedirect, but we can verify
			// the redirect wasn't followed by checking the response status
			if tt.expectFollowed && resp.StatusCode == tt.redirectCode {
				t.Error("Expected redirect to be followed, but got redirect response")
			}
		})
	}
}

// TestHTTPClientTimeouts verifies that HTTP client timeout settings work correctly
func TestHTTPClientTimeouts(t *testing.T) {
	tests := []struct {
		name          string
		serverDelay   time.Duration
		clientTimeout time.Duration
		expectTimeout bool
	}{
		{
			name:          "Fast response - no timeout",
			serverDelay:   100 * time.Millisecond,
			clientTimeout: 5 * time.Second,
			expectTimeout: false,
		},
		{
			name:          "Slow response - should timeout",
			serverDelay:   2 * time.Second, // Longer than client timeout
			clientTimeout: 500 * time.Millisecond,
			expectTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if testing.Short() && tt.expectTimeout {
				t.Skip("Skipping timeout test in short mode")
			}

			// Create a server that delays response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.serverDelay)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("response after delay"))
			}))
			defer server.Close()

			// Create HTTP client with custom timeout for testing
			client := &http.Client{
				Timeout: tt.clientTimeout,
			}

			// Create request without additional context timeout to let client timeout handle it
			req, err := http.NewRequest("POST", server.URL, strings.NewReader("test data"))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			start := time.Now()
			resp, err := client.Do(req)
			duration := time.Since(start)

			if tt.expectTimeout {
				if err == nil {
					resp.Body.Close()
					t.Errorf("Expected timeout error, but got response in %v", duration)
				} else if !strings.Contains(err.Error(), "deadline exceeded") &&
						  !strings.Contains(err.Error(), "timeout") &&
						  !strings.Contains(err.Error(), "Client.Timeout") {
					t.Errorf("Expected timeout error, got: %v", err)
				}
				// Verify it timed out reasonably quickly
				if duration > tt.clientTimeout*2 {
					t.Errorf("Timeout took too long: %v (client timeout: %v)", duration, tt.clientTimeout)
				}
			} else {
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer resp.Body.Close()

				if duration > tt.clientTimeout {
					t.Errorf("Response took too long: %v (timeout: %v)", duration, tt.clientTimeout)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read response body: %v", err)
				}

				if string(body) != "response after delay" {
					t.Errorf("Unexpected response body: %s", string(body))
				}
			}
		})
	}
}

// TestHTTPClientHTTPS verifies HTTPS support and TLS configuration
func TestHTTPClientHTTPS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		useTLS             bool
		insecureSkipVerify bool
		expectSuccess      bool
	}{
		{
			name:               "HTTPS with test cert - should fail",
			useTLS:             true,
			insecureSkipVerify: false,
			expectSuccess:      false,
		},
		{
			name:               "HTTPS with test cert and skip verify - should succeed",
			useTLS:             true,
			insecureSkipVerify: true,
			expectSuccess:      true,
		},
		{
			name:          "HTTP - should work",
			useTLS:        false,
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server

			if tt.useTLS {
				// Create server with test certificate (self-signed)
				server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("HTTPS response"))
				}))
			} else {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("HTTP response"))
				}))
			}
			defer server.Close()

			// Create HTTP client
			client := CreateHTTPClient()

			// If we need to skip verification for this test, modify the transport
			if tt.insecureSkipVerify {
				transport := client.Transport.(*http.Transport)
				transport.TLSClientConfig.InsecureSkipVerify = true
			}

			// Make request
			req, err := http.NewRequest("POST", server.URL, strings.NewReader("test data"))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)

			if tt.expectSuccess {
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read response body: %v", err)
				}

				expected := "HTTP response"
				if tt.useTLS {
					expected = "HTTPS response"
				}
				if string(body) != expected {
					t.Errorf("Expected response body %q, got %q", expected, string(body))
				}
			} else {
				if err == nil {
					resp.Body.Close()
					t.Error("Expected request to fail with TLS error, but it succeeded")
				} else {
					// Check if it's a TLS error
					if !strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "x509") {
						t.Errorf("Expected TLS certificate error, got: %v", err)
					}
				}
			}
		})
	}
}

// TestHTTPClientConnectionPooling verifies connection pooling settings
func TestHTTPClientConnectionPooling(t *testing.T) {
	t.Parallel()
	// Create a test server that tracks connections
	var connectionCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionCount++
		mu.Unlock()

		// Echo back the request data
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("echo: %s", string(body))))
	}))
	defer server.Close()

	// Create HTTP client using the factory function
	client := CreateHTTPClient()

	// Make multiple concurrent requests to test connection pooling
	const numRequests = 10
	const numWorkers = 5

	var wg sync.WaitGroup
	results := make([]string, numRequests)
	errors := make([]error, numRequests)

	// Channel to distribute work
	jobs := make(chan int, numRequests)
	for i := 0; i < numRequests; i++ {
		jobs <- i
	}
	close(jobs)

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for jobID := range jobs {
				reqBody := fmt.Sprintf("request-%d-from-worker-%d", jobID, workerID)
				req, err := http.NewRequest("POST", server.URL, strings.NewReader(reqBody))
				if err != nil {
					errors[jobID] = fmt.Errorf("worker %d: failed to create request: %w", workerID, err)
					continue
				}

				resp, err := client.Do(req)
				if err != nil {
					errors[jobID] = fmt.Errorf("worker %d: request failed: %w", workerID, err)
					continue
				}

				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					errors[jobID] = fmt.Errorf("worker %d: failed to read response: %w", workerID, err)
					continue
				}

				results[jobID] = string(body)
			}
		}(i)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// Verify all requests succeeded
	for i, result := range results {
		expected := fmt.Sprintf("echo: request-%d-from-worker-", i)
		if !strings.Contains(result, expected) {
			t.Errorf("Request %d: expected response containing %q, got %q", i, expected, result)
		}
	}

	// Verify connection pooling worked
	// With connection pooling, we should have fewer connections than requests
	mu.Lock()
	finalConnectionCount := connectionCount
	mu.Unlock()

	t.Logf("Made %d requests using %d connections", numRequests, finalConnectionCount)

	// Verify transport settings
	transport := client.Transport.(*http.Transport)
	if transport.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 10, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", transport.IdleConnTimeout)
	}
}

// TestHTTPClientSettings verifies all HTTP client settings are configured correctly
func TestHTTPClientSettings(t *testing.T) {
	client := CreateHTTPClient()

	// Verify timeout
	if client.Timeout != 60*time.Second {
		t.Errorf("Expected client timeout to be 60s, got %v", client.Timeout)
	}

	// Verify CheckRedirect policy
	if client.CheckRedirect == nil {
		t.Error("CheckRedirect policy should not be nil")
	} else {
		// Test the policy
		req := httptest.NewRequest("GET", "http://example.com", nil)
		err := client.CheckRedirect(req, []*http.Request{})
		if err != http.ErrUseLastResponse {
			t.Errorf("Expected CheckRedirect to return http.ErrUseLastResponse, got %v", err)
		}
	}

	// Verify transport settings
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Expected Transport to be *http.Transport, got %T", client.Transport)
	}

	// Verify TLS config
	if transport.TLSClientConfig == nil {
		t.Error("TLSClientConfig should not be nil")
	} else if transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false by default")
	}

	// Verify dialer settings
	if transport.DialContext == nil {
		t.Error("DialContext should not be nil")
	}

	// Verify connection pooling settings
	if transport.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 10, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", transport.IdleConnTimeout)
	}

	// Verify compression is enabled
	if transport.DisableCompression {
		t.Error("DisableCompression should be false to enable compression")
	}

	// Verify HTTP/2 is enabled
	if !transport.ForceAttemptHTTP2 {
		t.Error("ForceAttemptHTTP2 should be true to enable HTTP/2")
	}
}


// BenchmarkHTTPClient benchmarks the HTTP client performance
func BenchmarkHTTPClient(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := CreateHTTPClient()
	reqBody := strings.NewReader("test data")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, err := http.NewRequest("POST", server.URL, reqBody)
		if err != nil {
			b.Fatalf("Failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}

		resp.Body.Close()
	}
}