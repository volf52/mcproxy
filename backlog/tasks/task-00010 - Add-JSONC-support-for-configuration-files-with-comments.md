---
id: task-00010
title: Add JSONC support for configuration files with comments
status: Done
assignee: []
created_date: '2025-11-28 16:50'
updated_date: '2025-11-29 02:27'
labels: []
dependencies: []
priority: low
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Enhance the configuration loader to support JSONC (JSON with Comments) format for both config and secrets files. This will allow adding documentation and comments directly in the configuration files, improving maintainability and user experience. Since Go doesn't have built-in JSONC support, this requires adding a third-party dependency.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Research and select a lightweight JSONC parsing library (e.g., github.com/marcozac/go-jsonc)
- [x] #2 Add JSONC library to go.mod with minimal external dependencies
- [x] #3 Support both .json and .jsonc file extensions for both config and secrets files
- [x] #4 Maintain 100% backward compatibility with existing .json files
- [x] #5 Strip // single-line and /* multi-line */ comments during parsing
- [x] #6 Preserve JSON structure and data types exactly as original JSON parser would

- [x] #7 Update documentation with examples showing commented configuration files
- [x] #8 Add unit tests for JSONC parsing with various comment styles and edge cases
- [x] #9 Ensure secret values remain secure - no comment parsing issues in sensitive sections
- [x] #10 Performance: JSONC parsing should be nearly as fast as standard JSON parsing
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Research available JSONC libraries and evaluate based on size, dependencies, and performance

Select best JSONC library and add to go.mod

Examine current JSON config loading implementation in pkg/config

Design file extension detection logic (.json vs .jsonc)

Implement JSONC parser wrapper that falls back to standard JSON for .json files

Update config loading functions to use appropriate parser based on file extension

Write comprehensive unit tests covering valid JSONC, edge cases, and error conditions

Performance testing to ensure JSONC parsing doesn't significantly impact startup time

Update documentation with examples and migration guide

Test with existing configurations to ensure no regressions
<!-- SECTION:PLAN:END -->
