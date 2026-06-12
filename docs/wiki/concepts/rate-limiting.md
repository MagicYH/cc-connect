# Rate Limiting

Two-layer rate control. **Inbound:** sliding-window per-key limiter (default 20 msgs/60s) prevents user flooding. **Outbound:** token-bucket per-platform limiter with per-platform config overrides prevents API rate limit violations.

**Implementation:** `core/ratelimit.go` (inbound), `core/outgoing_ratelimit.go` (outgoing)
**Config:** `[rate_limit]`, `[outgoing_rate_limit]` in `config.toml`

From [project-structure](../sources/project-structure.md): Implemented in `core/ratelimit.go` and `core/outgoing_ratelimit.go`.
