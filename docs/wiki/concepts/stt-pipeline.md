# STT Pipeline

Speech-to-text pipeline that converts voice messages from messaging platforms into text for agent input.

## Project Relation

Implemented in `core/speech.go`. Audio from platforms arrives in AMR/OGG format; `ffmpeg` converts to MP3 before sending to a provider. Supported providers: OpenAI Whisper, Google Gemini, DashScope Qwen ASR, Groq Whisper.

Config: `[speech] provider="<name>"`.

## Cross-References

- [TTS Pipeline](../concepts/tts-pipeline.md)
- [ffmpeg](../entities/ffmpeg.md)
- [OpenAI API](../entities/openai-api.md)
- [External Dependencies Source](../sources/external-dependencies.md)
