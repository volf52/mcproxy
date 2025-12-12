---
id: task-00020
title: Strengthen endpoint validation
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
## Current State

The endpoint validation in the codebase is minimal and only checks for:
- Non-empty endpoint name
- Non-empty URL

**Validation locations:**
- `pkg/config/config.go:101-116` - `validateEndpoints()` function
- `pkg/config/loader.go:358-374` - `validateMergedConfig()` function (duplicate validation)

**Problems identified:**
1. URLs are not parsed with `url.Parse()` - malformed schemes like `ftp://`, `file://`, or invalid URLs like `not-a-url` will pass validation
2. No restriction on allowed schemes - protocols like `ftp://`, `ws://`, `file://` would be accepted but fail at runtime
3. Header keys are not validated - invalid header names with spaces, control characters, or empty strings pass through
4. Header values are not validated - values with newlines or carriage returns could cause issues
5. Case-insensitive endpoint name collisions are not checked

**Runtime failure locations:**
- `pkg/proxy/server.go:111` - `http.NewRequest()` will fail with invalid URLs
- `pkg/proxy/server.go:125` - `upstreamReq.Header.Set()` may have issues with invalid header names/values
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 URL parsing with proper scheme validation (http/https only)
- [ ] #2 Header key validation to reject invalid characters
- [ ] #3 Header value validation to prevent line breaks
- [ ] #4 Case-insensitive endpoint name collision detection
- [ ] #5 Aggregated error reporting showing all validation issues at once
- [ ] #6 Comprehensive test coverage for all validation scenarios
- [ ] #7 Updated both validateEndpoints and validateMergedConfig functions
- [ ] #8 Clear, actionable error messages that identify which endpoint/has issues
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Detailed Implementation Plan

### 1. Create URL Validation Helper

```go
// validateURL validates that a URL is properly formatted and uses allowed scheme
func validateURL(urlStr string) error {
    if urlStr == "" {
        return fmt.Errorf("URL cannot be empty")
    }
    
    parsed, err := url.Parse(urlStr)
    if err != nil {
        return fmt.Errorf("invalid URL format: %w", err)
    }
    
    // Check scheme
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return fmt.Errorf("URL scheme must be http or https, got: %s", parsed.Scheme)
    }
    
    // Check host is not empty
    if parsed.Host == "" {
        return fmt.Errorf("URL must have a non-empty host")
    }
    
    // Optional: Validate no fragment or query for POST endpoints
    if parsed.Fragment != "" {
        return fmt.Errorf("URL cannot contain fragment (#) component")
    }
    
    return nil
}
```

### 2. Create Header Validation Helper

```go
import "net/textproto"

// validateHeaderKey validates that a header key follows HTTP specifications
func validateHeaderKey(key string) error {
    if key == "" {
        return fmt.Errorf("header key cannot be empty")
    }
    
    // Check for invalid characters that would cause issues
    // Based on RFC 7230, header names should not contain control characters or spaces
    for _, r := range key {
        if r <= ' ' || r == ':' || r == '\x7f' {
            return fmt.Errorf("header key contains invalid character: %q", string(r))
        }
    }
    
    // Optional: Use textproto.CanonicalMIMEHeaderKey to test validity
    canonical := textproto.CanonicalMIMEHeaderKey(key)
    if canonical != key && strings.ContainsAny(key, " \t\r\n") {
        return fmt.Errorf("header key contains whitespace or control characters")
    }
    
    return nil
}

// validateHeaderValue validates that a header value is safe
func validateHeaderValue(value string) error {
    // Header values should not contain newlines or carriage returns
    if strings.ContainsAny(value, "\r\n") {
        return fmt.Errorf("header value contains line break characters")
    }
    
    return nil
}
```

### 3. Update validateEndpoints Function

