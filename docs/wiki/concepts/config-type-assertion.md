# Config Type Assertion

The pattern for extracting typed values from `map[string]any` TOML config in factory functions.

## How It Works

Simple extraction: `token, _ := opts["token"].(string)`

Numeric with TOML ambiguity (TOML parses integers as `int` but may return `int64` or `float64`):

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

## Project Relation

All agent and platform factories receive `map[string]any` from config parsing. This pattern is used consistently rather than a reflection-based or schema-based approach.

## Cross-References

- [plugin-registry](./plugin-registry.md) — factories that receive the opts map
