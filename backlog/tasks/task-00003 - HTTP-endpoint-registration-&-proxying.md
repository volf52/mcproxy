---
id: task-00003
title: HTTP endpoint registration & proxying
status: To Do
assignee: []
created_date: '2025-11-26 23:07'
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
