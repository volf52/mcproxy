---
id: task-00004
title: Shared HTTP client with TLS & timeouts
status: Done
assignee: []
created_date: '2025-11-26 23:07'
updated_date: '2025-12-08 19:26'
labels:
  - backend
  - http
  - tls
  - v1
dependencies: []
priority: medium
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Use a shared http.Client with keep-alives, sane timeouts, no redirect following by default, and default TLS config to support HTTPS upstreams. Consider max idle conns per host defaults.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Task completed successfully! Enhanced the shared HTTP client implementation with:

- Disabled redirect following for security (CheckRedirect returns ErrUseLastResponse)

- Added HTTP/2 support for better performance (ForceAttemptHTTP2: true)

- Maintained secure TLS configuration (InsecureSkipVerify: false)

- Kept sensible timeout settings (30s dial, 60s total request)

- Preserved connection pooling (MaxIdleConns: 100, MaxIdleConnsPerHost: 10)

- Added comprehensive test coverage for redirect handling, timeouts, HTTPS support, and connection pooling
<!-- SECTION:NOTES:END -->
