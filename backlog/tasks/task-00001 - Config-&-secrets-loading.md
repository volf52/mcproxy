---
id: task-00001
title: Config & secrets loading
status: Done
assignee: []
created_date: '2025-11-26 23:07'
updated_date: '2025-11-26 23:47'
labels:
  - backend
  - config
  - v1
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Load JSON config (default ./config.json or MCPROXY_CONFIG) and secrets (default ~/secrets.json or MCPROXY_SECRETS). Validate presence and JSON shape; exit with clear error when missing/invalid. Allow partial availability only after successful load.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Config & secrets loading implementation completed successfully.

Key features implemented:

- JSON config loading from ./config.json or MCPROXY_CONFIG env var

- Secrets loading from ~/secrets.json or MCPROXY_SECRETS env var

- Comprehensive validation of endpoints and JSON structure

- Clear error messages for missing/invalid files

- Full test coverage for all error cases and happy path

Files created:

- pkg/config/config.go: Core config loading logic

- pkg/config/config_test.go: Comprehensive test suite

- cmd/mcproxy/main.go: Basic main function to test loading
<!-- SECTION:NOTES:END -->
