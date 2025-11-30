# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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

The service requires two JSON files:
- **Config file**: `~/.config/mcproxy/config.jsonc` (XDG-compliant, override with `MCPROXY_CONFIG` env var)
- **Secrets file**: `~/secrets.jsonc` (override with `MCPROXY_SECRETS` env var)

Both files support JSONC format (JSON with comments) and fall back to .json extensions.

Default listen address: `:8099`

## Architecture

The service consists of several key components:

### Core Components
- **Configuration Loader**: Loads and validates JSON config and secrets files from configurable paths
- **Template Resolver**: Substitutes `{{ var_name }}` placeholders in header values using secrets map
- **Proxy Registry**: Registers POST handlers for each valid endpoint at `/{name}`
- **HTTP Client**: Shared client with connection reuse, timeouts, and TLS support for HTTPS upstreams
- **Structured Logging**: Emits logs to stdout with optional file output

### Request Flow
1. Service starts and loads config/secrets files
2. For each valid MCP entry, registers a POST handler at `/{endpoint_name}`
3. Incoming POST requests are forwarded to the configured upstream URL
4. Headers are merged: incoming headers → configured headers (with secret substitution)
5. Response (status, headers, body) is streamed back to caller

### Error Handling
- Startup fails fast if config or secrets files are missing/invalid
- Endpoints with unresolved secret variables are skipped with warning logged
- Service continues operating with valid endpoints even if some are skipped

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
