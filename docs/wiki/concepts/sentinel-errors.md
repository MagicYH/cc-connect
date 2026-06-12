# Sentinel Errors

Standard Go sentinel errors created with `errors.New()`, matched with `errors.Is()` for programmatic branching. Key sentinels: `ErrNotSupported` (optional capability not implemented), `ErrAttachmentSendDisabled`, `ErrCronJobNotFound`, `ErrCronProjectNotFound`. Custom error types with `Is()` method exist for Feishu card API and JSON-RPC.

Cross-references: [error-wrapping](error-wrapping.md)
