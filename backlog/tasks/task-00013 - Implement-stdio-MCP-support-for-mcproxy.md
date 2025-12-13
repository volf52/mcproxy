---
id: task-00013
title: Implement stdio MCP support for mcproxy
status: In Progress
assignee: []
created_date: '2025-12-08 16:27'
updated_date: '2025-12-08 19:26'
labels: []
dependencies: []
priority: high
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add stdio MCP support alongside existing HTTP endpoints in mcproxy service. This feature preserves backward compatibility for HTTP-only configs while adding process management, request translation, and resilience features for stdio-based MCP servers.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria

<!-- AC:BEGIN -->

- [ ] #1 Service supports both HTTP and stdio endpoint types in configuration
- [ ] #2 Existing HTTP-only configurations remain fully functional
- [ ] #3 stdio endpoints can be configured with command, args, environment, and timeout
- [ ] #4 Process manager handles stdio process lifecycle with auto-restart
- [ ] #5 JSON-RPC requests are properly translated between HTTP and stdio protocols
- [ ] #6 Large payloads are handled with configurable buffer limits
- [ ] #7 Graceful shutdown works for both HTTP and stdio endpoints
- [ ] #8 Comprehensive test coverage for new functionality
- [ ] #9 Documentation updated with migration guide and examples
<!-- AC:END -->
