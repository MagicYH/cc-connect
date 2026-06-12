# cc-switch Database

SQLite database owned by the external `cc-switch` tool. cc-connect reads provider configurations from it in read-only mode. Schema is not owned by cc-connect and may change independently.

**Location:** `~/.cc-switch/cc-switch.db` (XDG-aware)
**Driver:** `modernc.org/sqlite` (pure-Go)
**Initialized:** `cmd/cc-connect/provider.go:317`

Cross-references: [ProviderProxy](provider-proxy.md)
