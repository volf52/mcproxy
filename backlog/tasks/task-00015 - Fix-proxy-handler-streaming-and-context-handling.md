---
id: task-00015
title: Fix proxy handler streaming and context handling
status: Done
assignee: []
created_date: '2025-12-11 19:55'
updated_date: '2025-12-12 01:41'
labels:
  - bug
  - streaming
  - context-handling
  - performance
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The proxy handler in pkg/proxy/server.go has critical issues with streaming and context handling:

1. **Memory Buffering**: Lines 106-111 use `io.ReadAll(r.Body)` followed by `strings.NewReader`, which buffers the entire request body in memory. This causes:
   - High memory usage for large uploads
   - Increased latency (must read entire body before forwarding)
   - Potential OOM crashes with very large requests

2. **Context Propagation**: The upstream request is created without proper context propagation (line 111). This causes:
   - Upstream calls continue after client disconnects
   - No cancellation propagation when client cancels
   - Wasted resources and slow error detection

3. **Header Management**: Lines 117-121 copy ALL headers from incoming request without filtering, including:
   - Hop-by-hop headers (Connection, Keep-Alive, Proxy-Authenticate, etc.)
   - The Host header (should be based on upstream URL)
   - Transfer-Encoding header (may conflict with streaming)
   - Potentially sensitive headers that shouldn't be forwarded

These issues violate HTTP proxy best practices and can cause protocol errors, resource leaks, and security concerns.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 [ ] Request bodies are streamed without buffering entire content in memory
- [ ] #2 [ ] Client context is properly propagated to upstream requests
- [ ] #3 [ ] Upstream requests are cancelled when client disconnects
- [ ] #4 [ ] Hop-by-hop headers (Connection, Keep-Alive, etc.) are filtered out
- [ ] #5 [ ] Host header is set based on upstream URL, not copied from client
- [ ] #6 [ ] Transfer-Encoding is handled correctly for streaming
- [ ] #7 [ ] Request size limits are enforced (configurable, default 10MB)
- [ ] #8 [ ] Returns 413 Payload Too Large for oversized requests
- [ ] #9 [ ] All existing functionality remains intact (headers merging, etc.)
- [ ] #10 [ ] Added comprehensive unit tests for new streaming behavior
- [ ] #11 [ ] Added tests for client cancellation scenarios
- [ ] #12 [ ] Verified memory usage remains constant regardless of request size
- [ ] #13 [ ] Error messages are clear and include relevant context
- [ ] #14 [ ] Performance tests show no regression for small requests
- [ ] #15 [ ] Large file uploads work without memory issues
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
## Implementation Plan with Code Examples

### Step 1: Update createUpstreamRequest Signature
```go
// New signature with context and size limit
func (s *Server) createUpstreamRequest(
    ctx context.Context,
    r *http.Request,
    endpoint config.Endpoint,
    maxBodySize int64,
) (*http.Request, error)
```

### Step 2: Add Request Size Validation
```go
// Check Content-Length if present
if contentLength := r.ContentLength; contentLength > maxBodySize {
    return nil, &RequestTooLargeError{
        Size:     contentLength,
        MaxSize:  maxBodySize,
        Endpoint: endpoint.Url,
    }
}
```

### Step 3: Implement Streaming Request Creation
```go
// Create request with context and streaming body
upstreamReq, err := http.NewRequestWithContext(
    ctx,
    http.MethodPost,
    endpoint.Url,
    r.Body, // Use body directly, no buffering
)
if err != nil {
    return nil, fmt.Errorf("failed to create upstream request: %w", err)
}
```

### Step 4: Add Header Filtering
```go
// Filter out hop-by-hop headers
hopByHopHeaders := map[string]bool{
    "connection":        true,
    "keep-alive":        true,
    "proxy-authenticate": true,
    "proxy-authorization": true,
    "te":               true,
    "trailers":         true,
    "transfer-encoding": true,
    "upgrade":          true,
}

// Copy only safe headers
for key, values := range r.Header {
    normalized := strings.ToLower(key)
    if !hopByHopHeaders[normalized] && normalized != "host" {
        for _, value := range values {
            upstreamReq.Header.Add(key, value)
        }
    }
}
```

### Step 5: Set Proper Host Header
```go
// Parse upstream URL to set correct Host
if parsedURL, err := url.Parse(endpoint.Url); err == nil {
    upstreamReq.Host = parsedURL.Host
}
```

