# Anthropic API

Anthropic's API at `api.anthropic.com` for Claude models.

## Project Relation

Used for `cc-connect doctor` health checks (`core/doctor.go`). Indirectly consumed by the Claude Code CLI subprocess, which makes its own outbound calls. Also accessible via AWS Bedrock (env-based routing).

Config: `[projects.agent.providers] name="anthropic"`.

## Cross-References

- [Agent CLI](../concepts/agent-cli.md)
- [External Dependencies Source](../sources/external-dependencies.md)
