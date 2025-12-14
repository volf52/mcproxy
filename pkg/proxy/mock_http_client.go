package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MockHTTPClient implements HTTPClient for testing
type MockHTTPClient struct {
	// Test control
	responses []MockResponse
	mu        sync.Mutex

	// Recorded requests for verification
	Requests []RequestRecord

	// Behavior control
	delay time.Duration
	err   error

	// Request handler function for custom behavior
	handler func(req *http.Request) (*http.Response, error)
}

// MockResponse defines a mock HTTP response
type MockResponse struct {
	Status int
	Header http.Header
	Body   string
	Error  error
}

// RequestRecord captures details of requests made to the mock
type RequestRecord struct {
	Method  string
	URL     string
	Header  http.Header
	Body    string
	Context context.Context
}

// NewMockHTTPClient creates a new mock HTTP client
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make([]MockResponse, 0),
		Requests:  make([]RequestRecord, 0),
	}
}

// SetResponse sets the response to return for subsequent requests
func (m *MockHTTPClient) SetResponse(status int, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = []MockResponse{{Status: status, Body: body}}
}

// SetResponses sets multiple responses to return in sequence
func (m *MockHTTPClient) SetResponses(responses []MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = responses
}

// SetError sets an error to return for requests
func (m *MockHTTPClient) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// SetDelay adds a delay to simulate network latency
func (m *MockHTTPClient) SetDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = delay
}

// SetHandler sets a custom handler function for requests
func (m *MockHTTPClient) SetHandler(handler func(req *http.Request) (*http.Response, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
}

// Do implements HTTPClient interface
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Record the request
	m.recordRequest(req)

	// Apply delay if configured
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Return error if configured
	if m.err != nil {
		return nil, m.err
	}

	// Use custom handler if provided
	if m.handler != nil {
		return m.handler(req)
	}

	// Return next configured response
	if len(m.responses) > 0 {
		resp := m.responses[0]
		// If multiple responses, cycle through them
		if len(m.responses) > 1 {
			m.responses = m.responses[1:]
		}

		// Create HTTP response
		body := io.NopCloser(strings.NewReader(resp.Body))
		header := make(http.Header)
		for k, v := range resp.Header {
			header[k] = v
		}

		return &http.Response{
			Status:     http.StatusText(resp.Status),
			StatusCode: resp.Status,
			Header:     header,
			Body:       body,
			Request:    req,
		}, nil
	}

	// Default response if none configured
	return &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

// recordRequest records details of the request for later verification
func (m *MockHTTPClient) recordRequest(req *http.Request) {
	var body string
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		body = string(bodyBytes)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	record := RequestRecord{
		Method:  req.Method,
		URL:     req.URL.String(),
		Header:  make(http.Header),
		Body:    body,
		Context: req.Context(),
	}

	// Copy headers
	for k, v := range req.Header {
		record.Header[k] = v
	}

	m.mu.Lock()
	m.Requests = append(m.Requests, record)
	m.mu.Unlock()
}

// GetRequests returns a copy of all recorded requests
func (m *MockHTTPClient) GetRequests() []RequestRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	requests := make([]RequestRecord, len(m.Requests))
	copy(requests, m.Requests)
	return requests
}

// ClearRequests clears all recorded requests
func (m *MockHTTPClient) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Requests = make([]RequestRecord, 0)
}

// AssertRequestCalled verifies that a request was made with the given method and URL
func (m *MockHTTPClient) AssertRequestCalled(t TestingT, method, url string) {
	requests := m.GetRequests()
	for _, req := range requests {
		if req.Method == method && req.URL == url {
			return
		}
	}
	t.Errorf("Expected request to %s %s, but it was not made. Requests: %+v", method, url, requests)
}

// AssertRequestNotCalled verifies that no request was made with the given method and URL
func (m *MockHTTPClient) AssertRequestNotCalled(t TestingT, method, url string) {
	requests := m.GetRequests()
	for _, req := range requests {
		if req.Method == method && req.URL == url {
			t.Errorf("Expected no request to %s %s, but it was made", method, url)
		}
	}
}

// AssertRequestCount verifies the number of requests made
func (m *MockHTTPClient) AssertRequestCount(t TestingT, expected int) {
	requests := m.GetRequests()
	if len(requests) != expected {
		t.Errorf("Expected %d requests, but got %d. Requests: %+v", expected, len(requests), requests)
	}
}

// AssertHeader verifies that a header was sent with a request
func (m *MockHTTPClient) AssertHeader(t TestingT, method, url, header, expectedValue string) {
	requests := m.GetRequests()
	for _, req := range requests {
		if req.Method == method && req.URL == url {
			value := req.Header.Get(header)
			if value == expectedValue {
				return
			}
			t.Errorf("Expected header %s to be %s, but got %s", header, expectedValue, value)
			return
		}
	}
	t.Errorf("No request to %s %s found to check header %s", method, url, header)
}

// AssertBody verifies that a request body was sent
func (m *MockHTTPClient) AssertBody(t TestingT, method, url, expectedBody string) {
	requests := m.GetRequests()
	for _, req := range requests {
		if req.Method == method && req.URL == url {
			if req.Body == expectedBody {
				return
			}
			t.Errorf("Expected body %q, but got %q", expectedBody, req.Body)
			return
		}
	}
	t.Errorf("No request to %s %s found to check body", method, url)
}

// TestingT interface matches testing.TB for our assertion methods
type TestingT interface {
	Errorf(format string, args ...interface{})
}

// Ensure MockHTTPClient implements HTTPClient
var _ HTTPClient = (*MockHTTPClient)(nil)
