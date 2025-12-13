---
id: task-00026
title: Fix incorrect configuration path messages in main.go
status: Done
assignee: []
created_date: '2025-12-13 14:56'
updated_date: '2025-12-13 15:49'
labels:
  - bug-fix
  - logging
  - documentation
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The logging messages in main.go lines 52-56 show incorrect paths for configuration files.

Current incorrect messages:

- "Global config: ~/config.jsonc or ~/config.json"
- "Project config: ~/.config/mcproxy/config.jsonc"

Should be (based on actual loader.go logic):

- Global config: ~/config.jsonc or ~/config.json (this is actually correct)
- Global secrets: ~/secrets.jsonc or ~/secrets.json (this is actually correct)
- Project config: ~/.config/mcproxy/config.jsonc (XDG) OR ./.mcproxy/config.jsonc (fallback)
- Project secrets: ./.mcproxy/secrets.jsonc

The current messages are confusing because:

1. They don't reflect the actual priority order (XDG first, then .mcproxy)
2. They show project config in XDG location as if it's the only option
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Successfully implemented enhanced configuration file logging that shows actual search and loading behavior instead of generic guidance.

Changes made:

1. Added attempt and success logs to hierarchical loading in loader.go (lines 451-505)
2. Added attempt and success logs to exclusive mode loading in loader.go (lines 286-342)  
3. Added mode detection logs (hierarchical vs exclusive)
4. Added fallback logging when exclusive files aren't found
5. Replaced generic guidance in main.go with reference to check logs above

The logging now clearly shows:

- Configuration mode (hierarchical or exclusive)
- Exact file paths being attempted
- Success/not found status for each file
- Number of endpoints/secrets loaded
- Priority order and search behavior

Tested successfully in both modes:

- Exclusive mode: Shows env var detection and exclusive file loading
- Hierarchical mode: Shows global → project search order with XDG precedence
<!-- SECTION:NOTES:END -->
