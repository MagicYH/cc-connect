# APIServer

HTTP server on a Unix domain socket (`api.sock`). Provides local endpoints `/send` and `/relay/send` for cron jobs and `cc-connect send` CLI. Secured by file permissions (0600).

**Implementation:** `core/api.go:45` (`NewAPIServer`)
**Listen:** `<data_dir>/run/api.sock`

Cross-references: [ManagementServer](management-server.md), [WebhookServer](webhook-server.md)
