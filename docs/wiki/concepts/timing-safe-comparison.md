# Timing-Safe Comparison

All token/auth comparisons use `crypto/subtle.ConstantTimeCompare` to prevent timing attacks. Applied in management API bearer auth, bridge WebSocket/HTTP token auth, webhook token auth, and MAX platform webhook secret. WeCom uses SHA1 signature sort-join-hash; WPS Xiezuo uses `hmac.Equal`.

Cross-references: [token-redaction](token-redaction.md)
