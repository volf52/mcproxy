## Project Overview

mcproxy is a lightweight Go service that acts as an HTTP proxy layer, exposing dynamic POST endpoints that forward requests to configured upstream HTTP/HTTPS targets. The service loads configuration from JSON files and supports secret templating in header values using `{{ var_name }}` placeholders.

## Development Commands

### Build and Run

```bash
go build -o mcproxy              # Build the binary
./mcproxy                        # Run the service
```

### Testing

```bash
go test ./...                    # Run all tests
go test ./... -run TestName      # Run specific test by name
go test ./pkg/config -v          # Run tests for specific package with verbose output
```

### Code Quality

```bash
go vet ./...                     # Static analysis
gofmt -w .                       # Format code (or use goimports)
```

## Configuration

The service loads configuration from multiple locations in priority order:

### Project-specific Configuration (highest priority)

- **Config file**: `./.mcproxy/config.jsonc` (override with `MCPROXY_CONFIG` env var)
- **Secrets file**: `./.mcproxy/secrets.jsonc` (override with `MCPROXY_SECRETS` env var)

### Global Configuration

- **Config file**: `~/.config/mcproxy/config.jsonc` (XDG-compliant)
- **Secrets file**: `~/secrets.jsonc`

Both files support JSONC format (JSON with comments) and fall back to .json extensions.

Default listen address: `:8099`

### Configuration Options

```json
{
  "endpoints": {
    "http-endpoint": {
      "type": "http",
      "url": "https://api.example.com/webhook",
      "headers": {
        "Authorization": "Bearer {{ API_TOKEN }}",
        "Content-Type": "application/json"
      },
      "timeout": "30s",         // Optional: Per-endpoint timeout
      "maxBodySize": 5242880    // Optional: Max request body size in bytes (5MB)
    },
    "stdio-endpoint": {
      "type": "stdio",
      "command": ["/usr/local/bin/mcp-server", "--option", "value"],
      "env": {
        "API_KEY": "{{ API_TOKEN }}",
        "LOG_LEVEL": "debug"
      },
      "args": ["init-arg-1"],   // Optional: Arguments passed to MCP during initialization
      "timeout": "30s",
      "maxBodySize": 5242880
    }
  },
  "globalTimeout": "60s",       // Optional: Global timeout for all endpoints
  "globalMaxBodySize": 10485760, // Optional: Global max body size in bytes (10MB)
  "logFile": "/var/log/mcproxy.log",
  "server": {                   // Optional: Server configuration
    "readTimeout": 30,          // Optional: Max duration for reading request (seconds, default: 30)
    "writeTimeout": 30,         // Optional: Max duration for writing response (seconds, default: 30)
    "idleTimeout": 120,         // Optional: Max time for keep-alive connections (seconds, default: 120)
    "shutdownTimeout": 30       // Optional: Max time for graceful shutdown (seconds, default: 30)
  }
}
```

#### Endpoint Types

mcproxy supports two endpoint types:

##### HTTP Endpoints (type: "http")

Forwards HTTP requests to an upstream HTTP/HTTPS server.

**Fields:**

- **type** (optional): Set to "http" (default if not specified)
- **url** (required): Upstream server URL
- **headers** (optional): Custom headers to add to requests. Supports secret templating with `{{ VAR_NAME }}`
- **timeout** (optional): Per-endpoint timeout in duration format (e.g., "30s", "1m")
- **maxBodySize** (optional): Maximum request body size in bytes (e.g., 5242880 for 5MB)

##### Stdio MCP Endpoints (type: "stdio")

Starts a local MCP server process and translates HTTP requests to JSON-RPC calls.

**Fields:**

- **type** (required): Must be "stdio"
- **command** (required): Command and arguments to execute
- **env** (optional): Environment variables (supports secret templating)
- **args** (optional): Initialization arguments for MCP connection
- **headers** (optional): HTTP headers to add/override
- **timeout** (optional): Per-endpoint timeout
- **maxBodySize** (optional): Maximum request body size

**Common fields for both types:**

- **timeout** (optional): Per-endpoint timeout in duration format (e.g., "30s", "1m")
- **maxBodySize** (optional): Maximum request body size in bytes (e.g., 5242880 for 5MB)
- **headers** (optional): Custom headers (supports secret templating)

#### Global Configuration Fields

- **globalTimeout** (optional): Default timeout for all endpoints (default: 60s)
- **globalMaxBodySize** (optional): Default max body size for all endpoints (default: 10MB)
- **logFile** (optional): Path to log file for structured logging output

