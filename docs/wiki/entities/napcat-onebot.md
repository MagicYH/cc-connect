# NapCat / OneBot v11

QQ messaging via the NapCat intermediary server using the OneBot v11 protocol.

## Project Relation

Platform adapter in `platform/qq/` connects to a user-hosted NapCat WebSocket server (default `ws://127.0.0.1:3001`). This is a local intermediary, not a direct QQ API. Supports file operations.

Config: `type="qq"`.

## Cross-References

- [Platform SDK](../concepts/platform-sdk.md)
- [External Dependencies Source](../sources/external-dependencies.md)
