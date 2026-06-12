# Media Processing (STT/TTS)

**STT** transcribes voice messages via OpenAI (whisper-1), Groq, or Qwen APIs. Requires `ffmpeg` for AMR/OGG-to-MP3 conversion. **TTS** synthesizes replies via Qwen, OpenAI, MiniMax, MiMo, or local engines (espeak, pico, edge). Requires `ffmpeg` for WAV-to-Opus (Feishu format).

**Implementation:** `core/speech.go`, `core/tts.go`
**Config:** `[speech]`, `[tts]` in `config.toml`
