---
id: task-00018
title: Fix startup logging to use structured secure logger
status: Done
assignee: []
created_date: '2025-12-11 19:55'
updated_date: '2025-12-12 19:48'
labels:
  - bug
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Startup logs in cmd/mcproxy/main.go:43-52 use fmt.Printf to print upstream URLs and headers instead of structured logging. This can leak environment details and ignores the project's logging guidance.

The main.go file contains multiple fmt.Printf statements that bypass the secure logging system:

1. Lines 14, 25-31: Basic startup information using fmt.Printf/fmt.Println
2. Lines 36-40: Endpoint processing statistics
3. Lines 43-52: CRITICAL - Direct logging of endpoint URLs and headers that may contain secrets
4. Lines 56-63: Warning messages about no endpoints
5. Line 76: Server start message

This creates a security vulnerability where:

- Secret templates in headers ({{api_key}}) are exposed in plain text
- Actual resolved secret values may be leaked
- Structured logging benefits are lost (no log levels, no sanitization)
- Inconsistent with the proxy/server.go which properly uses logging.Printf

The project has a comprehensive secure logging system at pkg/logging/secure_logger.go with:

- Multiple debug levels (off, sanitized, partial, unredacted)
- Automatic secret detection and redaction
- Template pattern matching
- Structured logging functions (Info, Warning, Error, Debug, Printf)
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria

<!-- AC:BEGIN -->

- [ ] #1 Replace ALL fmt.Printf/fmt.Println statements with appropriate logging package calls
- [ ] #2 Ensure startup endpoint information uses logging.Printf for automatic secret sanitization
- [ ] #3 Add structured log levels: Info for normal startup, Warning for non-fatal issues, Error for failures
- [ ] #4 Preserve all existing output information but use secure logging channels
- [ ] #5 Verify no secret templates or values can leak in startup logs
- [ ] #6 Maintain readability of startup information while ensuring security
- [ ] #7 Ensure consistency with proxy/server.go logging patterns
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
### 1. Replace Basic Startup Messages (Lines 14-31)

**Current code:**

```go
fmt.Println("Starting mcproxy...")
fmt.Printf("Loaded configuration with %d endpoints\n", len(cfg.Endpoints))
fmt.Printf("Loaded %d secrets\n", len(secrets))
if logging.IsDebugMode() {
    fmt.Println("Debug mode enabled")
} else {
    fmt.Println("Production mode")
}
```

**Replacement:**

```go
logging.Info("Starting mcproxy")
logging.Printf("Loaded configuration with %d endpoints", len(cfg.Endpoints))
logging.Printf("Loaded %d secrets", len(secrets))
if logging.IsDebugMode() {
    logging.Debug("Debug mode enabled")
} else {
    logging.Info("Production mode")
}
```

### 2. Replace Endpoint Processing Statistics (Lines 36-40)

**Current code:**

```go
fmt.Printf("Processed %d endpoints with secret templates\n", len(processedEndpoints))
if len(skipped) > 0 {
    fmt.Printf("Skipped %d endpoints due to missing secrets: %v\n", len(skipped), skipped)
}
```

**Replacement:**

```go
logging.Printf("Processed %d endpoints with secret templates", len(processedEndpoints))
if len(skipped) > 0 {
    logging.Warning("Skipped %d endpoints due to missing secrets: %v", len(skipped), skipped)
}
```

### 3. CRITICAL: Replace Endpoint Information Loop (Lines 43-52)

**Current code (SECURITY ISSUE):**

```go
for name, endpoint := range processedEndpoints {
    fmt.Printf("Endpoint '%s' -> %s\n", name, endpoint.Url)
    sanitizedHeaders := logging.SanitizeHeadersForLogging(endpoint.Headers)
    for header, value := range sanitizedHeaders {
        fmt.Printf("  %s: %s\n", header, value)
    }
    fmt.Println()
}
```