### Step 6: Add Response Streaming Improvement
```go
// In createProxyHandler, improve response handling
// Flush headers immediately
w.WriteHeader(resp.StatusCode)
if flusher, ok := w.(http.Flusher); ok {
    flusher.Flush()
}

// Copy response with context awareness
done := make(chan struct{})
go func() {
    defer close(done)
    _, err = io.Copy(w, resp.Body)
    if err != nil {
        select {
        case <-ctx.Done():
            // Client disconnected, expected error
            logging.Debugf("Client disconnected during response copy: %v", ctx.Err())
        default:
            // Unexpected error
            logging.Printf("Error copying response body: %v", err)
        }
    }
}()

// Wait for copy or context cancellation
select {
case <-done:
    // Normal completion
case <-ctx.Done():
    // Client disconnected
    logging.Debugf("Request cancelled by client")
    return
}
```

### Step 7: Add Configuration Options
Add to config.Endpoint:
```go
type Endpoint struct {
    Url            string            `json:"url"`
    Headers        map[string]string `json:"headers"`
    MaxRequestSize int64             `json:"maxRequestSize,omitempty"` // New field
    RequestTimeout time.Duration     `json:"requestTimeout,omitempty"` // New field
}
```

### Step 8: Create Custom Error Types
```go
type RequestTooLargeError struct {
    Size     int64
    MaxSize  int64
    Endpoint string
}

func (e *RequestTooLargeError) Error() string {
    return fmt.Sprintf("request body %d bytes exceeds limit of %d bytes for endpoint %s",
        e.Size, e.MaxSize, e.Endpoint)
}
```

### Step 9: Update Handler Logic
```go
func (s *Server) createProxyHandler(name string, endpoint config.Endpoint) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Only accept POST requests
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        // Use endpoint-specific or default max size
        maxSize := endpoint.MaxRequestSize
        if maxSize <= 0 {
            maxSize = defaultMaxRequestSize // e.g., 10MB
        }

        // Create context with timeout if configured
        ctx := r.Context()
        if endpoint.RequestTimeout > 0 {
            var cancel context.CancelFunc
            ctx, cancel = context.WithTimeout(ctx, endpoint.RequestTimeout)
            defer cancel()
        }

        // Create the upstream request with streaming
        upstreamReq, err := s.createUpstreamRequest(ctx, r, endpoint, maxSize)
        if err != nil {
            var sizeErr *RequestTooLargeError
            if errors.As(err, &sizeErr) {
                logging.Printf("Request too large for endpoint '%s': %v", name, err)
                http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
            } else {
                logging.Printf("Error creating upstream request for endpoint '%s': %v", name, err)
                http.Error(w, "Internal server error", http.StatusInternalServerError)
            }
            return
        }

        // Rest of the handler...
    }
}
```

### Step 10: Testing Strategy

#### Unit Tests to Add:

1. **Streaming Body Test** - Verify large bodies don't consume excessive memory

2. **Context Cancellation Test** - Verify upstream requests cancel when client disconnects

3. **Header Filtering Test** - Verify hop-by-hop headers are properly removed

4. **Size Limit Test** - Verify 413 response for oversized requests

5. **Memory Leak Test** - Verify resources are cleaned up after requests

### Additional Considerations

#### Edge Cases to Handle:

1. **Chunked Transfer Encoding** - Client sends chunked body without Content-Length

2. **Early Client Disconnect** - Client disconnects during upload

3. **Slow Upstream** - Upstream takes longer to respond than client timeout

4. **Compressed Bodies** - Handle Content-Encoding properly

5. **Multiple Values per Header** - Preserve all values for non-hop-by-hop headers

#### Performance Metrics to Track:

- Memory usage per request (should stay constant)

- Latency impact (should improve for large requests)

- Concurrent request handling (no resource contention)

- Connection reuse efficiency

#### Monitoring Points:

- Log when requests exceed size thresholds

- Track cancellation rates

- Monitor upstream connection pool usage

- Alert on memory usage anomalies
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Fixed proxy handler streaming and context handling issues:

1. **Implemented streaming request bodies** - Removed io.ReadAll buffering, now streams requests directly using r.Body with io.LimitReader for size limits.

2. **Added context propagation** - Upstream requests now use http.NewRequestWithContext with client context, enabling proper cancellation and timeout handling.

3. **Implemented RFC-compliant header filtering** - Created filterHopByHopHeaders function that removes hop-by-hop headers (Connection, Keep-Alive, etc.) per RFC 2616.

4. **Added per-endpoint configuration** - Extended Endpoint struct with Timeout and MaxBodySize fields for fine-grained control.

5. **Enhanced error handling** - Added RequestTooLargeError type and proper error responses (413 for oversized requests, 504 for timeouts, 508 for cancellation).

6. **Maintained backward compatibility** - All new fields are optional with sensible defaults, existing configurations continue to work.

The implementation successfully addresses all acceptance criteria from the original task and includes comprehensive test coverage for all new functionality.
<!-- SECTION:NOTES:END -->
