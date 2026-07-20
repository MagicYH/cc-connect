# Team Roster Append Prompt Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move team roster injection from overriding `system_prompt` to appending via `append_system_prompt`, expose append prompt in WebUI, and add a configurable team roster injection switch.

**Architecture:** Team roster becomes a generated append-only prompt fragment. Persisted user prompt remains in `agent.options.append_system_prompt`; generated roster is merged at startup into runtime agent options without writing back to config.

**Tech Stack:** Go config/management API, React TypeScript WebUI, Go unit tests.

## Global Constraints

- Do not overwrite Claude Code default system prompt for team roster injection.
- Existing team projects keep roster injection enabled by default.
- Empty `append_system_prompt` clears the persisted option.
- Settings that affect startup return `restart_required`.

---

### Task 1: Backend config/API and runtime injection

**Files:**
- Modify: `core/team.go`
- Modify: `cmd/cc-connect/main.go`
- Modify: `config/config.go`
- Modify: `core/management.go`
- Test: `core/team_test.go`
- Test: `config/config_test.go`

**Interfaces:**
- Produces: `TeamRegistry.RosterPrompt(selfProject, team, memberDescribe string) string`
- Produces: `ProjectConfig.TeamRosterEnabled *bool`
- Produces: `ProjectSettingsUpdate.AppendSystemPrompt *string`
- Produces: `ProjectSettingsUpdate.TeamRosterEnabled *bool`

- [ ] Write failing Go tests for roster-only prompt and config save.
- [ ] Implement minimal backend changes.
- [ ] Run `go test ./core ./config`.

### Task 2: WebUI settings

**Files:**
- Modify: `web/src/api/projects.ts`
- Modify: `web/src/pages/Projects/ProjectDetail.tsx`

**Interfaces:**
- Consumes API fields: `append_system_prompt?: string`, `team_roster_enabled?: boolean`.
- Sends PATCH fields: `append_system_prompt`, `team_roster_enabled`.

- [ ] Update TypeScript API types.
- [ ] Add settings state and controls.
- [ ] Include fields in save payload.
- [ ] Run WebUI type/build command.

### Task 3: Full verification

- [ ] Run `go test ./...`.
- [ ] Run frontend build or typecheck.
- [ ] Summarize changes and restart/deploy implications.
