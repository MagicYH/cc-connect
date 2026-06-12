# Redact

Utility for redacting secrets from logs. Lives in `core/redact.go`. Provides `RedactEnv` and `RedactArgs` for CLI arguments.

Risk: `sensitiveKeys` list (`KEY`, `TOKEN`, `SECRET`, `PASSWORD`, `CREDENTIAL`) misses `AWS_ACCESS_KEY_ID`, `PRIVATE_KEY`, `AUTH`, `COOKIE`, `SESSION`, and provider-specific keys. Does not cover HTTP API response bodies from OAuth/token endpoints (qqbot, dingtalk include token bodies in error messages).

Cross-references: [bridge-server](./bridge-server.md)
