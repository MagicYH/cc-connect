# Streaming Card Initial Footer Design

## Status

Approved in chat on 2026-07-09.

## Goal

Feishu streaming cards should show useful footer metadata as soon as the card first appears, instead of keeping the footer empty until the final turn update. The initial footer must include the model name, working directory, and session identifier when those values are available.

## Scope

In scope:

- Feishu rich/streaming card rendering only.
- Initial in-progress card footer content.
- Final card footer replacement using the existing completion-time footer.

Out of scope:

- Non-Feishu platforms.
- Non-rich-card/plain-text reply footer behavior.
- Token, elapsed-time, or context-window calculation changes.
- New user-facing configuration.

## Current Behavior

During rich-card streaming, `core.Engine` builds each in-progress card through `RichCardSupporter.BuildRichCard(...)` and passes `e.composeRichStatusFooter(true, turnStart, e.agent, state.agentSession, state.workspaceDir)` as `statusFooter`. `composeRichStatusFooter` returns an empty string whenever `streaming == true`, so the Feishu card footer slot remains empty until the final `streaming == false` update.

At completion, `composeRichStatusFooter(false, ...)` builds the final footer containing elapsed time plus model/token/context/workdir details, and Feishu renders that final footer normally. On the current `shadow` baseline, `replyFooterWorkDir` already appends `AgentSession.CurrentSessionID()` to the workdir line when available, so the final rich footer already preserves the session identifier as `cwd · session`.

## Proposed Design

Change `composeRichStatusFooter` so the streaming path returns a lightweight initial footer instead of always returning an empty string. The completion path stays unchanged.

The streaming footer should contain only stable metadata that is expected to be available before result-token accounting settles:

```text
model: <model> · session: <session>
cwd: <workdir>
```

Rules:

- Respect the existing master `replyFooterEnabled` toggle.
- Include model only when `showContextIndicator` is enabled and a model is available.
- Include workdir only when `showWorkdirIndicator` is enabled and a directory is available.
- Include session through `replyFooterWorkDir` on the workdir line when `showWorkdirIndicator` is enabled and both workdir and session id are available, matching the existing final footer behavior.
- Omit missing fields instead of showing empty labels.
- Return an empty footer if all fields are missing.
- Keep final footer behavior exactly as it is today.

## Components

### `composeRichStatusFooter`

Keep the existing non-streaming branch. Replace the current early `if streaming { return "" }` with a call to a small helper that composes initial metadata.

### Initial footer helper

Add a focused helper near the existing footer helpers in `core/engine.go`. It should:

- Read model through `replyFooterModel(session, agent)`.
- Read workdir and session together through `replyFooterWorkDir(session, agent, workspaceDir)` so the initial footer matches the final footer's existing `cwd · session` semantics.
- Join line-one fields with ` · `.
- Put workdir/session on line two as `cwd: <compact-workdir> · <session>` when both are available, or `cwd: <compact-workdir>` when only the directory is available.

No new interface is required because `AgentSession.CurrentSessionID()` already exists in `core/interfaces.go`.

### Feishu renderer

No structural change is needed in Feishu card rendering. `BuildRichCard` already accepts a `statusFooter` string, and the Feishu renderer already renders non-empty footer lines as dim notation blocks.

## Data Flow

1. A Feishu turn starts and the engine creates or patches in-progress rich cards.
2. On every in-progress rich-card rebuild, the engine calls `composeRichStatusFooter(streaming=true, ...)`.
3. The streaming branch rebuilds the initial footer from the currently known model, workdir, and session metadata. If a session id appears after the first patch, later streaming patches can show it without a separate API call.
4. Feishu `BuildRichCard` receives the non-empty footer and includes it in the card immediately.
5. When the turn completes, the existing `composeRichStatusFooter(streaming=false, ...)` branch builds the final footer.
6. The final rich-card update replaces the initial footer with final elapsed/token/context/workdir/session metadata.

## Error Handling

- Missing model, workdir, or session values are normal. The helper skips unavailable fields.
- If all fields are missing, the helper returns an empty string, preserving current empty-footer behavior.
- If footer rendering fails downstream, existing card send/update error handling remains responsible; this design does not add new network operations.

## Testing

Add focused Go unit coverage for the footer composition behavior:

1. Streaming rich footer includes model, session, and cwd when all are available.
2. Streaming rich footer respects `replyFooterEnabled=false` by returning empty.
3. Streaming rich footer omits missing fields without empty labels or extra separators.
4. Non-streaming rich footer output remains governed by the existing final footer behavior.

If existing tests cover `BuildRichCard` footer rendering, update or add a small assertion that a non-empty streaming `statusFooter` is rendered by Feishu as footer notation.

## Risks and Mitigations

- **Risk:** Session id may not be populated at the very first streaming update.
  **Mitigation:** Recompute the initial footer on each streaming rich-card rebuild and reuse `replyFooterWorkDir`; omit the session until `CurrentSessionID()` becomes non-empty.

- **Risk:** The initial footer may be too long when paths or model names are long.
  **Mitigation:** Reuse `replyFooterWorkDir`, which already compacts workspace paths for footer display.

- **Risk:** Token/context information could be stale during streaming.
  **Mitigation:** Do not include token, elapsed, or context-window data in the initial footer.

## Adversarial Review Resolution

An independent review raised three must-fix points and several should-fix points. Resolutions:

- **Session continuity:** Addressed by basing implementation on the `shadow` branch, where final `replyFooterWorkDir` already includes `cwd · session`. The initial footer reuses the same helper instead of inventing separate session formatting.
- **Streaming recomputation:** Addressed by requiring `composeRichStatusFooter(streaming=true, ...)` to rebuild the initial footer on every in-progress rich-card rebuild.
- **`CurrentSessionID()` availability:** Verified on the `AgentSession` interface in `core/interfaces.go`; no new interface is needed.
- **Session truncation/hash:** Not adopted. Existing `2026-06-19-session-id-in-footer-design.md` establishes raw session id display as acceptable because the id alone does not grant filesystem access; this design reuses that established behavior.
- **Multi-line Feishu rendering:** Verified in Feishu `BuildRichCard` comments and implementation: the renderer splits pre-composed `statusFooter` on `\n` and emits one dim notation block per non-empty line.

## Acceptance Criteria

- Initial Feishu streaming cards show footer metadata immediately when model, workdir, or session values are available.
- Later streaming updates recompute the footer, so a session id that becomes available after the first card can appear before finalization.
- Final Feishu cards still show the existing completion footer and do not duplicate the initial footer.
- Footer toggles continue to apply: master footer off disables all rich-card footer output, context/model visibility follows `showContextIndicator`, and workdir visibility follows `showWorkdirIndicator`.
- Missing metadata does not produce malformed output.