**Secure replacement:**

```go
for name, endpoint := range processedEndpoints {
    // Use logging.Printf which automatically sanitizes secrets
    logging.Printf("Endpoint '%s' -> %s", name, endpoint.Url)

    // Headers are already sanitized, but use logging.Printf for consistency
    sanitizedHeaders := logging.SanitizeHeadersForLogging(endpoint.Headers)
    for header, value := range sanitizedHeaders {
        logging.Printf("  %s: %s", header, value)
    }
}
```

### 4. Replace No Endpoints Warning (Lines 56-63)

**Current code:**

```go
fmt.Println("Warning: No valid endpoints to serve")
fmt.Println("Please check your configuration files:")
fmt.Println("  - Global config: ~/config.jsonc or ~/config.json")
// ... more config paths
```

**Replacement:**

```go
logging.Warning("No valid endpoints to serve")
logging.Warning("Please check your configuration files:")
logging.Warning("  - Global config: ~/config.jsonc or ~/config.json")
logging.Warning("  - Global secrets: ~/secrets.jsonc or ~/secrets.json")
logging.Warning("  - Project config: ~/.config/mcproxy/config.jsonc (or MCPROXY_CONFIG env var)")
logging.Warning("  - Project secrets: ./.mcproxy/secrets.jsonc (or MCPROXY_SECRETS env var)")
logging.Warning("  - Fallback: ./.mcproxy/config.jsonc and ./.mcproxy/secrets.jsonc in project directory")
```

### 5. Replace Server Start Message (Line 76)

**Current code:**

```go
fmt.Printf("Starting proxy server on port %s\n", port)
```

**Replacement:**

```go
logging.Printf("Starting proxy server on port %s", port)
```

### Security Considerations

1. **Why logging.Printf is crucial**: It automatically calls SanitizeForLogging on all string arguments, ensuring secret templates like `{{api_key}}` are replaced with `[SECRET]`.

2. **Debug level handling**: The logging system respects debug levels:
   - Production mode: Shows `[SECRET]` for all sensitive data
   - Debug sanitized mode: Same as production
   - Debug partial mode: Shows partial secrets like `api***123`
   - Debug unredacted mode: Shows full secrets (admin use only)

3. **Consistency**: This change makes main.go consistent with proxy/server.go which already uses logging.Printf.

### Testing Strategy

1. Verify all log output is properly sanitized in production mode
2. Test with various debug levels to ensure proper behavior
3. Confirm no secret templates leak in startup logs
4. Validate that all information is still visible but secure

### Files to Modify

- `/home/volfy/hobby/mcpproxy/cmd/mcproxy/main.go` - Replace all fmt.Printf/fmt.Println with logging package equivalents

### Dependencies

- Already imported: `"mcproxy/pkg/logging"`
- No additional imports required
<!-- SECTION:NOTES:END -->
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Implementation Plan

### 1. Replace Basic Startup Messages (Lines 14-31)

**Current code:**

```go
fmt.Println("Starting mcproxy...")
fmt.Printf("Loaded configuration with %d endpoints\n", len(cfg.Endpoints))
fmt.Printf("Loaded %d secrets\n", len(secrets))
if logging.IsDebugMode() {
    fmt.Println("Debug mode enabled")
} else {
    fmt.Println("Production mode")
}
```

**Replacement:**

```go
logging.Info("Starting mcproxy")
logging.Printf("Loaded configuration with %d endpoints", len(cfg.Endpoints))
logging.Printf("Loaded %d secrets", len(secrets))
if logging.IsDebugMode() {
    logging.Debug("Debug mode enabled")
} else {
    logging.Info("Production mode")
}
```

### 2. Replace Endpoint Processing Statistics (Lines 36-40)

**Current code:**

```go
fmt.Printf("Processed %d endpoints with secret templates\n", len(processedEndpoints))
if len(skipped) > 0 {
    fmt.Printf("Skipped %d endpoints due to missing secrets: %v\n", len(skipped), skipped)
}
```

