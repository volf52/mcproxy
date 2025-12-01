---
id: task-00011
title: Support MCPROXY_PORT environment variable for port configuration
status: Done
assignee: []
created_date: '2025-12-01 00:16'
updated_date: '2025-12-01 01:08'
labels: []
dependencies: []
priority: low
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The MCPROXY_PORT environment variable currently exists but needs proper validation to support port formats like "8098" without requiring the colon prefix. The current implementation requires the format ":8099" but should be more flexible to accept "8099" as well.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 MCPROXY_PORT accepts both '8099' and ':8099' formats
- [x] #2 Default port remains ':8099' when MCPROXY_PORT is not set
- [x] #3 Port validation ensures valid port numbers
- [x] #4 Error handling for invalid port formats
- [x] #5 Documentation updated to reflect the flexible port format
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
## Implementation Plan

### Current Issue Analysis
- Location: `/home/volfy/hobby/mcpproxy/cmd/mcproxy/main.go:73-76`
- Current code directly uses `os.Getenv("MCPROXY_PORT")` without validation
- Error: `listen tcp: address 8098: missing port in address` when MCPROXY_PORT="8098"
- Root cause: Go's `net.Listen` requires port format with leading colon (":8098")

### Implementation Steps

#### 1. Create Port Normalization Function
- Add `func normalizePortFormat(port string) string` in cmd/mcproxy/main.go
- Function should:
  - Check if string starts with ":"
  - If not, validate it's a valid port number (1-65535)
  - Prepend ":" if missing
  - Return normalized string or error for invalid ports

#### 2. Add Port Validation
- Validate port range (1-65535)
- Handle edge cases: empty string, non-numeric values, out-of-range ports
- Return descriptive error messages for invalid inputs

#### 3. Update Main Function
- Replace direct `os.Getenv("MCPROXY_PORT")` usage with validation logic
- Maintain current default behavior (":8099" when env var not set)
- Add proper error handling with user-friendly messages

#### 4. Add Tests
- Unit tests for `normalizePortFormat` function
- Test cases: ":8099", "8099", "", invalid ports, edge cases
- Integration test for complete server startup flow

#### 5. Update Documentation
- Update CLAUDE.md to document the flexible port format
- Add examples showing both "8099" and ":8099" usage
- Document error handling behavior

### Files to Modify
- `cmd/mcproxy/main.go` - main implementation
- Add test file: `cmd/mcproxy/main_test.go` - unit tests
- `CLAUDE.md` - documentation updates

### Success Criteria
- `MCPROXY_PORT="8098" ./mcproxy` succeeds (currently fails)
- `MCPROXY_PORT=":8098" ./mcproxy` continues to work
- Invalid port numbers show clear error messages
- Default behavior unchanged when MCPROXY_PORT not set
<!-- SECTION:PLAN:END -->
