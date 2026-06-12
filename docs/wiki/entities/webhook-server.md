# WebhookServer

HTTP server receiving triggers from external systems (git hooks, CI/CD, file watchers). Default port 9111, path `/hook`. Bearer token auth.

**Implementation:** `core/webhook.go` (`NewWebhookServer`)
**Config:** `[webhook]` in `config.toml`

Cross-references: [APIServer](api-server.md), [HookRunner](hook-runner.md)
