# OpenAI API

OpenAI's API at `api.openai.com` for GPT models and Whisper STT.

## Project Relation

Used as STT (Whisper) provider (`core/speech.go`) and TTS provider (`core/tts.go`). Indirectly consumed by Codex and Copilot CLI subprocesses.

Config: `[projects.agent.providers] name="openai"`, `[speech] provider="openai"`, `[tts] provider="openai"`.

## Cross-References

- [STT Pipeline](../concepts/stt-pipeline.md)
- [TTS Pipeline](../concepts/tts-pipeline.md)
- [External Dependencies Source](../sources/external-dependencies.md)
