---
id: task-00016
title: Add server timeouts and graceful shutdown
status: Done
assignee: []
created_date: '2025-12-11 19:55'
updated_date: '2025-12-12 14:26'
labels:
  - bug
  - security
  - feature
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The server in pkg/proxy/server.go currently uses http.ListenAndServe with default settings, which creates several security and operational issues:

1. **No timeouts configured**: The server uses Go's default timeouts (ReadTimeout=0, WriteTimeout=0, IdleTimeout=0), making it vulnerable to:
   - Slowloris attacks (clients sending data extremely slowly)
   - Connection exhaustion from idle clients
   - Unclosed connections that consume server resources

2. **No graceful shutdown support**: The server lacks proper shutdown handling, which causes:
   - Abrupt termination of in-flight requests when the process receives SIGINT/SIGTERM
   - Potential data corruption or incomplete responses
   - No ability to drain existing connections before shutdown

3. **Blocking server start**: The server.Start() method blocks forever with no way to implement signal handling or graceful shutdown in the main application.

4. **Missing context handling**: The server doesn't utilize context for request lifecycle management, making it difficult to implement timeouts and cancellation properly.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Server must be configurable with read/write/idle timeouts via environment variables
- [ ] #2 Server must support graceful shutdown on SIGINT and SIGTERM signals
- [ ] #3 In-flight requests must complete during graceful shutdown period
- [ ] #4 Server must use context for proper request lifecycle management
- [ ] #5 Server must return status 503 when shutting down
- [ ] #6 Timeouts must have sensible defaults and prevent resource exhaustion
- [ ] #7 Implementation must include proper error handling and logging
- [ ] #8 Unit tests must cover new functionality including timeout and shutdown scenarios
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
# Implementation Plan for Server Timeouts and Graceful Shutdown

## 1. Configuration Structure Changes

Add timeout configuration to pkg/config/config.go:

```go
type ServerConfig struct {
    // Timeouts in seconds
    ReadTimeout  int `json:"readTimeout,omitempty" description:"Maximum duration for reading the entire request, including the body"`
    WriteTimeout int `json:"writeTimeout,omitempty" description:"Maximum duration before timing out writes of the response"`
    IdleTimeout  int `json:"idleTimeout,omitempty" description:"Maximum amount of time to wait for the next request when keep-alives are enabled"`
    // Graceful shutdown
    ShutdownTimeout int `json:"shutdownTimeout,omitempty" description:"Maximum time to wait for graceful shutdown"`
}

// Extend main Config struct
type Config struct {
    Endpoints map[string]Endpoint `json:"endpoints"`
    LogFile   string              `json:"logFile,omitempty"`
    Server    ServerConfig        `json:"server,omitempty"`
}
```

## 2. Server Implementation Changes

Modify pkg/proxy/server.go to support timeouts and graceful shutdown:

```go
package proxy

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "mcproxy/pkg/config"
    "mcproxy/pkg/logging"
)

type Server struct {
    port       string
    endpoints  map[string]config.Endpoint
    httpClient *http.Client
    server     *http.Server
    shutdownWG sync.WaitGroup
}

func NewServer(port string, endpoints map[string]config.Endpoint, serverConfig config.ServerConfig) *Server {
    return &Server{
        port:      port,
        endpoints: endpoints,
        httpClient: createHTTPClient(),
        server: &http.Server{
            Addr:         port,
            ReadTimeout:  time.Duration(serverConfig.ReadTimeout) * time.Second,
            WriteTimeout: time.Duration(serverConfig.WriteTimeout) * time.Second,
            IdleTimeout:  time.Duration(serverConfig.IdleTimeout) * time.Second,
        },
    }
}

func (s *Server) StartWithShutdown() error {
    // Create context for graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Set up signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Start server in goroutine
    errChan := make(chan error, 1)
    s.shutdownWG.Add(1)
    go func() {
        defer s.shutdownWG.Done()
        logging.Printf("Starting mcproxy server on port %s", s.port)
        errChan <- s.listenAndServe()
    }()

    // Wait for either error or signal
    select {
    case err := <-errChan:
        return err
    case sig := <-sigChan:
        logging.Printf("Received signal %v, initiating graceful shutdown", sig)
        return s.gracefulShutdown(ctx)
    }
}

func (s *Server) listenAndServe() error {
    mux := http.NewServeMux()
    
    // Register a shutdown check middleware
    shutdownMiddleware := func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Check if server is shutting down
            select {
            case <-r.Context().Done():
                http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
                return
            default:
                next.ServeHTTP(w, r)
            }
        })
    }

    // Apply middleware to all handlers
    handler := shutdownMiddleware(mux)

    // Register endpoint handlers
    for name, endpoint := range s.endpoints {
        handlerFunc := s.createProxyHandler(name, endpoint)
        pattern := fmt.Sprintf("/mcp/%s", name)
        mux.HandleFunc(pattern, handlerFunc)
        logging.LogEndpointRegistration(name, endpoint.Url, "/mcp", endpoint.Headers)
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

    s.server.Handler = handler
    return s.server.ListenAndServe()
}

func (s *Server) gracefulShutdown(ctx context.Context) error {
    // Create shutdown context with timeout
    shutdownTimeout := 30 * time.Second // Default
    if s.server != nil {
        // Use configured timeout if available
        shutdownTimeout = time.Duration(30) * time.Second // Will be configurable
    }
    
    shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
    defer cancel()

    logging.Printf("Shutting down server gracefully (timeout: %v)", shutdownTimeout)

    // Shutdown the server
    if err := s.server.Shutdown(shutdownCtx); err != nil {
        logging.Printf("Error during server shutdown: %v", err)
        return err
    }

    // Wait for all handlers to complete
    s.shutdownWG.Wait()

    logging.Printf("Server shutdown complete")
    return nil
}
```

