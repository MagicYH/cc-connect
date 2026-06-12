# Self-Hosted Service

HTTP/WebSocket services that cc-connect itself listens on, as opposed to external APIs it connects to.

## Project Relation

Four self-hosted services: Webhook Server (port 9111, git hooks/CI), Bridge Platform (port 9810, external adapters), Management API (port 9820, web dashboards/TUI), and Codex App Server (port 3845, internal loopback for Codex sandbox). All are configurable in `config.example.toml`.

## Cross-References

- [External Dependencies Source](../sources/external-dependencies.md)
