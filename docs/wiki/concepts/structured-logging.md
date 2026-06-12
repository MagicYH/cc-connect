# Structured Logging

The logging convention using `log/slog` exclusively (never `log.Printf` or `fmt.Printf`).

## Log Levels

- `slog.Debug` — verbose tracing (e.g. "stale user message dropped detail")
- `slog.Info` — normal operational events (e.g. "engine started")
- `slog.Warn` — degraded conditions (e.g. "platform start failed")
- `slog.Error` — unexpected failures (e.g. "SaveFilesToDisk: write failed")

## Sensitive Data Redaction

- `core.RedactArgs(args)` — masks `--api-key`, `--token` in CLI arguments
- `core.RedactEnv(env)` — masks env vars containing KEY/TOKEN/SECRET/PASSWORD
- `core.RedactToken(text, token)` — replaces a specific token with `[REDACTED]`

## Cross-References

- [error-wrapping](./error-wrapping.md) — how errors are wrapped before logging
