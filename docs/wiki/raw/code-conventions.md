# Code Conventions

## Code Organization Rules

### Plugin Architecture via Registries

All agents and platforms register themselves through `core.RegisterAgent()` / `core.RegisterPlatform()` in their `init()` functions. The engine creates instances via `core.CreateAgent()` / `core.CreatePlatform()` using string names from config.

- Registration happens in `init()`: `core.RegisterAgent("claudecode", New)` (e.g. `agent/claudecode/claudecode.go:24`)
- Factory signature: `func New(opts map[string]any) (core.Agent, error)` or `func New(opts map[string]any) (core.Platform, error)`
- Feishu registers two names from one package: `core.RegisterPlatform("feishu", ...)` and `core.RegisterPlatform("lark", ...)` (`platform/feishu/feishu.go:101-104`)

### Selective Compilation via Build Tags

Each agent/platform is imported via a separate `plugin_*.go` file in `cmd/cc-connect/`:

- File naming: `plugin_agent_<name>.go` and `plugin_platform_<name>.go`
- Build tag pattern: `//go:build !no_<name>` (negative opt-out)
- Content: blank import only — `import _ "github.com/chenhg5/cc-connect/agent/claudecode"`
- Example: `cmd/cc-connect/plugin_agent_claudecode.go` with tag `!no_claudecode`

### Dependency Direction

Strict unidirectional dependency rule enforced by convention:

```
cmd/ → config/, core/, agent/*, platform/*
agent/*   → core/   (never other agents or platforms)
platform/* → core/  (never other platforms or agents)
core/     → stdlib only (never agent/ or platform/)
```

The `core/` package must never import from `agent/` or `platform/`. Cross-cutting concerns (i18n, cards, streaming, rate limiting) live in `core/`.

### Structural Typing via Optional Interfaces

Instead of type switches or hardcoded names, the codebase defines small optional interfaces in `core/interfaces.go`. Platforms and agents opt in by implementing the relevant interface:

```go
// In core/interfaces.go
type ImageSender interface {
    SendImage(ctx context.Context, replyCtx any, img ImageAttachment) error
}

// In platform/telegram/telegram.go — implements ImageSender
func (p *Platform) SendImage(ctx context.Context, rctx any, img core.ImageAttachment) error { ... }

// In core/engine.go — capability check
if sender, ok := p.(core.ImageSender); ok {
    sender.SendImage(ctx, replyCtx, img)
}
```

There are ~25 optional capability interfaces defined in `core/interfaces.go`, including `CardSender`, `TypingIndicator`, `ProviderSwitcher`, `ModelSwitcher`, `StreamingCardPlatform`, etc.

## Coding Style & Patterns

### Error Wrapping Convention

All errors are wrapped with a lowercase context prefix using `fmt.Errorf`, with the package/component name as prefix:

- Agent errors: `fmt.Errorf("claudecode: %q CLI not found in PATH, please install it first", cliBin)` (`agent/claudecode/claudecode.go:198`)
- Platform errors: `fmt.Errorf("telegram: invalid proxy URL %q: %w", proxyURL, err)` (`platform/telegram/telegram.go:151`)
- Engine errors: `fmt.Errorf("shell: start: %w", err)` (`core/engine.go:1617`)
- Errors that wrap underlying causes use `%w`; sentinel errors use `%v` or plain text.

### Structured Logging with slog

All runtime logging uses `log/slog` (never `log.Printf` or `fmt.Printf`). Log levels follow a clear convention:

- `slog.Debug` — verbose tracing (e.g. "stale user message dropped detail", "session launch args")
- `slog.Info` — normal operational events (e.g. "engine started", "platform ready")
- `slog.Warn` — degraded conditions (e.g. "platform start failed", "allow_from is not set")
- `slog.Error` — unexpected failures (e.g. "SaveFilesToDisk: write failed")

Sensitive data is redacted before logging:
- `core.RedactArgs(args)` — masks `--api-key`, `--token`, etc. in CLI arguments
- `core.RedactEnv(env)` — masks values of env vars containing KEY/TOKEN/SECRET/PASSWORD
- `core.RedactToken(text, token)` — replaces a specific token string with `[REDACTED]`

### Concurrency Safety Patterns

- Agent sessions are accessed from multiple goroutines; shared state protected with `sync.RWMutex` (`agent/claudecode/claudecode.go:65`, `platform/telegram/telegram.go:115`)
- `defer mu.Unlock()` / `defer mu.RUnlock()` is the standard pattern
- `sync.Once` for one-time teardown operations (e.g. `closeOnce` in session close: `agent/codex/session.go:42`, `core/engine.go:483`)
- `context.Context` passed as first argument to all methods that may block or need cancellation
- Channel ownership is documented; `Session.TryLock()` / `Session.Unlock()` pattern for session busy-state

### Internationalization (i18n)

All user-facing strings go through `core/i18n.go`:

1. Define a `MsgKey` constant: `MsgStarting MsgKey = "starting"`
2. Add translations for all 5 languages (EN, ZH, ZH-TW, JA, ES)
3. Use `e.i18n.T(MsgKey)` or `e.i18n.Tf(MsgKey, args...)` in engine code

