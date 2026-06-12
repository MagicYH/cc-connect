# Code Conventions

[Raw scan](../raw/code-conventions.md)

## Code Organization

### Plugin Architecture via Registries
Agents and platforms self-register in `init()` via `core.RegisterAgent()` / `core.RegisterPlatform()`. Factories take `map[string]any` opts and return interface types. The engine instantiates by name from config.

### Selective Compilation via Build Tags
Each agent/platform has a `plugin_*.go` in `cmd/cc-connect/` with `//go:build !no_<name>` tag containing a blank import. Build with `make build AGENTS=... PLATFORMS_INCLUDE=...` or `EXCLUDE=...`.

### Dependency Direction
Strict rule: `core/` imports only stdlib. `agent/*` and `platform/*` import `core/` only, never each other. `cmd/` imports everything.

### Structural Typing via Optional Interfaces
~25 optional capability interfaces in `core/interfaces.go` (e.g. `CardSender`, `ImageSender`, `TypingIndicator`). Code checks via type assertion: `if sender, ok := p.(core.ImageSender); ok { ... }`. No type switches or hardcoded names in core.

## Coding Style

### Error Wrapping
All errors wrapped with lowercase package prefix: `fmt.Errorf("telegram: invalid proxy: %w", err)`. Use `%w` for wrapping, `%v` for sentinel errors.

### Structured Logging
`log/slog` exclusively (never `log.Printf`). Levels: Debug=tracing, Info=ops, Warn=degraded, Error=failures. Sensitive data redacted via `core.RedactArgs()`, `core.RedactEnv()`, `core.RedactToken()`.

### Concurrency Safety
`sync.RWMutex` for shared state, `sync.Once` for teardown, `context.Context` as first arg on blocking methods. `defer mu.Unlock()` is standard.

### i18n
User-facing strings use `MsgKey` constants + `e.i18n.T(key)` / `e.i18n.Tf(key, args...)`. 5 languages: EN, ZH, ZH-TW, JA, ES.

### Builder Pattern for Cards
Fluent `CardBuilder`: `NewCard().Title(...).Markdown(...).Divider().Buttons(...).Build()`. Elements implement unexported `cardElement()` marker. `Card.RenderText()` for plain-text fallback.

### Config Type Assertion
Factory opts extracted via type assertion: `token, _ := opts["token"].(string)`. Numeric types handle TOML ambiguity with switch on `int`/`int64`/`float64`.

## Naming Conventions

### Constructors
- Package factory: `func New(opts map[string]any) (core.Agent, error)`
- Engine: `NewEngine(...)`
- Helpers: `NewI18n()`, `NewCard()`, `NewCommandRegistry()`

### Receivers
- Platform: `p` — `func (p *Platform) Reply(...)`
- Agent: `a` — `func (a *Agent) Name()`
- Engine: `e` — `func (e *Engine) Start(...)`

### Slash Commands
kebab-case with `/` prefix: `/new`, `/list`, `/switch`, `/model`, `/cron add|list|exec|del`, `/provider list|add|remove|switch`.

### Engine Methods
- `Set*`/`Get*` — accessors
- `Handle*` — message processing
- `Send*`/`Reply*` — output
- `Add*`/`Remove*`/`Delete*` — collection mutations

### Config Keys
`snake_case` throughout: `data_dir`, `allow_from`, `agent_type`.

### Tests
Co-located, stub types (`stubAgent`, `stubPlatform`), no mock frameworks. Naming: `Test<Entity>_<Scenario>`. Table-driven preferred.

## Known Concerns
- `core/engine.go` is 15,578 lines (guideline: ~800)
- `platform/feishu/feishu.go` is 6,099 lines
- A few platform name hardcodes remain in engine.go special-case handling
