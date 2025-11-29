# JSONC Configuration Guide

This guide explains how to use JSONC (JSON with Comments) for mcproxy configuration files to improve maintainability and documentation.

## What is JSONC?

JSONC is JSON with support for comments, making configuration files more self-documenting and maintainable. mcproxy automatically detects `.jsonc` files and uses the appropriate parser.

## Supported Comment Types

### Single-line Comments
```jsonc
{
  "api_token": "secret123", // This is an inline comment
  "database_url": "https://db.example.com" // Another inline comment
}

// This is a single-line comment that explains the next section
"endpoints": {
  // Nested comments are also supported
}
```

### Multi-line Comments
```jsonc
{
  /* This is a multi-line comment that
     can span multiple lines and is useful
     for detailed explanations */
  "api_token": "secret123",

  "endpoints": {
    /* Multi-line comments can also be used inline
       for complex configuration explanations */
    "webhook": {
      "url": "https://example.com/webhook"
    }
  }
}
```

## File Detection Priority

mcproxy automatically prioritizes JSONC files:

1. **Global config**: `~/config.jsonc` (preferred) → `~/config.json` (fallback)
2. **Global secrets**: `~/secrets.jsonc` (preferred) → `~/secrets.json` (fallback)
3. **Project config**: `./config.jsonc` (preferred) → `./config.json` (fallback)
4. **Project secrets**: `./secrets.jsonc` (preferred) → `./secrets.json` (fallback)

## Migration from JSON to JSONC

Converting existing JSON files to JSONC is straightforward:

### Before (`config.json`)
```json
{
  "endpoints": {
    "billing": {
      "upstreamUrl": "https://billing.internal.local/v1/charge",
      "headers": {
        "Authorization": "Bearer {{ api_token }}",
        "Content-Type": "application/json"
      }
    }
  },
  "logFile": "./logs/mcproxy.log"
}
```

### After (`config.jsonc`)
```jsonc
{
  // Main endpoint configuration
  "endpoints": {
    // Billing service endpoint - processes payment charges
    "billing": {
      "upstreamUrl": "https://billing.internal.local/v1/charge",
      "headers": {
        "Authorization": "Bearer {{ api_token }}", // From secrets file
        "Content-Type": "application/json"
      }
    }
  },

  // Optional: Path to log file for structured logging
  "logFile": "./logs/mcproxy.log"
}
```

## Best Practices

### 1. Document Complex Configurations
```jsonc
{
  "endpoints": {
    /*
     * External API Integration
     *
     * This endpoint connects to our payment processor.
     * Important notes:
     * - Timeout is set to 30 seconds
     * - Retry logic is handled by the upstream service
     * - All requests are logged for audit purposes
     */
    "payment-processor": {
      "upstreamUrl": "https://api.paymentprovider.com/v2/process",
      "headers": {
        // Authentication header uses API key from secrets
        "Authorization": "Bearer {{ payment_api_key }}",

        // Required content type for this API
        "Content-Type": "application/json",

        // Request ID tracking for debugging
        "X-Request-Source": "mcproxy-production"
      }
    }
  }
}
```

### 2. Comment Secret Templates
```jsonc
{
  "endpoints": {
    "slack-webhook": {
      "upstreamUrl": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
      "headers": {
        // Using {{slack_webhook_secret}} from secrets file for authentication
        "Authorization": "Bearer {{ slack_webhook_secret }}",

        // Slack expects JSON payloads
        "Content-Type": "application/json"
      }
    },

    "internal-api": {
      "upstreamUrl": "https://api.internal.company.com/v1/data",
      "headers": {
        // Internal service authentication
        "X-API-Key": "{{ internal_api_key }}", // From secrets file

        // Service identification for logging
        "X-Service-Name": "mcproxy",

        // Version of the client making the request
        "X-Client-Version": "1.0.0"
      }
    }
  }
}
```

### 3. Environment-Specific Comments
```jsonc
{
  // === DEVELOPMENT ENVIRONMENT CONFIGURATION ===
  // This configuration is optimized for local development
  // and testing environments.

  "endpoints": {
    // Local development services
    "dev-api": {
      "upstreamUrl": "http://localhost:8080/api", // Local dev server
      "headers": {
        "Authorization": "Bearer {{ dev_api_key }}" // Dev environment key
      }
    }
  },

  // Development logging - verbose output to console
  "logFile": "", // Empty string means stdout only
  "debug": true // Enable debug logging for development
}
```

### 4. Hierarchical Configuration Comments
When using hierarchical configuration (global + project overrides), use comments to clarify the inheritance:

**Global Config (`~/config.jsonc`):**
```jsonc
{
  // === GLOBAL CONFIGURATION ===
  // These settings are inherited by all projects unless overridden

  "endpoints": {
    // Shared service - available to all projects
    "shared-auth": {
      "upstreamUrl": "https://auth.company.com/v1/validate",
      "headers": {
        "X-Global-API-Key": "{{ global_auth_key }}" // Shared auth key
      }
    }
  },

  // Global logging configuration
  "logFile": "/var/log/mcproxy/global.log"
}
```

**Project Config (`./config.jsonc`):**
```jsonc
{
  // === PROJECT-SPECIFIC CONFIGURATION ===
  // These settings override or extend global configuration

  "endpoints": {
    // Project-specific service - overrides global if same name exists
    "project-api": {
      "upstreamUrl": "https://project-specific.company.com/api",
      "headers": {
        "Authorization": "Bearer {{ project_api_key }}" // Project-specific auth
      }
    }
  },

  // Project logging - overrides global log file location
  "logFile": "./logs/project.log" // Project-local log file
}
```

## Performance Considerations

- JSONC parsing is nearly as fast as standard JSON parsing
- Comments are stripped during parsing and don't affect runtime performance
- File detection adds minimal overhead (simple file existence check)
- The service maintains backward compatibility with existing `.json` files

## IDE Support

Most modern IDEs support JSONC syntax highlighting:

- **VS Code**: Built-in JSONC support with `.jsonc` extension
- **IntelliJ IDEA**: JSON with Comments plugin
- **Vim/Neovim**: JSONC syntax highlighting plugins available
- **Emacs**: JSONC mode packages available

## Troubleshooting

### Common Issues

1. **Unterminated multi-line comments**
   ```jsonc
   /* This comment is not properly closed
   "invalid": "json"
   ```
   **Fix**: Ensure all `/*` have matching `*/`

2. **Invalid comment placement**
   ```jsonc
   {
     "key": /* inline multi-line comments should be avoided */ "value"
   }
   ```
   **Fix**: Place multi-line comments on separate lines

3. **JSON Syntax Errors**
   JSONC still requires valid JSON syntax. Comments are ignored for structure validation.

### Validation

Use the generated JSON Schema for validation:
```bash
go run cmd/generate-schema/main.go
# Validate your .jsonc files against the generated schema
```

## Examples Repository

For more examples, check the project documentation:
- `README.md` - Basic configuration examples
- Configuration schema: `config.schema.json` - Generated from Go structs