The `I18n` struct itself is concurrency-safe with `sync.RWMutex`.

### Builder Pattern for Cards

Rich messages use a fluent `CardBuilder` API:

```go
card := NewCard().
    Title(e.i18n.T(MsgCardTitleStatus), "blue").
    Markdown(content).
    Divider().
    Buttons(PrimaryBtn("OK", "cmd:/ok")).
    Note("footer text").
    Build()
```

Each element type (`CardMarkdown`, `CardDivider`, `CardActions`, `CardNote`, `CardListItem`, `CardSelect`) implements the unexported `cardElement()` marker interface. `Card.RenderText()` provides a plain-text fallback for platforms without card support.

### Type Assertion from map[string]any

Since factories receive `map[string]any` from TOML config, the standard extraction pattern is:

```go
token, _ := opts["token"].(string)
allowFrom, _ := opts["allow_from"].(string)
```

For numeric types with TOML ambiguity (int vs float64):

```go
switch v := opts["max_context_tokens"].(type) {
case int:
    maxContextTokens = v
case int64:
    maxContextTokens = int(v)
case float64:
    maxContextTokens = int(v)
}
```

Example: `agent/claudecode/claudecode.go:160-173`

### Constructor Naming

- Package-level factory: `func New(opts map[string]any) (core.Agent, error)` or `func New(opts map[string]any) (core.Platform, error)`
- Engine constructor: `func NewEngine(name string, ag Agent, platforms []Platform, ...) *Engine`
- Registry constructors: `func NewCommandRegistry() *CommandRegistry`, `func NewSessionManager(...) *SessionManager`
- Helper constructors: `func NewI18n(lang Language) *I18n`, `func NewCard() *CardBuilder`

### Receiver Naming

- Platform structs use `p` receiver: `func (p *Platform) Reply(...)`
- Agent structs use `a` receiver: `func (a *Agent) Name()`
- Engine uses `e` receiver: `func (e *Engine) Start(...)`

## API Naming Conventions

### Slash Commands (IM Bot Interface)

User-facing commands use kebab-case with `/` prefix. Commands support prefix matching (e.g. `/pro l` = `/provider list`):

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

### Internal Method Naming

Engine methods follow a consistent verb-noun pattern:

- `Set*` / `Get*` — configuration accessors (e.g. `SetModel`, `GetModel`, `SetWorkDir`, `GetWorkDir`)
- `Handle*` — message processing (e.g. `HandleMessage` via `MessageHandler` callback)
- `Send*` / `Reply*` — output methods
- `Add*` / `Remove*` / `Delete*` — collection mutations

### Platform Interface Methods

All platforms implement the core `Platform` interface with 5 methods:

```go
type Platform interface {
    Name() string
    Start(handler MessageHandler) error
    Reply(ctx context.Context, replyCtx any, content string) error
    Send(ctx context.Context, replyCtx any, content string) error
    Stop() error
}
```

Optional capabilities add methods following the same pattern: `SendCard`, `ReplyCard`, `SendImage`, `SendFile`, `SendWithButtons`, `UpdateMessage`, etc.

### Agent Interface Methods

All agents implement the core `Agent` interface:

```go
type Agent interface {
    Name() string
    StartSession(ctx context.Context, sessionID string) (AgentSession, error)
    ListSessions(ctx context.Context) ([]AgentSessionInfo, error)
    Stop() error
}
```

Agent sessions implement:

```go
type AgentSession interface {
    Send(prompt string, images []ImageAttachment, files []FileAttachment) error
    RespondPermission(requestID string, result PermissionResult) error
    Events() <-chan Event
    CurrentSessionID() string
    Alive() bool
    Close() error
}
```

### Config TOML Naming

Config keys use `snake_case` consistently:

- Top-level: `data_dir`, `attachment_send`, `language`
- Nested sections: `[log]`, `[display]`, `[speech]`
- Project entries: `[[projects]]` with `agent_type`, `work_dir`, `cli_path`
- Platform options: `token`, `allow_from`, `proxy`, `group_reply_all`

### Test Conventions

- Test files co-located with source (e.g. `engine.go` + `engine_test.go`)
- Stub types named `stub*` (e.g. `stubAgent`, `stubPlatform`, `stubAgentSession`) — no mock frameworks
- Test functions named `Test<Entity>_<Scenario>` (e.g. `TestEngineSendToSessionWithAttachments_UnsupportedPlatform`)
- Table-driven tests preferred; `*testing.T` only (no testify assertions in core tests)

## Scan Warnings

- `core/engine.go` is 15,578 lines with 338 methods on `Engine`, far exceeding the ~800-line file guideline. This is a known architectural concern but not addressed by this scan.
- `platform/feishu/feishu.go` is 6,099 lines — also well above the 800-line guideline.
- A few instances of platform name hardcoding in `core/engine.go` (e.g. `strings.EqualFold(p.Name(), "telegram")` at line 2177, `!strings.EqualFold(platformName, "telegram")` at line 8706) — these appear to be special-case handling for platform-specific behavior that could not be expressed via optional interfaces.
- The `weibo` platform uses `p.name` (instance variable) instead of a hardcoded string for `Name()`, unlike all other platforms which return a constant string. This is the only platform that does this.
