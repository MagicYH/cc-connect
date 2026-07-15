# Workspace Binding（群↔工作目录绑定）

cc-connect 多工作区模式下，**每个群必须先绑定工作目录，bot 才会真正跑 agent**——未绑定时被 @ 只回 "No workspace found for this channel"，不干活。动态建群的多 bot 系统必须把这一步纳入初始化流程。

## 机制

- 绑定持久化：`dataDir/workspace_bindings.json`，结构 `project:<name>` → `feishu:<chatID>` → `{channel_name, workspace, bound_at}`。
- 群内命令绑定：`@bot /workspace init <git-url 或本地路径>`；**本地路径默认被拒**，需各 bot 的 `[[projects]]` 配置加 `workspace_init_allow_local_paths = true`（实测默认 disabled，报 "Local directory targets are disabled"）。本地路径绑定免 clone、免重启（`core/engine.go:15876`）。
- 直接改 JSON + 重启 daemon 也可（适合批量预置）。

## 动态建群初始化清单（实测总结）

1. 建群拉齐 bot（`+chat-create --bots <app_id,...>`）；
2. **逐个 @bot 发 `/workspace init <目录>`**（每个 bot 独立绑定，一个都不能少）；
3. 确认回执后才派活。

另两条相关配置硬前提：角色 bot 的 `allow_chat`/`allow_from` 必须留空（静态 allowlist 会拒收运行时新建群的消息，`platform/feishu/feishu.go:1217`）；`resolve_mentions=false` 防误唤醒。

Cross-references: [Cron Self-Check](cron-self-check.md), [Allow From](../allow-from.md), [Group @-Mention Wake](group-at-wake.md)
