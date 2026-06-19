# Session ID in Reply Footer

**Date:** 2026-06-19
**Status:** Approved

## Problem

The final reply card footer shows the working directory but not the agent session ID. Users cannot correlate a reply in their messaging app with agent-side logs or transcript files.

## Solution

Append the agent session ID to the working directory line in the status footer, separated by ` · `.

**Before:**
```
~/projects/foo
```

**After:**
```
~/projects/foo · abc123
```

The session ID shown is the agent-side session ID (`AgentSession.CurrentSessionID()`), e.g. Claude Code's session UUID. This matches the identifier used in agent transcript file paths, enabling correlation between a messaging app reply and the corresponding agent transcript.

## Implementation

Modify `replyFooterWorkDir()` in `core/engine.go` (line 6997):

1. After resolving the workdir path, call `session.CurrentSessionID()` to get the agent session ID — guard with `session != nil` to avoid panic
2. If both workdir and session ID are non-empty, return `compactPath(dir) · sessionID`
3. If only workdir exists, return `compactPath(dir)` (current behavior)
4. If workdir is empty, return `""` (current behavior)

No config changes needed — the existing `showWorkdirIndicator` toggle controls visibility. Session ID visibility is intentionally coupled to workdir visibility: a session ID without workdir context is not useful for correlation.

## Degradation

- If `session` is nil, no session ID is shown (nil guard prevents panic).
- If `session.CurrentSessionID()` returns empty (e.g. agent hasn't emitted session ID yet), only the workdir is shown. This can happen for early-turn failures where the agent process exits before reporting its session ID.
- In the legacy footer path (`buildReplyFooter`), the entire footer is suppressed when only a workdir line is present and no model/context line exists (#701). Session ID is suppressed along with it — this is expected behavior.

## Scope

All four rendering paths automatically inherit the change through `replyFooterWorkDir()`:

| Path | Platform | Mechanism |
|------|----------|-----------|
| Rich card | Feishu | `composeRichStatusFooter` → `replyFooterWorkDir` |
| CCD statusline | Feishu | `buildClaudeStatusLineFooter` → `replyFooterWorkDir` |
| Legacy footer | Others | `buildReplyFooter` → `replyFooterWorkDir` |
| Inline append | Telegram/Discord/etc | Receives pre-computed footer string from above paths |

## Notes

- Session ID is shown raw (no truncation). Format varies by agent: Claude Code uses short hex, Codex uses `thread_*` prefixes, etc.
- Session IDs are not secrets but are part of local transcript file paths. Showing them in shared channels (group chats) is acceptable since the ID alone doesn't grant filesystem access.
- The ` · ` separator is consistent with existing footer formatting (model · effort · tokens) and is not i18n'd.

## Testing

Add test cases for `replyFooterWorkDir` with a mock `AgentSession` that returns a `CurrentSessionID`:
- Both workdir and session ID present → `dir · sid`
- Only workdir present → `dir`
- Only session ID present (no workdir) → `""`
- Both empty → `""`
- Nil session → no panic, returns workdir only
