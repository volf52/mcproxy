---
id: task-00003
title: HTTP endpoint registration & proxying
status: Done
assignee: []
created_date: '2025-11-26 23:07'
updated_date: '2025-11-27 16:09'
labels:
  - backend
  - http
  - proxy
  - v1
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Register POST handlers for each valid MCP entry at /{name}. Forward incoming request body and headers to upstream URL (http/https) with configured headers applied after templating. Return upstream status/headers/body. Reject non-POST with 405.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented HTTP endpoint registration and proxying functionality:

- Created pkg/proxy/server.go with Server struct and proxy handlers

- Created pkg/proxy/client.go with shared HTTP client with TLS & timeouts

- Updated main.go to start the HTTP server with processed endpoints

- Registered POST handlers for each endpoint at /{name}

- Implemented 405 Method Not Allowed for non-POST requests

- Forward requests to upstream URLs with header merging

- Stream upstream responses back to caller

- Build passes with `go build ./cmd/mcproxy`

- Static analysis passes with `go vet ./...`
<!-- SECTION:NOTES:END -->
