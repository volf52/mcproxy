---
id: task-00007
title: Implement secure logging for secrets with debug flag
status: Done
assignee: []
created_date: '2025-11-27 16:15'
updated_date: '2025-11-27 17:52'
labels:
  - security
  - logging
  - secrets
  - v1
dependencies: []
priority: high
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Currently the application logs filled secret values when showing endpoint configurations. This is a security risk as secrets are exposed in plain text logs. Need to implement secure logging that hides secrets by default and optionally shows them via a debug/verbose flag for troubleshooting purposes.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 By default, endpoint logging should show placeholder text like [SECRET] or [HIDDEN] instead of actual secret values
- [x] #2 Add a debug/verbose flag (environment variable) that enables showing filled secrets for troubleshooting
- [x] #3 When debug mode is enabled, log additional information about secret templating process
- [x] #4 Maintain current functionality in debug mode while securing production logs
- [x] #5 Ensure no secret values are ever logged in structured logs or error messages
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
## Implementation Plan

### Phase 1: Create Secure Logging Infrastructure
1. **Create `pkg/logging/secure_logger.go`**
   - Implement a SecureLogger wrapper around standard log package
   - Add debug mode flag: `MCPROXY_DEBUG` environment variable
   - Create constants for secret placeholders: `[SECRET]`, `[HIDDEN]`, `***REDACTED***`
   - Add helper functions to detect and redact secret values

2. **Create secret detection utilities**
   - Function to identify if a string contains secret template patterns
   - Function to check if a value matches known secret keys from secrets map
   - Function to redact/securify log messages containing sensitive data

### Phase 2: Update Configuration Package (`pkg/config/`)
1. **Modify `config.go`**
   - Update `ProcessSecretTemplates()` to add secure logging
   - Add debug logging for secret resolution process when enabled
   - Replace direct `log.Printf` calls with secure logging calls
   - Add logging of template processing details in debug mode only

2. **Add secure logging helpers**
   - Function to safely log endpoint configurations
   - Function to log template resolution details in debug mode
   - Function to log missing secret variables without exposing secret values

### Phase 3: Update Proxy Package (`pkg/proxy/`)
1. **Modify `server.go`**
   - Update endpoint registration logging to hide header values
   - Add debug mode logging for upstream request creation
   - Ensure no secret values appear in error messages
   - Add secure logging for request forwarding in debug mode only

### Phase 4: Main Application Update (`cmd/mcproxy/main.go`)
1. **Add debug flag initialization**
   - Read `MCPROXY_DEBUG` environment variable
   - Initialize secure logger with appropriate settings
   - Log application startup information securely

### Phase 5: Testing (`pkg/config/config_test.go`)
1. **Add secure logging tests**
   - Test that secret values are redacted in normal mode
   - Test that secrets are shown in debug mode
   - Test endpoint logging with various header configurations
   - Test error logging doesn't expose secret information

### Key Implementation Details:
- **Placeholder format**: Use `[SECRET]` or `[HIDDEN]` for redacted values
- **Debug flag**: Check `MCPROXY_DEBUG=true` environment variable
- **Backward compatibility**: Maintain all current logging behavior when debug mode is enabled
- **Structured logging**: Ensure no secret values ever appear in structured logs or error messages
- **Performance**: Minimal overhead for secret detection and redaction

### Files to Create/Modify:
- `pkg/logging/secure_logger.go` (new)
- `pkg/config/config.go` (modify)
- `pkg/proxy/server.go` (modify)  
- `cmd/mcproxy/main.go` (modify)
- `pkg/config/config_test.go` (add tests)

### Validation Criteria:
- Normal logs never contain actual secret values
- Debug mode shows full information for troubleshooting
- Template resolution process is logged only in debug mode
- Error messages never expose secret information
- All existing functionality is preserved in debug mode
<!-- SECTION:PLAN:END -->
