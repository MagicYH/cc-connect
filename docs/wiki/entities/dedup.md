# Dedup

Message deduplication system in `core/dedup.go`. Prevents duplicate message processing per platform instance.

Risks: TTL eviction is O(n) on every call under write lock; empty message IDs bypass dedup entirely; `IsOldMessage` grace period is only 2 seconds post-restart, risking dropped legitimate messages under clock skew or slow restarts.

Cross-references: [session-manager](./session-manager.md)
