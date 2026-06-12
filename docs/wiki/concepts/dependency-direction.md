# Dependency Direction

The strict unidirectional import rule enforced by convention across packages.

## Rule

```
cmd/       → config/, core/, agent/*, platform/*
agent/*    → core/   (never other agents or platforms)
platform/* → core/  (never other platforms or agents)
core/      → stdlib only (never agent/ or platform/)
```

## Project Relation

This rule keeps `core/` agnostic of concrete implementations. Cross-cutting concerns (i18n, cards, streaming, rate limiting) live in `core/`. The plugin registry and selective compilation patterns exist to maintain this rule while allowing extensibility.

## Cross-References

- [plugin-registry](./plugin-registry.md) — how core avoids importing agent/platform packages
- [selective-compilation](./selective-compilation.md) — blank imports that pull in implementations

From [project-structure](../sources/project-structure.md): Core has two external dependency exceptions: `gorilla/websocket` and `robfig/cron/v3`.
