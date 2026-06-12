# Token-Bucket Rate Limiter

Per-platform outgoing message rate limiter in `core/outgoing_ratelimit.go`. Configurable `MaxPerSecond` with per-platform overrides. `Wait()` blocks until a token is available, respecting `context.Context` cancellation. Buckets lazily created; burst defaults to `ceil(MaxPerSecond)`.

Throttles outbound API calls to platforms to avoid rate-limit errors.

Cross-references: [sliding-window-rate-limiter](sliding-window-rate-limiter.md), [rbac](../concepts/rbac.md)
