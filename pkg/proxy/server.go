package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"mcproxy/pkg/config"
	"mcproxy/pkg/logging"
)

// Server represents the proxy server
type Server struct {
	port       string
	endpoints  map[string]config.Endpoint
	httpClient *http.Client
}

// NewServer creates a new proxy server instance
func NewServer(port string, endpoints map[string]config.Endpoint) *Server {
	return &Server{
		port:       port,
		endpoints:  endpoints,
		httpClient: createHTTPClient(),
	}
}

// Start starts the proxy server and registers all endpoints
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register handlers for each endpoint
	for name, endpoint := range s.endpoints {
		handler := s.createProxyHandler(name, endpoint)
		pattern := fmt.Sprintf("/%s", name)
		mux.HandleFunc(pattern, handler)
		logging.LogEndpointRegistration(name, endpoint.Url, endpoint.Headers)
	}

	// Add a root handler for basic info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"service": "mcproxy", "status": "running"}`)
	})

	logging.Printf("Starting mcproxy server on port %s", s.port)
	return http.ListenAndServe(s.port, mux)
}

// createProxyHandler creates an HTTP handler for a specific endpoint
func (s *Server) createProxyHandler(name string, endpoint config.Endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Create the upstream request
		upstreamReq, err := s.createUpstreamRequest(r, endpoint)
		if err != nil {
			logging.Printf("Error creating upstream request for endpoint '%s': %v", name, err)
			logging.Debugf("Upstream request error details: endpoint='%s', error=%v", name, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Execute the upstream request
		resp, err := s.httpClient.Do(upstreamReq)
		if err != nil {
			logging.Printf("Error forwarding request for endpoint '%s': %v", name, err)
			logging.Debugf("Request forwarding error: endpoint='%s', upstream='%s', error=%v", name, endpoint.Url, err)
			http.Error(w, "Bad gateway", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Set response status code
		w.WriteHeader(resp.StatusCode)

		// Copy response body
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			logging.Printf("Error copying response body for endpoint '%s': %v", name, err)
			logging.Debugf("Response copy error: endpoint='%s', status=%d, error=%v", name, resp.StatusCode, err)
		}
	}
}

// createUpstreamRequest creates a new HTTP request to forward to the upstream URL
func (s *Server) createUpstreamRequest(r *http.Request, endpoint config.Endpoint) (*http.Request, error) {
	// Create new request with the same body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	upstreamReq, err := http.NewRequest(http.MethodPost, endpoint.Url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	// Copy headers from the incoming request
	for key, values := range r.Header {
		for _, value := range values {
			upstreamReq.Header.Add(key, value)
		}
	}

	// Add/override configured headers
	for headerName, headerValue := range endpoint.Headers {
		upstreamReq.Header.Set(headerName, headerValue)
		// Log header setting in debug mode only (without exposing the actual value)
		logging.Debugf("Setting header for upstream request: %s", headerName)
		if logging.IsDebugMode() {
			logging.Debugf("Header value: %s", headerValue)
		}
	}

	// Set Content-Type if not present
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/octet-stream")
	}

	return upstreamReq, nil
}
