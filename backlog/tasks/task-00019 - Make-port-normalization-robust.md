---
id: task-00019
title: Make port normalization robust
status: Done
assignee: []
created_date: '2025-12-11 19:55'
updated_date: '2025-12-12 19:48'
labels:
  - bug
  - config
  - refactoring
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Port normalization in pkg/config/port.go:12-32 has several robustness issues:

1. **Whitespace handling**: The function doesn't trim whitespace from input, causing valid ports surrounded by whitespace to be rejected
2. **Inconsistent normalization**: When a port with a colon has leading zeros (e.g., ":0080"), it's not normalized to ":80" despite the comment promising normalization
3. **Inconsistent error messages**: Error messages include the original (untrimmed) input, which can be confusing

The current tests expect this behavior (lines 153-157, 159-163), but this is fragile and can cause issues when environment variables contain whitespace.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria

<!-- AC:BEGIN -->

- [ ] #1 NormalizePort must trim leading and trailing whitespace from input
- [ ] #2 NormalizePort must always return canonical format (e.g., ":0080" becomes ":80")
- [ ] #3 Error messages must be deterministic and show the cleaned input
- [ ] #4 All edge cases must be handled: empty strings after trimming, multiple colons, invalid formats
- [ ] #5 Tests must cover whitespace and normalization scenarios
- [ ] #6 Existing functionality must remain unchanged for valid inputs
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
### 1. Add input sanitization

```go
func NormalizePort(portStr string) (string, error) {
    // Trim whitespace first
    portStr = strings.TrimSpace(portStr)

    // Continue with existing logic...
}
```

### 2. Ensure consistent normalization

Instead of returning early when port starts with ":", always normalize:

```go
if strings.HasPrefix(portStr, ":") {
    portNum := strings.TrimPrefix(portStr, ":")
    // Normalize port number and always reconstruct
    port, err := strconv.Atoi(portNum)
    if err != nil {
        return "", fmt.Errorf("invalid port format: %w", err)
    }
    if err := validatePortNumber(port); err != nil {
        return "", fmt.Errorf("invalid port format: %w", err)
    }
    return ":" + strconv.Itoa(port), nil
}
```

### 3. Improve error handling

Create a helper for consistent error messages:

```go
func portError(original, cleaned string, err error) error {
    // Show cleaned input in error for clarity
    return fmt.Errorf("invalid port format '%s': %w", cleaned, err)
}
```

### 4. Handle edge cases

- Empty string after trimming should return default
- Multiple colons should be rejected with clear error
- Port number validation should be done after cleaning

### 5. Update tests

Add new test cases for:

- Whitespace trimming: " 8080 ", "\t8080\n", "  :8080  "
- Leading zero normalization with colon: ":0080" -> ":80"
- Mixed whitespace and invalid formats
- Ensure all existing tests still pass

### 6. Consider additional validation

- Reject ports with multiple colons explicitly
- Better error messages for non-numeric content
- Document the normalization behavior more clearly
<!-- SECTION:NOTES:END -->
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Technical Analysis

### Current Implementation Issues

1. **Whitespace Not Trimmed** (lines 12-31):
   - Function doesn't call `strings.TrimSpace()` on input
   - Environment variables with whitespace fail unpredictably
   - Tests on lines 153-163 expect failures for whitespace inputs

2. **Inconsistent Normalization** (lines 17-22):
   - When port starts with ":", function returns input unchanged (line 22)
   - This preserves leading zeros (":0080" stays ":0080")
   - Only ports without colon get normalized (line 29-31)
   - Test on line 124-126 expects ":0080" to remain unchanged

3. **Error Messages Include Raw Input**:
   - Error messages show untrimmed input
   - Confusing when whitespace is the issue

### Usage Context

- Called from `cmd/mcproxy/main.go:68` with `os.Getenv("MCPROXY_PORT")`
- Used with `http.ListenAndServe()` in `pkg/proxy/server.go:53`
- Port format must include leading ":" for Go's HTTP server

## Detailed Implementation Plan

### Phase 1: Refactor NormalizePort Function

