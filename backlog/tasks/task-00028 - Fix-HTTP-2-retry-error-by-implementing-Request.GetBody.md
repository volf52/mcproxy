---
id: task-00028
title: Fix HTTP/2 retry error by implementing Request.GetBody
status: To Do
assignee: []
created_date: '2025-12-15 02:09'
labels:
  - bug
  - http2
  - retry
  - networking
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Fix the HTTP/2 retry error "cannot retry err after Request.Body was written; define Request.GetBody" by implementing Request.GetBody in the createUpstreamRequest function. This error occurs when upstream servers (api.z.ai) perform graceful shutdowns and the HTTP/2 transport needs to retry the request but cannot because the body has already been consumed.

The fix involves:

1. Preserving the original request body
2. Implementing Request.GetBody as a function that returns a new reader for the body
3. Ensuring the body can be replayed for retries

This affects the createUpstreamRequest function in pkg/proxy/server.go around line 419.
<!-- SECTION:DESCRIPTION:END -->
