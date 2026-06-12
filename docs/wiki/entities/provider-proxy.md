# ProviderProxy

Local reverse proxy on an ephemeral port that rewrites incompatible Anthropic API fields (e.g., `thinking.type: "adaptive"`) for third-party providers. One proxy per Claude Code agent instance. Started/stopped per-agent lifecycle.

**Implementation:** `core/providerproxy.go:36` (`NewProviderProxy`)
**Listen:** `127.0.0.1:<random-port>`

Cross-references: [cc-switch Database](cc-switch-database.md)