```go
func validateEndpoints(endpoints map[string]Endpoint) error {
    if len(endpoints) == 0 {
        return fmt.Errorf("at least one endpoint must be defined")
    }
    
    var allErrors []error
    seenNames := make(map[string]bool)
    
    for name, endpoint := range endpoints {
        // Validate endpoint name
        if name == "" {
            allErrors = append(allErrors, fmt.Errorf("endpoint name cannot be empty"))
            continue
        }
        
        // Check for case-insensitive name collisions
        lowerName := strings.ToLower(name)
        if seenNames[lowerName] {
            allErrors = append(allErrors, fmt.Errorf("endpoint name %q conflicts with existing endpoint (case-insensitive comparison)", name))
            continue
        }
        seenNames[lowerName] = true
        
        // Validate URL
        if err := validateURL(endpoint.Url); err != nil {
            allErrors = append(allErrors, fmt.Errorf("endpoint %q: %w", name, err))
            continue
        }
        
        // Validate headers
        for headerKey, headerValue := range endpoint.Headers {
            if err := validateHeaderKey(headerKey); err != nil {
                allErrors = append(allErrors, fmt.Errorf("endpoint %q header key %q: %w", name, headerKey, err))
            }
            
            if err := validateHeaderValue(headerValue); err != nil {
                allErrors = append(allErrors, fmt.Errorf("endpoint %q header %q: %w", name, headerKey, err))
            }
        }
    }
    
    // Return combined errors
    if len(allErrors) > 0 {
        return fmt.Errorf("endpoint validation failed:\n%v", formatErrors(allErrors))
    }
    
    return nil
}

// formatErrors formats multiple errors for readable output
func formatErrors(errs []error) string {
    var msgs []string
    for _, err := range errs {
        msgs = append(msgs, "  - "+err.Error())
    }
    return strings.Join(msgs, "\n")
}
```

### 4. Update validateMergedConfig in loader.go

Replace duplicate validation with:
```go
func validateMergedConfig(config *Config) error {
    if config == nil {
        return fmt.Errorf("configuration is nil")
    }
    
    // Allow empty endpoints - this will be handled in main with graceful exit
    // But still validate if any are present
    if len(config.Endpoints) > 0 {
        return validateEndpoints(config.Endpoints)
    }
    
    return nil
}
```

### 5. Add Comprehensive Tests

```go
func TestValidateURL(t *testing.T) {
    tests := []struct {
        name        string
        url         string
        expectError bool
        errorMsg    string
    }{
        {
            name:        "valid HTTP URL",
            url:         "http://example.com",
            expectError: false,
        },
        {
            name:        "valid HTTPS URL",
            url:         "https://api.example.com/v1",
            expectError: false,
        },
        {
            name:        "empty URL",
            url:         "",
            expectError: true,
            errorMsg:    "cannot be empty",
        },
        {
            name:        "invalid URL format",
            url:         "not-a-url",
            expectError: true,
            errorMsg:    "invalid URL format",
        },
        {
            name:        "unsupported scheme - ftp",
            url:         "ftp://example.com",
            expectError: true,
            errorMsg:    "scheme must be http or https",
        },
        {
            name:        "unsupported scheme - ws",
            url:         "ws://example.com",
            expectError: true,
            errorMsg:    "scheme must be http or https",
        },
        {
            name:        "missing host",
            url:         "http://",
            expectError: true,
            errorMsg:    "must have a non-empty host",
        },
        {
            name:        "URL with fragment",
            url:         "https://example.com#section",
            expectError: true,
            errorMsg:    "cannot contain fragment",
        },
    }
    // ... test implementation
}

func TestValidateHeaderKey(t *testing.T) {
    tests := []struct {
        name        string
        key         string
        expectError bool
    }{
        {"valid header", "Authorization", false},
        {"valid with hyphens", "X-API-Key", false},
        {"empty key", "", true},
        {"key with space", "X API Key", true},
        {"key with colon", "Content-Type:", true},
        {"key with control char", "X-API\u0001Key", true},
    }
    // ... test implementation
}
```

### 6. Error Handling Strategy

- Use `errors.Join` or custom formatting to aggregate multiple validation errors
- Provide clear error messages that identify which endpoint and header has issues
- Continue validation to collect all errors before failing, so users can fix all issues at once

### 7. Performance Considerations

- URL parsing is O(n) where n is the length of the URL string
- Header validation is also linear in the size of headers
- These validations happen once at startup, so performance impact is negligible

### 8. Migration Path

- Existing configurations with valid URLs and headers will continue to work
- Configurations with invalid URLs or headers will now fail fast at startup instead of at runtime
- This improves developer experience by catching errors early

Task completed successfully.

Added comprehensive URL, header, and endpoint name validation

Implemented HTTP/HTTPS scheme restriction and RFC 7230 header validation

Added case-insensitive collision detection and aggregated error reporting

Created comprehensive test suite covering all scenarios
<!-- SECTION:NOTES:END -->