**Replacement:**

```go
logging.Printf("Processed %d endpoints with secret templates", len(processedEndpoints))
if len(skipped) > 0 {
    logging.Warning("Skipped %d endpoints due to missing secrets: %v", len(skipped), skipped)
}
```

### 3. CRITICAL: Replace Endpoint Information Loop (Lines 43-52)

**Current code (SECURITY ISSUE):**

```go
for name, endpoint := range processedEndpoints {
    fmt.Printf("Endpoint '%s' -> %s\n", name, endpoint.Url)
    sanitizedHeaders := logging.SanitizeHeadersForLogging(endpoint.Headers)
    for header, value := range sanitizedHeaders {
        fmt.Printf("  %s: %s\n", header, value)
    }
    fmt.Println()
}
```

**Secure replacement:**

```go
for name, endpoint := range processedEndpoints {
    // Use logging.Printf which automatically sanitizes secrets
    logging.Printf("Endpoint '%s' -> %s", name, endpoint.Url)

    // Headers are already sanitized, but use logging.Printf for consistency
    sanitizedHeaders := logging.SanitizeHeadersForLogging(endpoint.Headers)
    for header, value := range sanitizedHeaders {
        logging.Printf("  %s: %s", header, value)
    }
}
```

### 4. Replace No Endpoints Warning (Lines 56-63)

**Current code:**

```go
fmt.Println("Warning: No valid endpoints to serve")
fmt.Println("Please check your configuration files:")
fmt.Println("  - Global config: ~/config.jsonc or ~/config.json")
// ... more config paths
```

**Replacement:**

```go
logging.Warning("No valid endpoints to serve")
logging.Warning("Please check your configuration files:")
logging.Warning("  - Global config: ~/config.jsonc or ~/config.json")
logging.Warning("  - Global secrets: ~/secrets.jsonc or ~/secrets.json")
logging.Warning("  - Project config: ~/.config/mcproxy/config.jsonc (or MCPROXY_CONFIG env var)")
logging.Warning("  - Project secrets: ./.mcproxy/secrets.jsonc (or MCPROXY_SECRETS env var)")
logging.Warning("  - Fallback: ./.mcproxy/config.jsonc and ./.mcproxy/secrets.jsonc in project directory")
```

### 5. Replace Server Start Message (Line 76)

**Current code:**

```go
fmt.Printf("Starting proxy server on port %s\n", port)
```

**Replacement:**

```go
logging.Printf("Starting proxy server on port %s", port)
```

### Security Considerations

1. **Why logging.Printf is crucial**: It automatically calls SanitizeForLogging on all string arguments, ensuring secret templates like `{{api_key}}` are replaced with `[SECRET]`.

2. **Debug level handling**: The logging system respects debug levels:
   - Production mode: Shows `[SECRET]` for all sensitive data
   - Debug sanitized mode: Same as production
   - Debug partial mode: Shows partial secrets like `api***123`
   - Debug unredacted mode: Shows full secrets (admin use only)

3. **Consistency**: This change makes main.go consistent with proxy/server.go which already uses logging.Printf.

### Testing Strategy

1. Verify all log output is properly sanitized in production mode
2. Test with various debug levels to ensure proper behavior
3. Confirm no secret templates leak in startup logs
4. Validate that all information is still visible but secure

### Files to Modify

- `/home/volfy/hobby/mcpproxy/cmd/mcproxy/main.go` - Replace all fmt.Printf/fmt.Println with logging package equivalents

### Dependencies

- Already imported: `"mcproxy/pkg/logging"`
- No additional imports required

Task completed successfully.

Replaced all fmt.Printf/fmt.Println statements with secure logging calls

Prevents secret template exposure during application startup

Code reviewed by Go expert and all tests passing
<!-- SECTION:NOTES:END -->
