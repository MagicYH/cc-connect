# Error Wrapping

Project convention: all error wrapping uses `fmt.Errorf("context: %w", err)` with the `%w` verb to preserve error chains for `errors.Is`/`errors.As`. No third-party error libraries. ~162 occurrences in `agent/`, ~120 in `core/`. External API errors (e.g. WeCom) use flat `fmt.Errorf` without `%w`.

Cross-references: [sentinel-errors](sentinel-errors.md), [Engineering Constraints source](../sources/engineering-constraints.md), [structured-logging](structured-logging.md)

## Code Conventions Supplement

All errors include a lowercase package prefix: `fmt.Errorf("claudecode: %q CLI not found: %w", bin, err)`, `fmt.Errorf("telegram: invalid proxy: %w", err)`, `fmt.Errorf("shell: start: %w", err)`. Use `%w` for wrapping, `%v` for sentinel errors. Never silently swallow errors; at minimum log with `slog.Error`/`slog.Warn`.
