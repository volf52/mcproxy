---
id: task-00006
title: 'Tests: happy path, missing secret, HTTPS upstream'
status: To Do
assignee: []
created_date: '2025-11-26 23:07'
updated_date: '2025-12-15 15:29'
labels:
  - backend
  - tests
  - v1
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add automated tests covering: valid endpoint proxying; endpoint skipped when secret missing; forwarding to HTTPS upstream (can use httptest TLS server); ensure headers templated and methods enforced.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Verification result: NOT COMPLETED - Happy path and missing secret tests done, but missing HTTPS upstream integration tests with real TLS connections.
<!-- SECTION:NOTES:END -->
