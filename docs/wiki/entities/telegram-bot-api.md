# Telegram Bot API

Telegram's bot messaging API at `api.telegram.org`.

## Project Relation

Platform adapter in `platform/telegram/` using `go-telegram/bot` (v1.20.0). Supports long-polling and WebSocket for message reception, reactions, file download, and proxy support.

Config: `type="telegram"`.

## Cross-References

- [Platform SDK](../concepts/platform-sdk.md)
- [External Dependencies Source](../sources/external-dependencies.md)
