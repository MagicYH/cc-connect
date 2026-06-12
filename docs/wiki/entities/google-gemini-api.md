# Google Gemini API

Google's Generative AI API at `generativelanguage.googleapis.com`.

## Project Relation

Used as STT provider (`core/speech.go`). Indirectly consumed by the Gemini CLI agent subprocess. Also accessible via Vertex AI (env-based routing).

Config: `[projects.agent.providers] name="google"`, `[speech] provider="google"`.

## Cross-References

- [STT Pipeline](../concepts/stt-pipeline.md)
- [External Dependencies Source](../sources/external-dependencies.md)
