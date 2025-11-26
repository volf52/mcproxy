# mcproxy

A lightweight Go service that publishes dynamic HTTP POST endpoints and proxies them to configured upstream HTTP/HTTPS targets. Configuration and secrets are JSON-based; header values can reference secrets using `{{ var_name }}` placeholders.

## Quick Start
- Prerequisites: Go 1.21+.
- Defaults:
  - Config file: `./config.json` (override with env `MCPROXY_CONFIG`).
  - Secrets file: `~/secrets.json` (override with env `MCPROXY_SECRETS`).
  - Listen address: `:8099`.
- Build and run:
```bash
go build -o mcproxy
./mcproxy
```

## Config Shapes (JSON)
### Secrets (`~/secrets.json` by default)
```json
{
  "api_key": "super-secret-token",
  "x_user": "alice@example.com"
}
```

### Config (`./config.json` by default)
```json
{
  "logging": {
    "level": "info",
    "file": "/var/log/mcproxy.log"
  },
  "mcps": [
    {
      "name": "billing-api",
      "kind": "http",
      "url": "https://billing.internal.local/v1/charge",
      "headers": {
        "Authorization": "Bearer {{ api_key }}",
        "X-User": "{{ x_user }}"
      }
    }
  ]
}
```

## Behavior Highlights
- Startup fails if config or secrets file is missing or invalid JSON.
- For each `mcps` entry of kind `http`, a POST handler is registered at `/{name}` that forwards to `url`.
- Header values are templated with secrets; endpoints with missing variables are skipped with a warning.
- Uses a shared `http.Client` with keep-alives; outbound HTTPS is supported automatically.
- Structured logs go to stdout; when `logging.file` is set, logs also tee to that file.

## Docs
- Project description: `docs/project-description.md`
- Product requirements: `docs/prd.md`

## Next Steps
- Implement config parsing and validation.
- Add templating and endpoint registration.
- Introduce integration tests that cover happy path, missing secret, and HTTPS upstream cases.
