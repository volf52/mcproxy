# mcproxy

A lightweight Go service that publishes dynamic HTTP POST endpoints and proxies them to configured upstream HTTP/HTTPS targets. Configuration and secrets are JSON-based; header values can reference secrets using `{{ var_name }}` placeholders.

## Quick Start
- Prerequisites: Go 1.21+.
- Defaults (auto-detection prioritizes JSONC):
  - Config file: `~/.config/mcproxy/config.jsonc` → `~/.config/mcproxy/config.json` → `./config.jsonc` → `./config.json` (override with env `MCPROXY_CONFIG`).
  - Secrets file: `~/secrets.jsonc` → `~/secrets.json` (override with env `MCPROXY_SECRETS`).
  - Listen address: `:8099`.
- Build and run:
```bash
go build -o mcproxy
./mcproxy
```

## Configuration

The service uses JSON configuration files with support for **JSONC (JSON with Comments)** for better documentation and maintainability. The service automatically detects file extensions and uses the appropriate parser.

### File Detection Priority

The service automatically detects and prioritizes JSONC files:

- **Global config**: `~/config.jsonc` (preferred) → `~/config.json` (fallback)
- **Global secrets**: `~/secrets.jsonc` (preferred) → `~/secrets.json` (fallback)
- **Project config**: `~/.config/mcproxy/config.jsonc` → `~/.config/mcproxy/config.json` → `./config.jsonc` → `./config.json` (fallback)
- **Project secrets**: `./secrets.jsonc` (preferred) → `./secrets.json` (fallback)

JSONC supports both single-line (`//`) and multi-line (`/* */`) comments, making configuration files self-documenting.

### JSON Schema

The project includes a dynamically generated JSON Schema (`config.schema.json`) that provides:

- **IDE Autocomplete**: Use in VSCode, IntelliJ, etc. for intelligent suggestions
- **Validation**: Automatic validation in CI/CD and development environments
- **Documentation**: Self-documenting configuration structure

Generate schema:
```bash
./scripts/generate-schema.sh
# or
go run cmd/generate-schema/main.go
```

### Secrets (`~/secrets.jsonc` by default)

**JSONC format (recommended) - `~/secrets.jsonc`:**
```jsonc
{
  // API authentication token for billing service
  "api_token": "super-secret-token",

  // Slack webhook verification secret
  "webhook_secret": "whsec_1234567890",

  /* Slack signing secret for request validation
     Used to verify incoming webhook requests */
  "slack_signing": "v1_a1b2c3d4e5f6"
}
```

**JSON format - `~/secrets.json`:**
```json
{
  "api_token": "super-secret-token",
  "webhook_secret": "whsec_1234567890",
  "slack_signing": "v1_a1b2c3d4e5f6"
}
```

### Config (`~/.config/mcproxy/config.jsonc` by default)

**JSONC format (recommended) - `~/.config/mcproxy/config.jsonc`:**
```jsonc
{
  // Use JSON Schema for validation and IDE autocomplete
  "$schema": "./config.schema.json",

  "endpoints": {
    // Billing service endpoint - processes payment charges
    "billing": {
      "upstreamUrl": "https://billing.internal.local/v1/charge",
      "headers": {
        "Authorization": "Bearer {{ api_token }}", // Token from secrets file
        "Content-Type": "application/json"
      }
    },

    // Slack webhook endpoint - forwards messages to Slack channel
    "slack-webhook": {
      "upstreamUrl": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
      "headers": {
        "Content-Type": "application/json" // Slack expects JSON payloads
      }
    }
  },

  /* Log file path for structured logging
     Leave empty/unspecified to log to stdout only */
  "logFile": "./logs/mcproxy.log"
}
```

**JSON format - `~/.config/mcproxy/config.json` or `./config.json` for backward compatibility:**
```json
{
  "$schema": "./config.schema.json",
  "endpoints": {
    "billing": {
      "upstreamUrl": "https://billing.internal.local/v1/charge",
      "headers": {
        "Authorization": "Bearer {{ api_token }}",
        "Content-Type": "application/json"
      }
    },
    "slack-webhook": {
      "upstreamUrl": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
      "headers": {
        "Content-Type": "application/json"
      }
    }
  },
  "logFile": "./logs/mcproxy.log"
}
```

### Configuration Fields

- **`endpoints`**: Map of endpoint configurations (required)
  - Key: Endpoint name (becomes `POST /{name}` route)
  - `upstreamUrl`: Target URL for proxying (required)
  - `headers`: Additional headers with secret templating support
- **`logFile`**: Optional file path for logging output

## Development

### Build and Run
```bash
go build -o mcproxy              # Build the binary
./mcproxy                        # Run the service
go run ./cmd/mcproxy             # Run without building
```

### Testing
```bash
go test ./...                    # Run all tests
go test ./... -run TestName      # Run specific test by name
go test ./pkg/config -v          # Run tests for specific package with verbose output
go test -race -coverprofile=coverage.out ./...  # Run with race detection and coverage
```

### Code Quality
```bash
go vet ./...                     # Static analysis
gofmt -s -w .                    # Format code
go fmt ./...                     # Check formatting
go mod download                  # Download dependencies
go mod tidy                       # Clean up dependencies
```

### Schema Generation
```bash
./scripts/generate-schema.sh     # Generate JSON Schema from Go structs
go run cmd/generate-schema/main.go  # Direct schema generation
```

## CI/CD

The project includes GitHub Actions for automated checks:

### CI Workflow (`.github/workflows/ci.yml`)
Runs automatically on:
- Push to `main` or `dev` branches
- Manual trigger from GitHub Actions UI

**Checks performed:**
- Code formatting with `gofmt`
- Static analysis with `go vet`
- Unit tests with race detection
- Test coverage reporting
- JSON Schema generation and auto-commit if changed

### Update Schema Workflow (`.github/workflows/update-schema.yml`)
Manual workflow to update JSON Schema:
- Can be triggered from GitHub Actions tab
- Option to control whether to commit changes
- Shows diff of schema changes
- Generates summary of updates

**Auto-commit behavior:**
- Only commits schema changes on pushes (not PRs)
- Uses conventional commit format with emoji
- Includes attribution and timestamp
- Prevents CI failures from schema drift

## Behavior

- **Startup**: Fails fast if config or secrets files are missing/invalid
- **Endpoints**: Each endpoint registers a POST handler at `/{name}`
- **Secret Templating**: Header values support `{{ secret_name }}` substitution from secrets file
- **HTTP Client**: Shared client with connection reuse and TLS support for HTTPS
- **Logging**: Structured logs to stdout; optional file output when `logFile` is configured

## Docs
- Project description: `docs/project-description.md`
- Product requirements: `docs/prd.md`
