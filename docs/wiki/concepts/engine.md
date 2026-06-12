# Engine

The central orchestrator in `core/engine.go` — the largest and most critical file in CC-Connect.

## What It Does

Handles message routing between platforms and agents, session lifecycle (create, resume, prune, cancel), permission request/response flow, streaming output with throttling, multi-workspace binding, slash command routing, hook system, provider/model management, TTS, rate limiting, cron delegation, and timer tasks.

## Project Relation

At 15,578 lines, `engine.go` far exceeds the project's 800-line file guideline. It is the most actively changed file (13 changes in last 50 commits). All platform and agent interactions flow through the Engine.

## Cross-References

- [plugin-architecture](./plugin-architecture.md) — how Engine creates agents/platforms
- [capability-interface](./capability-interface.md) — how Engine queries optional interfaces
- [project-structure](../sources/project-structure.md)
