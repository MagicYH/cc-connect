# Streaming

Real-time output delivery from agent sessions to messaging platforms.

## What It Does

As AI agents produce output incrementally (token by token), the streaming system in `core/streaming.go` handles throttled delivery to platforms. Supports both streaming cards (for platforms like Feishu that support live-updating cards) and chunked text messages (for platforms without streaming card support).

## Project Relation

Streaming is a cross-cutting concern in core. The bridge queries `StreamingCardPlatform` capability to decide delivery mode. Rate limiting (`core/outgoing_ratelimit.go`) prevents streaming from hitting platform API limits.

## Cross-References

- [bridge-pattern](./bridge-pattern.md) — routes streaming output
- [rate-limiting](./rate-limiting.md) — throttles streaming updates
- [capability-interface](./capability-interface.md) — `StreamingCard`, `StreamingCardPlatform`
- [project-structure](../sources/project-structure.md)
