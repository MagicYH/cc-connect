# Command Registry

The registry in `core/` that manages slash commands for the IM bot interface.

## Project Relation

Commands use kebab-case with `/` prefix: `/new`, `/list`, `/switch`, `/model`, `/cron add|list|exec|del`, `/timer add|list|del`, `/provider list|add|remove|switch`. Supports prefix matching (e.g. `/pro l` = `/provider list`). Created via `NewCommandRegistry()`.

## Cross-References

- [slash-commands](../concepts/slash-commands.md) — full command reference and pattern
