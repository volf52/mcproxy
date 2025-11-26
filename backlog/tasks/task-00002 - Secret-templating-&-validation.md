---
id: task-00002
title: Secret templating & validation
status: To Do
assignee: []
created_date: '2025-11-26 23:07'
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
