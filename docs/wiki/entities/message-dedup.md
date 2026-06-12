# Message Dedup

Deduplication layer in `core/dedup.go`. `MessageDedup` uses `sync.Mutex`-protected map of seen message IDs with 60-second TTL (`dedupTTL`). `IsOldMessage()` rejects messages from before process startup (2-second grace) to prevent replay after restart.

Platform-specific variants: Discord uses two `sync.Map` instances, WeCom uses `msgDedup` field.

Cross-references: [sliding-window-rate-limiter](sliding-window-rate-limiter.md)
