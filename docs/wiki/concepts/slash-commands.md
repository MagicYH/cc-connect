# Slash Commands

User-facing bot commands using kebab-case with `/` prefix, supporting prefix matching.

## Command Reference

| Command | Purpose |
|---------|---------|
| `/new` | Start new session |
| `/list` | List sessions |
| `/switch <n>` | Switch to session by number |
| `/delete <n>` | Delete sessions |
| `/model [switch <name>]` | View/switch model |
| `/mode [name]` | View/switch permission mode |
| `/cron add\|list\|exec\|del` | Scheduled tasks |
| `/timer add\|list\|del` | One-shot timers |
| `/provider list\|add\|remove\|switch` | API provider management |

## Project Relation

Commands are registered in the `CommandRegistry` and processed by the engine. Prefix matching allows abbreviated commands (e.g. `/pro l` = `/provider list`).

## Cross-References

- [command-registry](../entities/command-registry.md) — the registry managing these commands
