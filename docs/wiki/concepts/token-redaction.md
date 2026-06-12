# Token Redaction

Systematic secret masking across logs, CLI args, and user-facing output. `RedactEnv()` masks env vars with sensitive keys. `RedactArgs()` redacts values after `--api-key`, `--token`, etc. `RedactToken()` replaces token strings with `[REDACTED]`. `redactInlineSecrets()` strips `key=value` and `Bearer` patterns. Applied on all agent launch paths and platform logging.

Cross-references: [timing-safe-comparison](timing-safe-comparison.md)
