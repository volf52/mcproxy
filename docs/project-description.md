# mcproxy — Project Description

mcproxy is a small Go service that exposes dynamic HTTP POST endpoints that proxy requests to configured upstream HTTP/HTTPS destinations. Configuration and secrets are provided as JSON files. Secrets can be referenced inside configuration values using `{{ var_name }}` placeholders and are substituted at startup. Invalid or missing secrets prevent the affected proxy from being registered but do not block the service from starting other valid proxies.

## Objectives
- Provide a lightweight, configurable HTTP proxy layer that can publish multiple named POST endpoints at runtime.
- Keep configuration simple (JSON only) while allowing secret substitution for headers and future fields.
- Support outbound HTTPS (TLS) connections from day one; serve clients over plain HTTP on port `8099` by default.
- Make logging structured and ready for fan-out (stdout plus optional file sink) without external dependencies.

## Key Components
- **Configuration loader**: Reads the required config JSON file plus a required secrets JSON file (default paths, env overrides). Validates presence; exits with a clear error if either is missing or unreadable.
- **Template resolver**: Replaces `{{ var_name }}` placeholders in header values (and future template-enabled fields) using the secrets map. Logs a warning and skips proxies with unresolved variables.
- **Proxy registry**: Registers a POST handler for each valid endpoint name. Dispatches requests to the configured upstream URL with merged headers and streamed body.
- **Transport layer**: Uses a shared `http.Client` with connection reuse and sensible timeouts; supports HTTPS automatically via Go’s standard TLS stack.
- **Logging**: Structured logs to stdout with an optional file destination once configured; log levels at least `info` and `warn` for v1.

## Non-Goals (v1)
- No UI or CLI beyond configuration files and environment variables.
- No authentication/authorization on incoming requests (can be added later).
- No rate limiting or request shaping.
- No metrics/observability beyond logging.

## Future-Friendly Hooks
- Allow TOML or YAML configs later by plugging additional decoders.
- Add upstream authentication schemes (e.g., Basic, Bearer) using the same secret templating model.
- Introduce optional HTTPS listener for inbound traffic.