## 3. Main Application Changes

Update cmd/mcproxy/main.go to use the new shutdown functionality:

```go
// Load server configuration from config or use defaults
serverConfig := config.ServerConfig{
    ReadTimeout:  30,  // seconds
    WriteTimeout: 30,  // seconds
    IdleTimeout:  120, // seconds
    ShutdownTimeout: 30, // seconds
}

// Use server config from file if available
if cfg.Server.ReadTimeout > 0 {
    serverConfig.ReadTimeout = cfg.Server.ReadTimeout
}
if cfg.Server.WriteTimeout > 0 {
    serverConfig.WriteTimeout = cfg.Server.WriteTimeout
}
if cfg.Server.IdleTimeout > 0 {
    serverConfig.IdleTimeout = cfg.Server.IdleTimeout
}
if cfg.Server.ShutdownTimeout > 0 {
    serverConfig.ShutdownTimeout = cfg.Server.ShutdownTimeout
}

// Create and start server with shutdown support
server := proxy.NewServer(port, processedEndpoints, serverConfig)
fmt.Printf("Starting proxy server on port %s\n", port)

if err := server.StartWithShutdown(); err != nil {
    log.Fatalf("Server error: %v", err)
}
```

## 4. Environment Variable Support

Add support for environment variables in pkg/config/loader.go:

```go
// Add after loading config:
// Override with environment variables if present
if readTimeout := os.Getenv("MCPROXY_READ_TIMEOUT"); readTimeout != "" {
    if val, err := strconv.Atoi(readTimeout); err == nil && val > 0 {
        config.Server.ReadTimeout = val
    }
}
if writeTimeout := os.Getenv("MCPROXY_WRITE_TIMEOUT"); writeTimeout != "" {
    if val, err := strconv.Atoi(writeTimeout); err == nil && val > 0 {
        config.Server.WriteTimeout = val
    }
}
if idleTimeout := os.Getenv("MCPROXY_IDLE_TIMEOUT"); idleTimeout != "" {
    if val, err := strconv.Atoi(idleTimeout); err == nil && val > 0 {
        config.Server.IdleTimeout = val
    }
}
if shutdownTimeout := os.Getenv("MCPROXY_SHUTDOWN_TIMEOUT"); shutdownTimeout != "" {
    if val, err := strconv.Atoi(shutdownTimeout); err == nil && val > 0 {
        config.Server.ShutdownTimeout = val
    }
}
```

## 5. Unit Tests

Create pkg/proxy/server_shutdown_test.go:

```go
func TestServerShutdown(t *testing.T) {
    // Test graceful shutdown with in-flight requests
    // Test timeout during shutdown
    // Test signal handling
}

func TestServerTimeouts(t *testing.T) {
    // Test read timeout with slow client
    // Test write timeout with slow upstream
    // Test idle timeout with keep-alive
}

func TestServerContextHandling(t *testing.T) {
    // Test request cancellation
    // Test shutdown status response
}
```

## 6. Documentation Updates

Update CLAUDE.md to include new environment variables and configuration options:

```
### Server Configuration
- **MCPROXY_READ_TIMEOUT**: Read timeout in seconds (default: 30)
- **MCPROXY_WRITE_TIMEOUT**: Write timeout in seconds (default: 30)
- **MCPROXY_IDLE_TIMEOUT**: Idle timeout in seconds (default: 120)
- **MCPROXY_SHUTDOWN_TIMEOUT**: Graceful shutdown timeout in seconds (default: 30)
```

## Key Implementation Details:

1. **Timeouts**: Configure sensible defaults that prevent resource exhaustion but allow for normal operation
2. **Signal Handling**: Properly catch SIGINT/SIGTERM and initiate graceful shutdown
3. **In-flight Requests**: Wait for ongoing requests to complete during shutdown
4. **Context Usage**: Use context throughout for proper cancellation and timeout handling
5. **Error Handling**: Log shutdown progress and errors appropriately
6. **Backwards Compatibility**: Ensure existing configurations continue to work

Implementation completed in commit 252039e1 - Added server timeouts and graceful shutdown with comprehensive tests

ServerConfig added with Read/Write/Idle/Shutdown timeout fields

NewServer updated to accept server config

Added StartWithShutdown method for graceful shutdown

Environment variable override support added (MCPROXY_READ_TIMEOUT, etc.)

Comprehensive test suite created in server_shutdown_test.go
<!-- SECTION:NOTES:END -->
