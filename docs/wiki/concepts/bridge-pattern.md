# Bridge Pattern

The `core/bridge.go` component that maps platform capabilities to agent expectations.

## What It Does

Handles capability matching between a platform and an agent session, event forwarding from agent to platform, and streaming output adaptation. At 1,420 lines, it is one of the larger core files.

## Project Relation

The bridge is created per-session and mediates all communication between a specific platform channel and a specific agent session. It queries capability interfaces on both sides to determine how to format and forward messages.

## Cross-References

- [capability-interface](./capability-interface.md) — interfaces the bridge queries
- [engine](./engine.md) — creates bridges for each session
- [project-structure](../sources/project-structure.md)
