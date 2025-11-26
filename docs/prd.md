# mcproxy — Product Requirements Document

## Overview
mcproxy is a Go-based HTTP proxy service that registers dynamic POST endpoints at startup based on a JSON configuration. Each endpoint forwards requests to a configured upstream URL (HTTP or HTTPS), applies optional headers that may reference secrets, and returns the upstream response transparently.

## Goals
- Load a JSON config and a JSON secrets file; fail fast if either is missing or unreadable.
- Substitute `{{ var_name }}` placeholders using the secrets map; skip any endpoint with unresolved variables while logging a warning.
- Expose an HTTP server on port `8099` (configurable) that defines one POST route per configured endpoint name.
- Forward requests efficiently with connection reuse and support outbound HTTPS.
- Emit structured logs to stdout, with optional fan-out to a file path defined in config (future-ready).

## Users and Use Cases
- **Internal service owners** needing quick proxy shims to external/internal HTTP APIs without exposing secrets in config.
- **Platform engineers** orchestrating multiple upstreams behind stable, named endpoints for internal clients.

## Assumptions
- Runtime environment has Go 1.21+ available for building.
- JSON is the only supported config format in v1.
- Inbound traffic is HTTP-only; outbound may be HTTP or HTTPS.

## Functional Requirements
- **Config sources**
  - Default config path: `./config.json` relative to the working directory unless overridden by env `MCPROXY_CONFIG`.
  - Default secrets path: `~/secrets.json` unless overridden by env `MCPROXY_SECRETS`.
  - If either file is missing, unreadable, or invalid JSON, startup must exit with a clear error code and message.
- **Schema (v1)**
  - Top-level object with `logging` and `mcps`.
  - `logging` (optional):
    - `level`: string, default `info`.
    - `file`: optional string path; when set, log to both stdout and the file.
  - `mcps`: array of objects of kind `http` for v1.
    - `name` (string, required, unique): endpoint name used in route `/name`.
    - `kind` (string, required): must be `"http"` for now.
    - `url` (string, required): absolute upstream URL (http or https).
    - `headers` (object, optional): key/value pairs; values may contain `{{ var_name }}` placeholders.
- **Secret substitution**
  - Secrets file is a flat JSON object mapping names to string values.
  - For each header value, replace all `{{ var_name }}` occurrences with the corresponding secret value.
  - If any placeholder cannot be resolved, log a `warn` with the endpoint name and missing variable, and skip registering that endpoint.
- **Endpoint registration**
  - For each valid `mcps` entry, register an HTTP `POST` handler at `/ENDPOINT_NAME`.
  - Reject non-POST methods with 405.
  - On each request:
    - Forward the body as-is to the upstream `url`.
    - Forward headers: start with incoming headers, overlay configured headers (after substitution), and ensure `Host` is derived from upstream.
    - Preserve response status, headers, and body back to caller.
- **HTTP client behavior**
  - Use a shared `http.Client` with keep-alives enabled, reasonable timeouts, and default TLS config (system roots).
  - Follow redirects off by default (return upstream redirect responses directly).
- **Logging**
  - Structured logs with fields: level, ts, msg, endpoint, method, path, status, duration_ms, missing_secret (when applicable).
  - Startup logs summarizing loaded endpoints and those skipped.
  - Errors should include enough context to debug (file path, reason).

## Non-Functional Requirements
- **Performance**: Handle at least low hundreds of RPS on modest hardware; rely on connection reuse to reduce latency.
- **Reliability**: Startup fails fast on invalid config; partial availability allowed when some endpoints are skipped.
- **Security**: Secrets remain only in memory; never log secret values. Templated headers must be fully substituted before use.
- **Observability**: Logging only in v1; metrics/tracing out of scope.
- **Portability**: Should build and run on Linux/macOS.

## Out of Scope (v1)
- Authentication/authorization on inbound routes.
- Rate limiting, caching, or request mutation beyond headers.
- Config hot-reload.
- TLS termination for inbound traffic (listener is HTTP only).
- Alternate config formats (TOML/YAML).

## Open Questions
- Should we allow templating in the upstream URL itself?
- Should timeouts and max idle connections be configurable per endpoint?
- Do we need per-endpoint overrides for redirect behavior?
