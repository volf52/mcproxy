---
id: task-00009
title: Change default config file location to ~/.config/mcproxy/config.jsonc
status: In Progress
assignee: []
created_date: '2025-11-28 16:49'
updated_date: '2025-11-30 22:33'
labels: []
dependencies: []
priority: medium
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Update the configuration loader to use ~/.config/mcproxy/config.jsonc as the default location instead of ./config.json. This follows the XDG Base Directory specification for better Linux/macOS integration while maintaining backward compatibility and adding support for JSONC format.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Default config path changes from ./config.json to ~/.config/mcproxy/config.jsonc
- [ ] #2 Environment variable MCPROXY_CONFIG still overrides the default path
- [ ] #3 Create ~/.config/mcproxy directory if it doesn't exist with proper error handling
- [ ] #4 Maintain backward compatibility - fallback to ./config.json if ~/.config/mcproxy/config.jsonc doesn't exist
- [ ] #5 Update CLAUDE.md documentation to reflect new default path and JSONC support
- [ ] #6 Include proper error handling for permission issues when creating ~/.config/mcproxy directory

- [ ] #7 Update README.md with new default configuration location and JSONC format support
- [ ] #8 Add unit tests for new config loading logic, JSONC parsing, and fallback behavior
- [ ] #9 Ensure secrets file path behavior remains consistent (still ~/secrets.json by default)

- [ ] #10 Validate that JSONC comments are properly parsed and ignored
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Research current config loading implementation in pkg/config

Identify all places where config file path is defined and used

Add JSONC parsing support (strip comments before JSON parsing)

Design directory creation logic with proper error handling and permissions

Implement fallback logic: ~/.config/mcproxy/config.jsonc → ./config.jsonc → ./config.json → error

Update environment variable override logic to work with new defaults

Write comprehensive unit tests covering all scenarios (new path, JSONC parsing, fallback, env override, permission errors)

Update documentation files (CLAUDE.md, README.md) with new default paths and JSONC format

Test with various scenarios: fresh install, existing user, permission issues, JSONC files

Ensure logging provides clear feedback about which config file is being loaded
<!-- SECTION:PLAN:END -->
