---
id: task-00011
title: Support MCPROXY_PORT environment variable for port configuration
status: To Do
assignee: []
created_date: '2025-12-01 00:16'
labels: []
dependencies: []
priority: low
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The MCPROXY_PORT environment variable currently exists but needs proper validation to support port formats like "8098" without requiring the colon prefix. The current implementation requires the format ":8099" but should be more flexible to accept "8099" as well.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 MCPROXY_PORT accepts both '8099' and ':8099' formats
- [ ] #2 Default port remains ':8099' when MCPROXY_PORT is not set
- [ ] #3 Port validation ensures valid port numbers
- [ ] #4 Error handling for invalid port formats
- [ ] #5 Documentation updated to reflect the flexible port format
<!-- AC:END -->
