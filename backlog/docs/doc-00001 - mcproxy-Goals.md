---
id: doc-00001
title: mcproxy Goals
type: other
created_date: '2025-11-26 23:07'
---
## mcproxy Project Goals

- Deliver a lightweight Go HTTP proxy that dynamically publishes POST endpoints from JSON config.
- Support outbound HTTPS from day one; expose inbound HTTP on port 8099 by default.
- Keep secrets separate: resolve `{{ var_name }}` placeholders in config headers using a secrets JSON file; skip endpoints with missing secrets.
- Fail fast if config or secrets are missing/invalid; partial availability allowed when some endpoints are skipped.
- Use a shared HTTP client with connection reuse and sensible timeouts.
- Provide structured logging to stdout, and allow optional file fan-out via config.
- Keep JSON-only config in v1; future-proof for other formats and inbound TLS later.
