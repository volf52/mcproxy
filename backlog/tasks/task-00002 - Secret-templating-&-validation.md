---
id: task-00002
title: Secret templating & validation
status: Done
assignee: []
created_date: '2025-11-26 23:07'
updated_date: '2025-11-27 00:25'
labels:
  - backend
  - templating
  - v1
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement `{{ var }}` substitution for templated fields (headers). Resolve from secrets map; if any placeholder missing, log a warning including endpoint name and variable and skip registering that endpoint.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Secret templating & validation implementation completed successfully.

Key features implemented:

{{ var_name }} substitution using simple string operations (more efficient than regex)

Proper handling of missing secrets with warnings and endpoint skipping

Support for multiple templates in a single header value

Comprehensive error handling for malformed templates

Full test coverage with 8 test cases for substituteTemplate function

Files updated:

- pkg/config/config.go: Added ProcessSecretTemplates, substituteTemplate, and supporting functions

- pkg/config/config_test.go: Added comprehensive test suite with 16 total tests

- cmd/mcproxy/main.go: Updated to demonstrate templating functionality
<!-- SECTION:NOTES:END -->
