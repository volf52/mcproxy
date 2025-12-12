# Stdio MCP Endpoints

mcproxy now supports stdio-based MCP (Model Context Protocol) endpoints in addition to HTTP endpoints. This allows mcproxy to act as a bridge between HTTP clients and local MCP server processes.

## Overview

A stdio endpoint in mcproxy:

1. Starts a local MCP server process
2. Communicates with the process via stdin/stdout using JSON-RPC 2.0
3. Translates HTTP requests to MCP method calls
4. Translates MCP responses back to HTTP responses

## Configuration

### Basic Structure

```jsonc
{
  "endpoints": {
    "my-stdio-endpoint": {
      "type": "stdio",
      "command": ["/path/to/mcp-server", "--arg1", "value1"],
      "env": {
        "VAR_NAME": "value"
      },
      "args": ["init-arg-1", "init-arg-2"],
      "headers": {
        "X-Custom-Header": "value"
      },
      "timeout": "30s",
      "maxBodySize": 5242880
    }
  }
}
```

### Fields

- **type** (required): Must be `"stdio"` for stdio endpoints
- **command** (required): Array of command and arguments to execute
- **env** (optional): Environment variables to set for the process
- **args** (optional): Arguments passed to MCP during initialization
- **headers** (optional): HTTP headers to add/override in requests
- **timeout** (optional): Per-endpoint timeout (overrides global)
- **maxBodySize** (optional): Max request body size in bytes

## Request Mapping

HTTP requests are mapped to MCP calls based on:

### 1. X-MCP-Method Header (Explicit)

```bash
# List tools
curl -X POST http://localhost:8099/mcp/endpoint \
  -H "X-MCP-Method: tools/list" \
  -d '{}'

# Call a tool
curl -X POST http://localhost:8099/mcp/endpoint \
  -H "X-MCP-Method: tools/call" \
  -d '{"name": "my_tool", "arguments": {"param": "value"}}'
```

### 2. Path-Based Mapping

```bash
# List tools
curl -X POST http://localhost:8099/mcp/endpoint/tools/list

# Call a tool
curl -X POST http://localhost:8099/mcp/endpoint/tools/call \
  -d '{"name": "my_tool", "arguments": {"param": "value"}}'

# List resources
curl -X POST http://localhost:8099/mcp/endpoint/resources/list

# Read a resource
curl -X POST http://localhost:8099/mcp/endpoint/resources/read \
  -d '{"uri": "file:///path/to/file"}'

# List prompts
curl -X POST http://localhost:8099/mcp/endpoint/prompts/list

# Get a prompt
curl -X POST http://localhost:8099/mcp/endpoint/prompts/get \
  -d '{"name": "my_prompt", "arguments": {"var": "value"}}'
```

### 3. JSON-RPC Pass-through

```bash
# Direct JSON-RPC request
curl -X POST http://localhost:8099/mcp/endpoint \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "my_tool",
      "arguments": {"param": "value"}
    },
    "id": 1
  }'
```

## Supported MCP Methods

- `tools/list` - List available tools
- `tools/call` - Execute a tool
- `resources/list` - List available resources
- `resources/read` - Read a resource's contents
- `prompts/list` - List available prompts
- `prompts/get` - Get a prompt with arguments

## Process Management

mcproxy includes robust process management for stdio endpoints:

- **Auto-restart**: Failed processes are automatically restarted (up to 5 attempts)
- **Graceful shutdown**: Processes are properly terminated on server shutdown
- **Health monitoring**: Process health is continuously monitored
- **Resource limits**: Process memory and file handle limits are enforced
- **Stderr logging**: Process stderr output is captured and logged

## Example: Filesystem MCP Server

### Configuration

```jsonc
{
  "endpoints": {
    "fs": {
      "type": "stdio",
      "command": ["/usr/local/bin/mcp-filesystem-server", "/shared/data"],
      "env": {
        "LOG_LEVEL": "info",
        "FS_READ_ONLY": "true"
      },
      "timeout": "30s",
      "maxBodySize": 10485760
    }
  }
}
```

### Usage

```bash
# List available files
curl -X POST http://localhost:8099/mcp/fs/resources/list

# Read a file
curl -X POST http://localhost:8099/mcp/fs/resources/read \
  -d '{"uri": "file:///shared/data/example.txt"}'

# Use the search tool
curl -X POST http://localhost:8099/mcp/fs/tools/call \
  -d '{
    "name": "search",
    "arguments": {
      "query": "TODO",
      "path": "/shared/data"
    }
  }'
```

## Security Considerations

1. **Command Validation**: Commands in config are validated for security
2. **Resource Isolation**: Each endpoint runs in its own process
3. **Size Limits**: Request/response sizes are limited
4. **No Shell Injection**: Commands are executed directly without shell

## Troubleshooting

### Process Fails to Start

Check:

- Command path is correct and executable
- All required arguments are provided
- Environment variables are properly formatted
- Process has permission to access required resources

### Requests Timeout

- Increase the `timeout` value in configuration
- Check if the MCP server is handling requests properly
- Review stderr logs for error messages

### Process Keeps Restarting

- Check stderr logs for crash information
- Verify the MCP server is compatible with JSON-RPC 2.0
- Ensure all required dependencies are installed

## Migration from HTTP

To migrate an existing HTTP endpoint to stdio:

1. Change `"type"` from `"http"` to `"stdio"`
2. Replace `"url"` with `"command"` array
3. Add any required `"env"` variables
4. Add `"args"` for initialization if needed
5. Test with the new endpoint path

The rest of the configuration (headers, timeout, etc.) remains the same.
