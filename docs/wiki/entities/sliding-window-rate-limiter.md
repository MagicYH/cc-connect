# Sliding-Window Rate Limiter

Per-key sliding-window rate limiter in `core/ratelimit.go`. Tracks message timestamps per key; rejects requests exceeding `maxMessages` within a configurable window. Background `cleanupLoop` (5-min ticker) removes stale buckets. Graceful stop via `stopCh` channel. Uses `sync.Mutex`-protected bucket map.

Used for incoming message rate limiting across all platforms.

Cross-references: [token-bucket-rate-limiter](token-bucket-rate-limiter.md), [rbac](../concepts/rbac.md)
