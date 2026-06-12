# TTS Pipeline

Text-to-speech pipeline that converts agent text responses into audio for voice message playback on messaging platforms.

## Project Relation

Implemented in `core/tts.go`. Agent text is sent to a TTS provider; the resulting WAV audio is converted to Opus via `ffmpeg`. Supported providers: OpenAI, DashScope Qwen TTS, MiniMax, Xiaomi MiMo.

Config: `[tts] provider="<name>"`.

## Cross-References

- [STT Pipeline](../concepts/stt-pipeline.md)
- [ffmpeg](../entities/ffmpeg.md)
- [External Dependencies Source](../sources/external-dependencies.md)