```go
func NormalizePort(portStr string) (string, error) {
    // 1. Trim whitespace first
    original := portStr
    portStr = strings.TrimSpace(portStr)

    // 2. Handle empty string after trimming
    if portStr == "" {
        return ":8099", nil
    }

    // 3. Validate format - must be number or :number
    if !isValidPortFormat(portStr) {
        return "", fmt.Errorf("invalid port format '%s': must be a number (1-65535) with optional ':' prefix", portStr)
    }

    // 4. Extract and normalize port number
    portNum := strings.TrimPrefix(portStr, ":")
    port, err := strconv.Atoi(portNum)
    if err != nil {
        return "", fmt.Errorf("invalid port format '%s': %w", portStr, err)
    }

    // 5. Validate port range
    if err := validatePortNumber(port); err != nil {
        return "", fmt.Errorf("invalid port format '%s': %w", portStr, err)
    }

    // 6. Always return canonical format
    return ":" + strconv.Itoa(port), nil
}

// Helper to validate basic format
func isValidPortFormat(s string) bool {
    if s == "" {
        return false
    }

    // Allow single leading colon
    if strings.HasPrefix(s, ":") {
        s = s[1:]
    }

    // Must be digits only
    for _, r := range s {
        if r < '0' || r > '9' {
            return false
        }
    }

    return len(s) > 0
}
```

### Phase 2: Test Updates

New test cases to add:

```go
{
    name:        "whitespace trimmed successfully",
    input:       " 8080 ",
    expected:    ":8080",
    expectError: false,
},
{
    name:        "whitespace with colon trimmed",
    input:       "\t:8080\n",
    expected:    ":8080",
    expectError: false,
},
{
    name:        "multiple spaces become default",
    input:       "   ",
    expected:    ":8099",
    expectError: false,
},
{
    name:        "leading zeros normalized with colon",
    input:       ":0080",
    expected:    ":80",
    expectError: false,
},
{
    name:        "multiple leading zeros normalized",
    input:       ":00080",
    expected:    ":80",
    expectError: false,
},
{
    name:        "tab and newline whitespace",
    input:       "\t\n 8099 \n\t",
    expected:    ":8099",
    expectError: false,
},
{
    name:        "mixed whitespace and invalid format",
    input:       " :8080: ",
    expected:    "",
    expectError: true,
},
```

Update existing tests:

- Change test on lines 153-157 to expect ":8099" (empty after trimming)
- Change test on lines 159-163 to expect ":8080" (whitespace trimmed)
- Change test on lines 123-127 to expect ":80" (normalized)

### Phase 3: Additional Edge Case Handling

Consider adding validation for:

- Multiple colons (":8080:9090" should fail)
- Non-printable characters
- Unicode whitespace (use `strings.TrimSpace()` handles this)

### Phase 4: Documentation Updates

Update function comment:

```go
// NormalizePort validates and normalizes a port string to the canonical format ":port".
// It accepts formats like "8099", ":8099", " 8099 ", "\t:8099\n" and always returns ":8099".
// Leading and trailing whitespace is trimmed. Leading zeros are stripped for consistency.
// Empty string or string containing only whitespace returns the default port ":8099".
```

### Phase 5: Integration Considerations

1. **Environment Variable Handling**:
   - Environment variables often have trailing newlines or spaces
   - Current behavior would fail on `MCPROXY_PORT="8099 "`
   - This fix makes it robust

2. **Backward Compatibility**:
   - All currently valid inputs remain valid
   - Only edge cases (whitespace, leading zeros) change behavior
   - Changes are improvements, not breaking

3. **Error Message Consistency**:
   - Show trimmed input in error messages
   - Use consistent error format throughout
   - Help users understand what went wrong

### Phase 6: Implementation Checklist

- [ ] Implement `isValidPortFormat` helper
- [ ] Refactor `NormalizePort` to trim and always normalize
- [ ] Add new test cases for whitespace handling
- [ ] Update existing tests to reflect new behavior
- [ ] Run all tests to ensure no regressions
- [ ] Test manually with environment variables containing whitespace
- [ ] Update function documentation
- [ ] Consider adding integration tests for environment variable scenarios

## Additional Context

### Dependencies

- Task-00011 (Support MCPROXY_PORT environment variable) is marked as Done and introduced the NormalizePort function
- This is a pure bug fix with no breaking changes to the API
- No integration with other components affected

### Risk Assessment

- **Low Risk**: Changes are internal to NormalizePort function
- **No Breaking Changes**: All currently valid inputs remain valid
- **Test Changes Required**: Some tests expect current (buggy) behavior and need updating
- **Manual Testing Recommended**: Test with environment variables containing whitespace

### Code Style Considerations

- Follow existing Go conventions in the codebase
- Use `strings.TrimSpace()` for Unicode-aware whitespace handling
- Keep error messages consistent with existing patterns
- Add comprehensive godoc comment updates

### Future Improvements

- Consider adding integration tests for environment variable scenarios
- Document behavior for users who might set MCPROXY_PORT with whitespace

Task completed successfully.

Added whitespace trimming and canonical port normalization

Handles edge cases like tabs, newlines, and whitespace

Improved error messages with trimmed input display

Code reviewed and tests passing
<!-- SECTION:NOTES:END -->
