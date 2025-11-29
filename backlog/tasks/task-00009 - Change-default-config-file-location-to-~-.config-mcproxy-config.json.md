---
id: task-00009
title: Change default config file location to ~/.config/mcproxy/config.json
status: In Progress
assignee: []
created_date: '2025-11-28 16:49'
updated_date: '2025-11-29 02:32'
labels: []
dependencies: []
priority: medium
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Update the configuration loader to use ~/.config/mcproxy/config.json as the default location instead of ./config.json. This follows the XDG Base Directory specification for better Linux/macOS integration while maintaining backward compatibility.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Default config path changes from ./config.json to ~/.config/mcproxy/config.json
- [ ] #2 Environment variable MCPROXY_CONFIG still overrides the default path
- [ ] #3 Create ~/.config/mcproxy directory if it doesn't exist with proper error handling
- [ ] #4 Maintain backward compatibility - fallback to ./config.json if ~/.config/mcproxy/config.json doesn't exist
- [ ] #5 Update CLAUDE.md documentation to reflect new default path and fallback behavior
- [ ] #6 Include proper error handling for permission issues when creating ~/.config/mcproxy directory

- [ ] #7 Update README.md with new default configuration location
- [ ] #8 Add unit tests for new config loading logic and fallback behavior
- [ ] #9 Ensure secrets file path behavior remains consistent (still ~/secrets.json by default)
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Research current config loading implementation in pkg/config

Identify all places where config file path is defined and used

Design directory creation logic with proper error handling and permissions

Implement fallback logic: ~/.config/mcproxy/config.json → ./config.json → error

Update environment variable override logic to work with new defaults

Write comprehensive unit tests covering all scenarios (new path, fallback, env override, permission errors)

Update documentation files (CLAUDE.md, README.md) with new default paths

Test with various scenarios: fresh install, existing user, permission issues

Ensure logging provides clear feedback about which config file is being loaded
<!-- SECTION:PLAN:END -->
