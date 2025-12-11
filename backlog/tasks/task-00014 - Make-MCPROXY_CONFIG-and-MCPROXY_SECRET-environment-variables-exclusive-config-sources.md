---
id: task-00014
title: >-
  Make MCPROXY_CONFIG and MCPROXY_SECRET environment variables exclusive config
  sources
status: Done
assignee: []
created_date: '2025-12-11 00:25'
updated_date: '2025-12-11 01:29'
labels:
  - configuration
  - bugfix
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Currently, setting `MCPROXY_CONFIG` or `MCPROXY_SECRET` environment variables sets the path for the "project" configuration, but the system still attempts to load and merge the "global" configuration (from user home or XDG paths).

The desired behavior is strict exclusivity:
- If `MCPROXY_CONFIG` is present, ONLY that specific config file should be loaded. Global config should be ignored.
- If `MCPROXY_SECRETS` is present, ONLY that specific secrets file should be loaded. Global secrets should be ignored.
- If neither are present, the existing hierarchical behavior (Global + Project) remains.

This allows for completely isolated execution environments (e.g., for testing or specific deployments) without accidental leakage from global user settings.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Logic in `pkg/config/loader.go` updated to check for env vars before attempting to load global files.
- [ ] #2 If `MCPROXY_CONFIG` is set, `LoadConfigHierarchical` returns a result where `GlobalFiles.Loaded` is false, even if a global file exists on disk.
- [ ] #3 If `MCPROXY_SECRETS` is set, global secrets are not loaded/merged.
- [ ] #4 Standard hierarchical loading (Global + Project) still works when env vars are NOT set.
- [ ] #5 Unit tests added to verify exclusive loading behavior.
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Modify `pkg/config/loader.go`:

```go
func LoadConfigHierarchical() (*HierarchicalLoadResult, error) {
    result := &HierarchicalLoadResult{}

    // Check env vars directly to determine mode
    envConfigPath := os.Getenv("MCPROXY_CONFIG")
    envSecretsPath := os.Getenv("MCPROXY_SECRETS")

    // Determine paths
    // If env var is set, we skip global loading for that type
    var globalConfigPath, globalSecretsPath string

    if envConfigPath == "" {
        globalConfigPath = getGlobalConfigPath()
    }
    // else: globalConfigPath remains empty, effectively skipping it

    if envSecretsPath == "" {
        globalSecretsPath = getGlobalSecretsPath()
    }

    projectConfigPath := getProjectConfigPath() 
    projectSecretsPath := getProjectSecretsPath()

    // ... existing loading logic ...
    // ensure loadConfigFromFile handles empty strings gracefully (it already does)
}
```

**Test Plan (pkg/config/loader_test.go):**
Need to add a test case that sets the Env var and mocks/creates a global file, asserting that the global file content is NOT present in the result.

```go
func TestLoadConfigHierarchical_ExclusiveEnv(t *testing.T) {
    // Setup temp home dir for global config
    tempHome := t.TempDir()
    t.Setenv("HOME", tempHome)
    
    // Create global config
    globalConfig := Config{
        Endpoints: map[string]Endpoint{
            "global-ep": {Url: "http://global"},
        },
    }
    // Write globalConfig to tempHome/config.json
    
    // Create specific config file
    specificConfig := Config{
        Endpoints: map[string]Endpoint{
            "specific-ep": {Url: "http://specific"},
        },
    }
    tempConfigPath := filepath.Join(t.TempDir(), "custom.json")
    // Write specificConfig to tempConfigPath
    
    // Set Env Var
    t.Setenv("MCPROXY_CONFIG", tempConfigPath)
    
    // Execute
    result, err := LoadConfigHierarchical()
    if err != nil { t.Fatalf("...") }
    
    // Assert
    if _, exists := result.Config.Endpoints["global-ep"]; exists {
        t.Errorf("Expected global endpoint to be ignored when MCPROXY_CONFIG is set")
    }
    if _, exists := result.Config.Endpoints["specific-ep"]; !exists {
        t.Errorf("Expected specific endpoint to be present")
    }
}
```

Implementation complete. Environment variables MCPROXY_CONFIG and MCPROXY_SECRETS now provide exclusive configuration loading.

Changes made to pkg/config/loader.go:

- LoadConfigHierarchical now checks for env vars first

- When MCPROXY_CONFIG is set, only that file is loaded (global config is skipped)

- When MCPROXY_SECRETS is set, only that file is loaded (global secrets are skipped)

- Standard hierarchical loading still works when env vars are not set

Added comprehensive test TestLoadConfigHierarchical_ExclusiveEnv that verifies all scenarios:

- Exclusive config loading with MCPROXY_CONFIG

- Exclusive secrets loading with MCPROXY_SECRETS

- Both env vars set (completely isolated)

- No env vars (standard hierarchical)

Fixed existing tests to not unintentionally trigger exclusive mode.
<!-- SECTION:NOTES:END -->
