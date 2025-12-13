---
id: task-00012
title: Add /mcp prefix to dynamic endpoints
status: Done
assignee: []
created_date: '2025-12-01 09:20'
updated_date: '2025-12-08 19:26'
labels:
  - backend
  - routing
  - v1
dependencies: []
priority: medium
ordinal: 500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Update the proxy registry to register endpoints with '/mcp/{name}' instead of '/{name}' to avoid conflicts and provide a clear namespace for MCP endpoints. This requires updating the handler registration logic and documentation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria

<!-- AC:BEGIN -->

- [ ] #1 Endpoints are registered at '/mcp/{name}' instead of '/{name}'
- [ ] #2 Incoming requests to '/mcp/{name}' are properly forwarded to configured upstreams
- [ ] #3 Documentation is updated to reflect the new endpoint pattern
- [ ] #4 Tests are updated to use the new /mcp prefix
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
## Detailed Implementation Plan: Add /mcp prefix to dynamic endpoints

### Current Implementation Analysis

Based on code analysis, endpoints are currently registered using `/{name}` pattern in `pkg/proxy/server.go:36`:

```go
pattern := fmt.Sprintf("/%s", name)
```

### Implementation Strategy

**Approach**: Minimal, focused change to only modify the URL pattern while preserving all existing functionality.

### Phase 1: Core Implementation (Critical Path)

1. **Update URL Pattern in pkg/proxy/server.go**
   - **File**: `/home/volfy/hobby/mcpproxy/pkg/proxy/server.go`
   - **Line**: 36
   - **Change**: `pattern := fmt.Sprintf("/%s", name)` → `pattern := fmt.Sprintf("/mcp/%s", name)`
   - **Impact**: Core routing change
   - **Risk**: Low (single line change)

2. **Update Root Handler Logic**
   - **File**: `/home/volfy/hobby/mcpproxy/pkg/proxy/server.go`  
   - **Lines**: 42-50
   - **Change**: Update 404 condition to handle requests to `/mcp` (should return 404)
   - **Current**: Only handles `/` exactly, returns 404 for other paths
   - **New**: Should return 404 for `/mcp` and `/mcp/*` when no endpoint matches

### Phase 2: Documentation Updates

1. **Update README.md Documentation**
   - **File**: `/home/volfy/hobby/mcpproxy/README.md`
   - **Lines**: 132, 201
   - **Changes**:
     - Line 132: `"endpoints": Map of endpoint configurations (required) - Key: Endpoint name (becomes POST /mcp/{name} route)`
     - Line 201: `Endpoints: Each endpoint registers a POST handler at /mcp/{name}`
   - **Configuration Examples**: Update example sections to reflect new `/mcp/{name}` pattern

2. **Update CLAUDE.md Project Instructions**
   - **File**: `/home/volfy/hobby/mcpproxy/CLAUDE.md`
   - **Section**: "Request Flow"  
   - **Change**: Update "registers a POST handler at /{endpoint_name}" → "registers a POST handler at /mcp/{endpoint_name}"

### Phase 3: Test Implementation (Critical for Quality)

1. **Create Server/Proxy Tests** (New test file needed)
   - **New File**: `/home/volfy/hobby/mcpproxy/pkg/proxy/server_test.go`
   - **Test Cases**:
     - Test endpoint registration at `/mcp/{name}`
     - Test POST request forwarding to `/mcp/{name}`
     - Test 404 for non-existent `/mcp/{name}` endpoints
     - Test 405 for non-POST requests to `/mcp/{name}`
     - Test root handler still works at `/`
     - Test multiple endpoints with different names
     - Test endpoints with hyphens in names (e.g., `/mcp/slack-webhook`)
   - **Test Structure**: Use `httptest.NewServer` for integration testing

2. **Update Existing Test References**
   - **File**: Search codebase for any hardcoded `/{name}` patterns in tests
   - **Action**: Update to use `/mcp/{name}` pattern
   - **Current Status**: No existing server tests found, so this may be minimal

### Phase 4: Validation & Edge Cases

1. **Edge Case Handling**
   - **Endpoint name validation**: Ensure names with slashes are handled safely
   - **URL pattern conflicts**: Verify `/mcp` prefix doesn't conflict with root handler
   - **Path cleaning**: Ensure `http.NewServeMux` handles the patterns correctly
   - **Backwards compatibility**: Document that this is a breaking change

2. **Integration Testing**
   - **Manual Test**: Start server with config containing endpoints
   - **Verify**: `curl -X POST http://localhost:8099/mcp/endpoint-name` works
   - **Verify**: `curl -X GET http://localhost:8099/mcp/endpoint-name` returns 405
   - **Verify**: `curl http://localhost:8099/mcp/nonexistent` returns 404
   - **Verify**: `curl http://localhost:8099/` returns service info

### Phase 5: Logging & Monitoring Updates

1. **Update Logging Messages**
   - **File**: Check if any log messages reference endpoint URLs
   - **Action**: Update any hardcoded URL patterns in logging
   - **Current**: `LogEndpointRegistration` only logs name and upstream URL, no change needed

### Implementation Notes

- **Breaking Change**: This is a breaking change for existing clients
- **Configuration**: No config file changes needed - only runtime behavior changes
- **Dependencies**: No new dependencies required
- **Performance**: No performance impact
- **Security**: No security implications

### Test Strategy

- Use table-driven tests for comprehensive coverage
- Mock upstream servers for isolated testing  
- Test both HTTP and HTTPS upstreams
- Verify header forwarding still works correctly
- Test secret templating functionality with new paths

### Rollback Plan

If issues arise, rollback is simple: revert the single line change in server.go line 36 back to `fmt.Sprintf("/%s", name)`
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->

- **Breaking Change**: This is a breaking change for existing clients
- **Configuration**: No config file changes needed - only runtime behavior changes
- **Dependencies**: No new dependencies required
- **Performance**: No performance impact
- **Security**: No security implications

### Test Strategy

- Use table-driven tests for comprehensive coverage
- Mock upstream servers for isolated testing  
- Test both HTTP and HTTPS upstreams
- Verify header forwarding still works correctly
- Test secret templating functionality with new paths

### Rollback Plan

If issues arise, rollback is simple: revert the single line change in server.go line 36 back to `fmt.Sprintf("/%s", name)`
<!-- SECTION:PLAN:END -->
<!-- SECTION:NOTES:END -->
