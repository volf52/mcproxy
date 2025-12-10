package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// createHTTPClient creates a shared HTTP client with proper timeouts and TLS support
func createHTTPClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false, // Secure by default
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false, // Enable compression
		// Additional TLS settings for better security and performance
		ForceAttemptHTTP2: true, // Enable HTTP/2 for better performance
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second, // Total request timeout
		// Do not follow redirects by default for security and predictability
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
