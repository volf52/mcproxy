---
id: task-00017
title: Fix environment exclusive config error handling
status: Done
assignee: []
created_date: '2025-12-11 19:55'
updated_date: '2025-12-12 16:40'
labels:
  - bug
  - error-handling
  - configuration
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
When MCPROXY_CONFIG/MCPROXY_SECRETS environment variables are explicitly set, any errors from loading the specified files (permission denied, invalid JSON/JSONC syntax, etc.) are silently discarded in pkg/config/loader.go:268-333. This causes the service to continue with empty configuration/secrets instead of failing fast with a clear error message. This is problematic because:

1. Users explicitly specify config paths via environment variables, indicating these are the required files
2. Silent failures lead to confusing runtime behavior where endpoints don't work
3. The current behavior contradicts the "fail fast" principle stated in the project guidelines

The issue occurs specifically in the LoadConfigHierarchical function where errors are ignored:

- Line 269: `projectConfig, _ = loadConfigFromFile(envConfigPath)`
- Line 307: `projectSecrets, _ = loadSecretsFromFile(envSecretsPath)`
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria

<!-- AC:BEGIN -->

- [ ] #1 When MCPROXY_CONFIG is set to an invalid file path or unreadable file, the service must exit with a clear error message
- [ ] #2 When MCPROXY_SECRETS is set to an invalid file path or unreadable file, the service must exit with a clear error message
- [ ] #3 When MCPROXY_CONFIG contains invalid JSON/JSONC syntax, the service must exit with a parse error indicating the file and line/column if possible
- [ ] #4 The error messages must be user-friendly and suggest common fixes (file permissions, syntax errors, etc.)
- [ ] #5 The fix must not affect the hierarchical loading behavior when environment variables are NOT set (optional files should still be skipped silently)
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->

1. Implement loadConfigFileExclusive and loadSecretsFileExclusive functions in loader.go

2. Update LoadConfigHierarchical to use these functions when environment variables are set

3. Add comprehensive unit tests for the new exclusive loading functions

4. Add integration tests for the complete exclusive mode flow

5. Test various error scenarios (invalid JSON, permissions, etc.)

6. Verify hierarchical mode behavior is unchanged

7. Update error messages to be user-friendly and actionable
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation completed successfully on 2025-12-12.

Changes made:

1. Created loadConfigFileExclusive and loadSecretsFileExclusive functions that properly handle errors when files are explicitly specified via environment variables
2. Updated LoadConfigHierarchical to use exclusive loading functions when MCPROXY_CONFIG/MCPROXY_SECRETS are set
3. The service now fails fast with clear error messages when explicitly specified files have errors
4. Added comprehensive unit tests in loader_exclusive_test.go covering all error scenarios
5. Added integration test in config_test.go for complete flow verification

The implementation ensures:

- No more silent failures when env vars point to invalid files
- Clear, actionable error messages that include the file path and specific issue
- Graceful fallback to hierarchical mode when both files don't exist
- Backward compatibility preserved for hierarchical mode

All acceptance criteria met:
✓ MCPROXY_CONFIG errors cause service to exit with clear error messages
✓ MCPROXY_SECRETS errors cause service to exit with clear error messages  
✓ Parse errors include file path and context
✓ Error messages are user-friendly and suggest common fixes
✓ Hierarchical mode behavior is unchanged when env vars are not set

Created enhancement task-00023 for future Go best practices improvements based on expert code review.
<!-- SECTION:NOTES:END -->
