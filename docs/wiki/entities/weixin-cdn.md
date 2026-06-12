# WeChat Personal / Weixin CDN

WeChat personal account messaging via `ilinkai.weixin.qq.com` with CDN at `novac2c.cdn.weixin.qq.com`.

## Project Relation

Platform adapter in `platform/weixin/` with custom long-poll client and CDN download/upload. Uses AES-128-ECB decryption for CDN media. CDN base URL is configurable.

Config: `type="weixin"`.

## Cross-References

- [Platform SDK](../concepts/platform-sdk.md)
- [External Dependencies Source](../sources/external-dependencies.md)
