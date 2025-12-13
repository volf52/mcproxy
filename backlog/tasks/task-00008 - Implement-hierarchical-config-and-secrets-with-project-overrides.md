---
id: task-00008
title: Implement hierarchical config and secrets with project overrides
status: Done
assignee: []
created_date: '2025-11-27 16:19'
updated_date: '2025-12-08 19:26'
labels:
  - configuration
  - secrets
  - hierarchy
  - v1
dependencies: []
priority: high
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement a hierarchical configuration system where global config/secrets are stored in user's home directory and project-specific config/secrets are stored in current directory. ALL config and secrets files are optional - load from whichever files are available. Project-specific values should override (not replace) global ones in cases of conflicts. Support environment variable overrides for project-specific file paths. If zero total endpoints are configured after merging all available configs, log a warning and exit gracefully.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria

<!-- AC:BEGIN -->

- [x] #1 Support global config/secrets in ~/config.json and ~/secrets.json (both optional)
- [x] #2 Support project-specific config/secrets in ./config.json and ./secrets.json by default (both optional)
- [x] #3 Add MCPROXY_CONFIG and MCPROXY_SECRETS env vars to override project-specific file paths
- [x] #4 Implement merging logic where project-specific endpoints/secrets override global ones, not replace them
- [x] #5 Maintain backward compatibility with current single-file configuration
- [x] #6 Log clear information about which files are loaded and what is being merged/overridden
- [x] #7 All config and secrets files are optional - load from available files only

- [x] #8 If zero total endpoints are configured after merging all available configs, log a warning and exit gracefully
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
## Implementation Plan

### Phase 1: Create Hierarchical Loading Infrastructure

1. **Create `pkg/config/loader.go`**
   - Define file paths with global and project-specific defaults
   - Add environment variable overrides for project paths
   - Create helper functions to check file existence
   - Add structured logging for file discovery and loading

2. **Define new file path structure**
   - Global config: `~/config.json` (optional)
   - Global secrets: `~/secrets.json` (optional)
   - Project config: `./config.json` or `MCPROXY_CONFIG` (optional)
   - Project secrets: `./secrets.json` or `MCPROXY_SECRETS` (optional)

### Phase 2: Implement Configuration Merging

1. **Create merging utilities in `loader.go`**
   - `mergeConfigs(global, project) *Config` - merges endpoint maps
   - `mergeSecrets(global, project) Secrets` - merges secret maps
   - Project values override global values for same keys
   - Global values are preserved for keys that don't exist in project

2. **Add file loading functions**
   - `loadConfigFromFile(path string) (*Config, error)` with file existence check
   - `loadSecretsFromFile(path string) (Secrets, error)` with file existence check
   - Functions return nil when file doesn't exist (not an error)

### Phase 3: Update Main Configuration Functions

1. **Modify `config.go`**
   - Update `LoadConfig()` to use hierarchical loading
   - Update `LoadSecrets()` to use hierarchical loading
   - Add new functions: `LoadConfigHierarchical()`, `LoadSecretsHierarchical()`
   - Maintain backward compatibility with existing function signatures

2. **Add helper functions**
   - `getGlobalConfigPath() string` - returns `~/config.json`
   - `getGlobalSecretsPath() string` - returns `~/secrets.json`
   - `getProjectConfigPath() string` - returns env var or `./config.json`
   - `getProjectSecretsPath() string` - returns env var or `./secrets.json`

### Phase 4: Update Main Application (`cmd/mcproxy/main.go`)

1. **Modify main() function**
   - Replace single file loading with hierarchical loading
   - Add detailed logging about which files are being loaded
   - Log merging/overriding information
   - Implement graceful exit when zero total endpoints

2. **Add comprehensive startup logging**
   - Log which config files are found and loaded
   - Log which secrets files are found and loaded
   - Log number of endpoints from global vs project configs
   - Log any override warnings for conflicting keys

### Phase 5: Update Validation Logic

1. **Modify `validateEndpoints()` in `config.go`**
   - Remove the hard requirement for at least one endpoint
   - Add separate validation for merged configuration
   - Return warning instead of error for empty endpoint map

2. **Add new validation**
   - `validateMergedConfig(config *Config) error` - checks final merged result
   - Handle case where no config files exist at all
   - Provide clear error messages for configuration issues

### Phase 6: Testing (`pkg/config/config_test.go`)

1. **Add hierarchical loading tests**
   - Test loading with only global files
   - Test loading with only project files
   - Test loading with both (override behavior)
   - Test loading with no files (graceful handling)
   - Test environment variable overrides

2. **Add merging logic tests**
   - Test endpoint merging with conflicting keys
   - Test secrets merging with conflicting keys
   - Test empty config merging
   - Test validation of merged configuration

### Key Implementation Details

- **File discovery**: Log which files are being searched for and loaded
- **Optional files**: All config and secrets files are optional, no errors for missing files
- **Merging strategy**: Project values override global values for same keys
- **Backward compatibility**: Existing `LoadConfig()` and `LoadSecrets()` continue to work
- **Graceful degradation**: Continue running with available files, warn about issues
- **Zero endpoint handling**: Log warning and exit gracefully when no endpoints are configured

### Files to Create/Modify

- `pkg/config/loader.go` (new)
- `pkg/config/config.go` (modify)
- `cmd/mcproxy/main.go` (modify)
- `pkg/config/config_test.go` (add tests)

### Environment Variables

- `MCPROXY_CONFIG` - override project config file path
- `MCPROXY_SECRETS` - override project secrets file path
- `MCPROXY_DEBUG` - enable detailed loading/merging logs

### Validation Criteria

- All four config files are optional
- Project config overrides global config for same endpoint keys
- Project secrets override global secrets for same secret keys
- Clear logging shows which files are loaded and what is merged
- Graceful exit with warning when zero endpoints are configured
- Backward compatibility maintained for single-file configurations
<!-- SECTION:PLAN:END -->