#### Notes

- Per-endpoint settings override global settings
- Request bodies are streamed without buffering in memory
- Hop-by-hop headers (Connection, Keep-Alive, etc.) are automatically filtered
- Size limits return HTTP 413 Payload Too Large when exceeded
- Timeouts return HTTP 504 Gateway Timeout

## Environment Variables

- **MCPROXY_CONFIG**: Path to the configuration file. Overrides default search paths (both .mcproxy/ and XDG).
- **MCPROXY_SECRETS**: Path to the secrets file. Overrides default search paths (both .mcproxy/ and home directory).
- **MCPROXY_PORT**: Port for the HTTP server to listen on. Supports both "8099" and ":8099" formats. Defaults to ":8099" if not set.

### Server Timeout Environment Variables

- **MCPROXY_READ_TIMEOUT**: Maximum duration for reading the entire request, including the body (seconds, default: 30)
- **MCPROXY_WRITE_TIMEOUT**: Maximum duration before timing out writes of the response (seconds, default: 30)
- **MCPROXY_IDLE_TIMEOUT**: Maximum amount of time to wait for the next request when keep-alives are enabled (seconds, default: 120)
- **MCPROXY_SHUTDOWN_TIMEOUT**: Maximum time to wait for graceful shutdown (seconds, default: 30)

## Architecture

The service consists of several key components:

### Core Components

- **Configuration Loader**: Loads and validates JSON config and secrets files from configurable paths
- **Template Resolver**: Substitutes `{{ var_name }}` placeholders in header values using secrets map
- **Proxy Registry**: Registers POST handlers for each valid endpoint at `/mcp/{name}`
- **HTTP Client**: Shared client with connection reuse, timeouts, and TLS support for HTTPS upstreams
- **Server with Timeouts**: HTTP server with configurable read/write/idle timeouts and graceful shutdown support
- **Structured Logging**: Emits logs to stdout with optional file output

### Request Flow

1. Service starts and loads config/secrets files
2. For each valid MCP entry, registers a POST handler at `/mcp/{endpoint_name}`
3. Incoming POST requests are forwarded to the configured upstream URL
4. Headers are merged: incoming headers → configured headers (with secret substitution)
5. Response (status, headers, body) is streamed back to caller

### Error Handling

- Startup fails fast if config or secrets files are missing/invalid
- Endpoints with unresolved secret variables are skipped with warning logged
- Service continues operating with valid endpoints even if some are skipped

### Graceful Shutdown

The server supports graceful shutdown when receiving SIGINT or SIGTERM signals:

- In-flight requests are allowed to complete before shutdown
- New requests during shutdown receive HTTP 503 Service Unavailable
- Server waits up to the configured shutdown timeout before forcing exit
- All connections are properly closed to prevent resource leaks

## Code Style Guidelines

- Use `gofmt` or `goimports` for formatting (no manual formatting)
- Group imports: stdlib, blank line, external deps, blank line, internal packages
- Use CamelCase for exports, lowercase for unexported
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Use structured logging with key-value pairs
- Never log secret values

## Project Structure Notes

- Go module: `mcproxy` with Go 1.25.4 requirement
- No external dependencies in go.mod (uses stdlib only)
- Configuration files are gitignored for security
- No existing Go source files yet - this is a new project setup

<!-- BACKLOG.MD MCP GUIDELINES START -->

<CRITICAL_INSTRUCTION>

## BACKLOG WORKFLOW INSTRUCTIONS

This project uses Backlog.md MCP for all task and project management activities.

**CRITICAL GUIDANCE**

- If your client supports MCP resources, read `backlog://workflow/overview` to understand when and how to use Backlog for this project.
- If your client only supports tools or the above request fails, call `backlog.get_workflow_overview()` tool to load the tool-oriented overview (it lists the matching guide tools).

- **First time working here?** Read the overview resource IMMEDIATELY to learn the workflow
- **Already familiar?** You should have the overview cached ("## Backlog.md Overview (MCP)")
- **When to read it**: BEFORE creating tasks, or when you're unsure whether to track work

These guides cover:

- Decision framework for when to create tasks
- Search-first workflow to avoid duplicates
- Links to detailed guides for task creation, execution, and completion
- MCP tools reference

You MUST read the overview resource to understand the complete workflow. The information is NOT summarized here.

</CRITICAL_INSTRUCTION>

<!-- BACKLOG.MD MCP GUIDELINES END -->
