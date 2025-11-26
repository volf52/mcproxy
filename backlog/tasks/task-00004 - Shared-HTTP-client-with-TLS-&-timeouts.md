---
id: task-00004
title: Shared HTTP client with TLS & timeouts
status: To Do
assignee: []
created_date: '2025-11-26 23:07'
labels:
  - backend
  - http
  - tls
  - v1
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Use a shared http.Client with keep-alives, sane timeouts, no redirect following by default, and default TLS config to support HTTPS upstreams. Consider max idle conns per host defaults.
<!-- SECTION:DESCRIPTION:END -->
