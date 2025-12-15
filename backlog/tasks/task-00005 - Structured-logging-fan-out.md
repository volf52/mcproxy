---
id: task-00005
title: Structured logging fan-out
status: To Do
assignee: []
created_date: '2025-11-26 23:07'
updated_date: '2025-12-15 15:29'
labels:
  - backend
  - logging
  - v1
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement structured logging (level, ts, msg, endpoint, path, status, duration_ms, missing_secret). Default stdout; when config.logging.file is set, tee logs to that file and stdout.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Verification result: NOT COMPLETED - Basic secure logging exists but missing structured logging fan-out (simultaneous stdout and file output).
<!-- SECTION:NOTES:END -->
