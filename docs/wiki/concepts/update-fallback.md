# Update Fallback

The pattern of using Gitee as an automatic fallback when GitHub is unreachable, primarily for China users.

## Project Relation

Applied to three service pairs: GitHub/Gitee Releases API (auto-update), GitHub/Gitee Raw for provider presets, and GitHub/Gitee Raw for skill presets. Implemented in `core/updater.go`, `core/provider_presets.go`, and `core/skill_presets.go`. Fallback is automatic; no user configuration needed.

## Cross-References

- [GitHub Releases](../entities/github-releases.md)
- [Gitee Releases](../entities/gitee-releases.md)
- [Provider Preset](../concepts/provider-preset.md)
- [External Dependencies Source](../sources/external-dependencies.md)
