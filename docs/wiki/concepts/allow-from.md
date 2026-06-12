# Allow From

Access control list determining which users can interact with the bot. Defined in `core/message.go`.

`AllowList` returns `true` when `allowFrom` is empty or `"*"`, meaning all users are permitted by default (fail-open). In contrast, `isAdmin` returns `false` when `adminFrom` is empty (fail-closed). A fresh install without explicit `allow_from` configuration is open to all users.

Project relation: `core/message.go:39-44`, config.toml `allow_from` field.

Cross-references: [bridge-server](../entities/bridge-server.md), [web-admin-token](./web-admin-token.md)
