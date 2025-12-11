---
id: task-00017
title: Fix environment exclusive config error handling
status: To Do
assignee: []
created_date: '2025-12-11 19:55'
updated_date: '2025-12-11 20:00'
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
## Technical Analysis

The bug is in the `LoadConfigHierarchical` function in `/home/volfy/hobby/mcpproxy/pkg/config/loader.go`. The function has two modes:

1. **Exclusive mode** (when MCPROXY_CONFIG/MCPROXY_SECRETS are set): Should load ONLY from the specified paths
2. **Hierarchical mode** (when env vars are NOT set): Should load from multiple optional locations

In exclusive mode, the code uses:
```go
// Lines 268-274
projectConfig, _ = loadConfigFromFile(envConfigPath)  // Error discarded!
if projectConfig != nil {
    // Success handling
}

// Lines 305-311  
projectSecrets, _ = loadSecretsFromFile(envSecretsPath)  // Error discarded!
if projectSecrets != nil {
    // Success handling
}
```

The problem is that errors are discarded, and only non-nil results are considered successful. This means:
- Invalid JSON returns (nil, error) → silently ignored
- Permission denied returns (nil, error) → silently ignored
- File exists but is empty returns (&Config{}, nil) → accepted as valid

## Implementation Plan

### 1. Create a new function for exclusive file loading
Add a new function that distinguishes between "file not found" and other errors:

```go
// loadConfigFileExclusive loads a config file that was explicitly specified
// Returns the config and any error except file-not-found
func loadConfigFileExclusive(path string) (*Config, error) {
    if path == "" {
        return nil, fmt.Errorf("config path cannot be empty")
    }
    
    config, err := loadConfigFromFile(path)
    if err != nil {
        // loadConfigFromFile already returns nil for file not found
        // Any other error should be propagated
        return nil, err
    }
    
    return config, nil
}

// loadSecretsFileExclusive loads a secrets file that was explicitly specified
// Returns the secrets and any error except file-not-found
func loadSecretsFileExclusive(path string) (Secrets, error) {
    if path == "" {
        return nil, fmt.Errorf("secrets path cannot be empty")
    }
    
    secrets, err := loadSecretsFromFile(path)
    if err != nil {
        // loadSecretsFromFile already returns nil for file not found
        // Any other error should be propagated
        return nil, err
    }
    
    return secrets, nil
}
```

### 2. Update LoadConfigHierarchical to handle exclusive mode errors
Replace the error-discarding calls with proper error handling:

```go
if envConfigPath != "" {
    // Exclusive mode: only load from the specified config file
    result.ProjectFiles.Path = envConfigPath
    projectConfig, err = loadConfigFileExclusive(envConfigPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load exclusive config from %s: %w", envConfigPath, err)
    }
    if projectConfig != nil {
        result.ProjectFiles.Loaded = true
        result.ProjectFiles.Endpoints = len(projectConfig.Endpoints)
        logging.Printf("Loaded exclusive config from %s (%d endpoints)", envConfigPath, len(projectConfig.Endpoints))
    }
} else {
    // ... existing hierarchical mode logic
}

if envSecretsPath != "" {
    // Exclusive mode: only load from the specified secrets file
    projectSecrets, err = loadSecretsFileExclusive(envSecretsPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load exclusive secrets from %s: %w", envSecretsPath, err)
    }
    if projectSecrets != nil {
        result.ProjectFiles.Secrets = len(projectSecrets)
        logging.Printf("Loaded exclusive secrets from %s (%d entries)", envSecretsPath, len(projectSecrets))
    }
} else {
    // ... existing hierarchical mode logic
}
```

### 3. Add unit tests
Create tests for the new exclusive loading behavior:

```go
func TestLoadConfigFileExclusive(t *testing.T) {
    tests := []struct {
        name          string
        path          string
        wantErr       bool
        wantErrContains string
    }{
        {
            name:    "non-existent file",
            path:    "/tmp/non-existent-config.json",
            wantErr: true,
            wantErrContains: "no such file or directory",
        },
        {
            name:    "invalid JSON",
            path:    "testdata/invalid.json",
            wantErr: true,
            wantErrContains: "failed to parse",
        },
        {
            name:    "valid config",
            path:    "testdata/valid.json",
            wantErr: false,
        },
        {
            name:    "empty path",
            path:    "",
            wantErr: true,
            wantErrContains: "cannot be empty",
        },
    }
    // ... test implementation
}
```

### 4. Integration test scenario
Test the complete flow with environment variables:

```go
func TestExclusiveModeErrorHandling(t *testing.T) {
    // Set env var to invalid file
    os.Setenv("MCPROXY_CONFIG", "/tmp/invalid.json")
    defer os.Unsetenv("MCPROXY_CONFIG")
    
    // Create invalid file
    os.WriteFile("/tmp/invalid.json", []byte("{ invalid json"), 0644)
    defer os.Remove("/tmp/invalid.json")
    
    // LoadConfigHierarchical should fail
    _, err := LoadConfigHierarchical()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to load exclusive config")
}
```

### 5. Edge Cases to Consider
- **Empty files**: Should load as empty config (valid behavior)
- **File exists but no read permissions**: Should fail with permission error
- **Symlink to non-existent file**: Should fail with appropriate error
- **Invalid JSONC with comments**: Should parse correctly if JSONC
- **Mixed mode**: MCPROXY_CONFIG set but not MCPROXY_SECRETS (should handle each independently)

### 6. Backward Compatibility
- Hierarchical mode behavior unchanged (optional files still silently skipped)
- Only affects exclusive mode when env vars are explicitly set
- Error messages follow existing pattern with context using fmt.Errorf
<!-- SECTION:NOTES:END -->
