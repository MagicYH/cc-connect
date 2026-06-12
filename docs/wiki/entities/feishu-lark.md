# Feishu / Lark

Official messaging platform by ByteDance, available as Feishu (China) and Lark (international).

## Project Relation

Primary messaging platform adapter in `platform/feishu/`. Uses the official Go SDK `larksuite/oapi-sdk-go/v3` (v3.5.3) for WebSocket message reception and REST API calls. Supports interactive cards, reactions, and file download.

Config: `type="feishu"`, domain configurable for Feishu vs Lark endpoints.

## Cross-References

- [Platform SDK](../concepts/platform-sdk.md)
- [External Dependencies Source](../sources/external-dependencies.md)
