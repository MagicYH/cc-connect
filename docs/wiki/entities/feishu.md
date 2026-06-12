# Feishu / Lark

Feishu (domestic) / Lark (international) is a messaging and collaboration platform by ByteDance. In cc-connect, it is one of the supported messaging platforms (`platform/feishu/`).

The Feishu adapter uses the `larksuite/oapi-sdk-go/v3` (v3.5.3) SDK for bot API interaction, including card messages and event subscription.

Cross-references: [tech-stack](../sources/tech-stack.md), [messaging-platform](../concepts/messaging-platform.md), [project-structure](../sources/project-structure.md)

From [project-structure](../sources/project-structure.md): The largest platform adapter at 6,714 lines. Registers as both `"feishu"` and `"lark"`. Key files: `feishu.go` (6,099 lines — largest single file), `card.go` (464 lines), `ws_shared.go` (84 lines).
