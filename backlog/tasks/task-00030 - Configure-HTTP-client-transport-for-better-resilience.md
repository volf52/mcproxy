---
id: task-00030
title: Configure HTTP client transport for better resilience
status: To Do
assignee: []
created_date: '2025-12-15 02:10'
labels:
  - enhancement
  - http-client
  - resilience
  - configuration
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Update the HTTP client transport configuration in CreateHTTPClient to handle graceful shutdowns and retry scenarios more gracefully.

The configuration should include:

1. Setting appropriate retry policies for HTTP/2
2. Configuring idle connection timeout to be more aggressive with flaky upstreams
3. Adding connection pool health checks
4. Considering HTTP/1.1 fallback for problematic endpoints
5. Adding custom retry logic for specific status codes

This affects the CreateHTTPClient function in pkg/proxy/client.go.
<!-- SECTION:DESCRIPTION:END -->
