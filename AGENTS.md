# AGENTS.md

## Build / Test Commands
```bash
go build -o mcproxy              # Build binary
go test ./...                    # Run all tests
go test ./... -run TestName      # Run single test by name
go test ./pkg/config -v          # Run tests in specific package
go vet ./...                     # Static analysis
```

## Code Style
- **Formatting**: Use `gofmt` or `goimports`; no manual formatting.
- **Imports**: Group stdlib, blank line, external deps, blank line, internal packages.
- **Naming**: CamelCase exports, lowercase unexported; avoid stuttering (e.g., `config.Config` not `config.ConfigStruct`).
- **Errors**: Return `error` as last value; wrap with `fmt.Errorf("context: %w", err)`.
- **Types**: Prefer explicit types; use `any` sparingly; avoid naked `interface{}`.
- **Logging**: Structured logs only (key-value pairs); never log secrets.

## Project Notes
- Config: `./config.json` (env `MCPROXY_CONFIG`); Secrets: `~/secrets.json` (env `MCPROXY_SECRETS`).
- Templating uses `{{ var_name }}` placeholders for secrets in header values.
- HTTP-only inbound on `:8099`; outbound supports HTTPS via stdlib TLS.

<!-- BACKLOG.MD MCP GUIDELINES START -->

<CRITICAL_INSTRUCTION>

## BACKLOG WORKFLOW INSTRUCTIONS

This project uses Backlog.md MCP for all task and project management activities.

**CRITICAL GUIDANCE**

- If your client supports MCP resources, read `backlog://workflow/overview` to understand when and how to use Backlog for this project.
- If your client only supports tools or the above request fails, call `backlog.get_workflow_overview()` tool to load the tool-oriented overview (it lists the matching guide tools).

- **First time working here?** Read the overview resource IMMEDIATELY to learn the workflow
- **Already familiar?** You should have the overview cached ("## Backlog.md Overview (MCP)")
- **When to read it**: BEFORE creating tasks, or when you're unsure whether to track work

These guides cover:
- Decision framework for when to create tasks
- Search-first workflow to avoid duplicates
- Links to detailed guides for task creation, execution, and completion
- MCP tools reference

You MUST read the overview resource to understand the complete workflow. The information is NOT summarized here.

</CRITICAL_INSTRUCTION>

<!-- BACKLOG.MD MCP GUIDELINES END -->
