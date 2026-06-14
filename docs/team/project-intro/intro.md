# CC-Connect 项目介绍

## 项目简介

CC-Connect 是一个用 Go 编写的桥接应用，将本地 AI 编程助手与你常用的即时通讯平台连接在一起。无论你在手机、平板还是电脑上，都可以通过飞书、Telegram、Discord、Slack、钉钉、企业微信、微信、QQ、LINE、微博等聊天工具，随时与你的 AI 编程助手对话、下达指令、审查代码变更。

**核心价值：** 让 AI 编程助手突破终端的限制，融入你日常沟通的工具链。你不再需要时刻守在电脑前——任何设备上的聊天窗口都可以成为你的编码控制台。

**项目特点：**

- **Agent 无关**：不绑定任何单一 AI 助手，支持 14 种编程 Agent（含通用终端驱动）
- **平台广泛**：支持 13 种即时通讯平台，多数无需公网 IP
- **双向会话**：不是简单的"发问-回答"，而是持久化的交互式编程会话，支持权限审批、流式输出、多轮对话
- **单二进制**：所有适配器、Web 管理界面、守护进程管理全部编译进一个 Go 二进制，零运行时依赖
- **开源免费**：MIT 协议

---

## 核心架构

CC-Connect 采用严格的分层设计和插件式注册机制，核心原则是 **`core/` 包绝不感知任何具体的 Agent 或平台**。

```
┌─────────────────────────────────────────────────┐
│                cmd/cc-connect                    │  入口、CLI、守护进程
├─────────────────────────────────────────────────┤
│                     config/                      │  TOML 配置解析
├─────────────────────────────────────────────────┤
│                      core/                       │  引擎、接口、i18n、
│                                                  │  卡片、会话、注册表
├──────────────────────┬──────────────────────────┤
│     agent/           │      platform/           │
│  ├── claudecode/     │  ├── feishu/             │
│  ├── codex/          │  ├── telegram/           │
│  ├── cursor/         │  ├── discord/            │
│  ├── gemini/         │  ├── slack/              │
│  ├── iflow/          │  ├── dingtalk/           │
│  ├── opencode/       │  ├── wecom/              │
│  ├── acp/            │  ├── weixin/             │
│  ├── copilot/        │  ├── qq/                 │
│  ├── kimi/           │  ├── qqbot/              │
│  ├── pi/             │  ├── line/               │
│  ├── antigravity/    │  ├── weibo/              │
│  ├── qoder/          │  ├── max/                │
│  ├── devin/          │  └── wps-xiezuo/         │
│  └── tmux/           │                          │
├──────────────────────┴──────────────────────────┤
│                     daemon/                      │  systemd/launchd/schtasks
└─────────────────────────────────────────────────┘
```

**依赖方向（严格单向）：**

- `core/` 仅导入 Go 标准库，绝不导入 `agent/` 或 `platform/`
- `agent/*` 仅导入 `core/`，绝不跨 Agent 或跨平台引用
- `platform/*` 仅导入 `core/`，绝不跨平台或跨 Agent 引用
- `cmd/cc-connect/` 导入所有包，负责组装

**插件注册机制：** 每个 Agent/平台在 `init()` 函数中调用 `core.RegisterAgent()` 或 `core.RegisterPlatform()` 注册工厂函数。主程序通过配置中的字符串名称（如 `"claudecode"`、`"feishu"`）从注册表创建实例。构建标签（`//go:build !no_claudecode`）控制选择性编译。

**消息流转：**

```
平台收到消息 → 平台调用 MessageHandler 回调
  → Engine.handleMessage()（限流、命令分发、权限检查）
    → processInteractiveMessageWith()（发送给 Agent 会话）
      → agentSession.Send()（将 prompt 发给 Agent 进程）
        → processInteractiveEvents()（消费事件流）
          → 根据平台能力接口选择回复方式：
             CardSender / InlineButtonSender / ImageSender / 纯文本回退
```

**能力接口系统：** `core/interfaces.go` 定义了 50+ 个可选接口，Engine 在运行时通过接口断言检测平台/Agent 能力，无需硬编码任何名称。

---

## 支持的 AI 编程助手

CC-Connect 支持 14 种 AI 编程助手，涵盖主流 CLI 工具和通用协议适配器：

| Agent | 包装工具 | 会话模型 | 权限交互 | 特色能力 |
|-------|---------|---------|---------|---------|
| **claudecode** | Anthropic Claude Code CLI | 持久进程 | 结构化 JSON 协议 | 最完整的适配器，支持 ~20 个可选接口、Provider 代理、run_as_user 隔离 |
| **codex** | OpenAI Codex CLI | 逐轮子进程 | CLI 标志 | 双后端（exec / app_server），Wire API 支持 |
| **cursor** | Cursor Agent CLI | 逐轮子进程 | interaction_query | SQLite 会话读取，模型发现 |
| **gemini** | Google Gemini CLI | 逐轮子进程 | CLI 标志 | Delta 流式输出，项目 slug 自动发现 |
| **copilot** | GitHub Copilot CLI | 持久进程 | JSON-RPC | LSP 风格 Content-Length 帧，双向 RPC |
| **acp** | 任意 ACP 协议 Agent | 持久进程 | session/request_permission | 通用协议适配器，模式动态发现 |
| **devin** | Devin CLI | 持久进程（委托 acp） | 继承 ACP | acp 的薄封装，仅覆盖名称和默认参数 |
| **iflow** | iFlow CLI | 逐轮子进程 (PTY) | CLI 标志 | 独特的 PTY 驱动 + Transcript 轮询 |
| **opencode** | OpenCode CLI | 逐轮子进程 | CLI 标志 | 持久化模型缓存，上下文压缩 |
| **kimi** | Kimi Code CLI | 逐轮子进程 | CLI 标志 | 双通道 Session ID 提取（stdout + stderr），思维事件解析 |
| **pi** | Pi Coding Agent CLI | 逐轮子进程 | CLI 标志 | settings/models JSON 解析，上下文用量追踪 |
| **antigravity** | Google Antigravity CLI | 逐轮子进程 | 正则检测 y/n | 共享 Gemini 会话存储布局 |
| **qoder** | Qoder CLI | 逐轮子进程 | CLI 标志 | 轻量适配器，双版本格式兼容 |
| **tmux** | 任意终端应用 | 持久 tmux 窗格 | 不支持 | 通用终端驱动，按键注入 + 屏幕捕获 |

**会话模型说明：**

- **持久进程**：Agent 进程在多轮对话中持续运行，通过 stdin/stdout 或 JSON-RPC 通信（claudecode、copilot、acp、devin）
- **逐轮子进程**：每次 Send() 启动新的 Agent 子进程，通过 `--resume` 参数延续对话（codex、cursor、gemini 等）
- **终端捕获**：通过 tmux 窗格驱动任意终端应用，注入按键并轮询屏幕输出

---

## 支持的消息平台

CC-Connect 支持 13 种即时通讯平台，覆盖国内外主流聊天工具：

| 平台 | 连接方式 | 关键能力 | 特色功能 |
|------|---------|---------|---------|
| **飞书 / Lark** | WebSocket / Webhook | 卡片消息、进度条、文件/图片发送、打字指示器 | 最完整的平台适配，交互式卡片按钮、话题隔离 |
| **Telegram** | 长轮询 | 内联按钮、图片/文件/音频、消息编辑 | 代理支持、论坛主题、权限回调按钮 |
| **Discord** | Gateway WebSocket | 内联按钮、图片/文件、进度条、斜杠命令 | 线程隔离、交互式响应、进度条渲染 |
| **Slack** | Socket Mode | 图片/文件、mrkdwn 格式引导、终端观察 | Assistant Chat 支持、会话范围控制 |
| **钉钉** | Stream SDK | 图片/文件/音频、AI 流式卡片 | AI Card 模板、音频消息接收 |
| **企业微信** | HTTP Webhook / WebSocket | 图片发送、AES 加密 | 回调签名验证、媒体上传下载 |
| **微信 / Weixin（个人）** | ilink 长轮询 | 图片/文件、打字指示器 | CDN 直连媒体解密、上下文令牌管理 |
| **QQ** | OneBot v11 WebSocket | 图片/文件 | CQ 码解析、NapCat/LLOneBot 兼容 |
| **QQ 机器人** | QQ 官方 WebSocket | 内联按钮、图片/文件 | OAuth2 令牌管理、Markdown 支持、键盘按钮 |
| **LINE** | HTTP Webhook | 基础收发 | 最简适配器，PushMessage 模式 |
| **微博** | WebSocket | 图片/文件 | Base64 传输、分块文本协议 |
| **MAX** | 长轮询 / Webhook | 内联按钮、图片/文件/音频、消息编辑 | 俄罗斯主流 IM，西里尔文安全分块，CDN 两步上传 |
| **WPS 协作** | WebSocket | 打字指示器 | KSO-1 HMAC-SHA256 认证，AES-256-CBC 事件解密 |

---

## 关键特性

### 多语言国际化（i18n）

支持 5 种语言：英语、简体中文、繁体中文、日语、西班牙语。自动从用户首条消息检测语言（CJK 字符分析、西班牙语重音检测），所有用户可见字符串均通过 `MsgKey` 常量 + `i18n.T()` 翻译。

### 流式预览

Agent 思考时实时更新消息内容（类似"正在输入"效果），可配置更新间隔（默认 1500ms）、最小增量字符数、最大预览长度。支持 Telegram/Discord/飞书等平台的原地消息编辑。

### 富卡片与进度展示

- 流畅的 CardBuilder API：`NewCard().Title().Markdown().Divider().Buttons().Note().Build()`
- 三种展示模式：`full`（思考/工具分开展示）、`compact`（独立卡片折叠）、`quiet`（单卡片静默）
- 工具步骤追踪：工具调用 + 思考步骤组成进度卡片，状态生命周期：thinking → working → done/error

### 权限处理

Agent 请求执行敏感操作时，用户可通过聊天内联按钮实时审批或拒绝。支持运行时切换权限模式（default / yolo / plan 等），无需重启 Agent 进程。

### 选择性编译

通过构建标签按需编译 Agent 和平台，生成精简二进制：

```bash
# 仅编译 Claude Code + 飞书和 Telegram
make build AGENTS=claudecode PLATFORMS_INCLUDE=feishu,telegram

# 排除不需要的平台
make build EXCLUDE=discord,dingtalk,qq,qqbot,line
```

### 定时任务与一次性计时器

- **Cron 调度器**：完整的 cron 生命周期（增删改查执行），支持 prompt 和 shell 命令两种类型
- **一次性计时器**：延迟触发或定时触发，执行后自动删除
- 均支持会话模式选择：复用已有会话 或 每次新建会话

### 机器人互转（Bot Relay）

同一群聊中的多个 AI 机器人可以互相通信。用户可以让 Claude 回答后转发给 Gemini 继续讨论，所有对话在同一群聊中完成。可配置可见性：完整、摘要、隐藏。

### Webhook 与生命周期钩子

- **Webhook 服务器**：HTTP 端点接收外部触发（Git 钩子、CI/CD、文件监控器）
- **8 种生命周期事件**：message.received、message.sent、session.started、session.ended、cron.triggered、timer.triggered、permission.requested、error
- 支持 shell 命令和 HTTP 请求两种处理器

### 语音输入输出（STT/TTS）

- **语音转文字**：支持 OpenAI Whisper、Groq、通义千问、Gemini 等提供商
- **文字转语音**：支持通义千问、OpenAI、MiniMax、Mimo、eSpeak、Pico、Edge 等提供商
- 语音消息闭环：用户发送语音 → STT → Agent 处理 → TTS → 语音回复

### 用户隔离（run_as_user）

通过 sudo 以不同 Unix 用户身份运行 Agent 进程，实现 OS 级别的安全隔离。支持环境变量跨 sudo 边界传递，并进行安全校验（禁止 LD_PRELOAD、PATH 等危险变量）。

### Web 管理界面

内嵌在二进制中的 Vite + React + Tailwind 管理面板：

```bash
cc-connect web
```

可可视化创建项目、添加平台、管理服务商、监控会话、编辑定时任务、直接与 Agent 聊天，无需手动编辑 TOML。

### 其他运营特性

- **双向限流**：入站滑动窗口 + 出站令牌桶 + 按角色限流
- **消息去重**：TTL 追踪 + 旧消息拒绝
- **守护进程管理**：systemd / launchd / schtasks 跨平台服务安装
- **自动更新**：从 GitHub/Gitee 检测新版本，`cc-connect update` 自更新
- **Provider 代理**：本地反向代理，改写不兼容的 Anthropic API 字段以适配第三方服务商
- **诊断系统**：`cc-connect doctor` 检查 Agent 二进制、认证、平台连通性

---

## 快速上手

### 安装

**方式一：npm（推荐）**

```bash
npm install -g cc-connect
```

**方式二：Homebrew（macOS / Linux）**

```bash
brew install cc-connect
```

**方式三：从 GitHub Releases 下载**

```bash
# Linux amd64
curl -L -o cc-connect https://github.com/chenhg5/cc-connect/releases/latest/download/cc-connect-linux-amd64
chmod +x cc-connect
sudo mv cc-connect /usr/local/bin/
```

**方式四：从源码构建**

```bash
git clone https://github.com/chenhg5/cc-connect.git
cd cc-connect
make build
```

### 安装 AI Agent

至少安装一个 AI 编程助手 CLI：

```bash
# Claude Code
npm install -g @anthropic-ai/claude-code

# Codex
npm install -g @openai/codex

# Gemini CLI
npm install -g @google/gemini-cli
```

### 配置

**推荐：Web UI 配置**

```bash
cc-connect web
```

浏览器打开后可可视化完成所有配置，无需手动编辑文件。

**手动配置：**

```bash
mkdir -p ~/.cc-connect
cp config.example.toml ~/.cc-connect/config.toml
vim ~/.cc-connect/config.toml
```

最小配置示例（Claude Code + Telegram）：

```toml
[[projects]]
name = "my-project"

[projects.agent]
type = "claudecode"

[projects.agent.options]
work_dir = "/home/user/my-project"
mode = "default"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "${TELEGRAM_BOT_TOKEN}"
allow_from = "your_telegram_user_id"
```

### 运行

```bash
# 前台运行
./cc-connect

# 安装为系统服务
./cc-connect daemon install
./cc-connect daemon start

# 查看状态
./cc-connect daemon status
```

---

## 配置说明

CC-Connect 使用 TOML 格式的单一配置文件，所有字符串值支持 `${VAR_NAME}` 环境变量替换，避免硬编码密钥。

### 全局设置

| 配置项 | 键名 | 默认值 | 说明 |
|-------|------|-------|------|
| 语言 | `language` | 自动检测 | "en"、"zh" 或留空 |
| 数据目录 | `data_dir` | `~/.cc-connect` | 会话存储目录 |
| Shell | `shell` | "sh" | /shell、cron、hooks 使用的 shell |
| 空闲超时 | `idle_timeout_mins` | 120 | Agent 无响应最大等待分钟数 |

### 项目配置（`[[projects]]`）

每个项目绑定一个 Agent 和一个或多个平台，这是 CC-Connect 的核心组织单元：

```toml
[[projects]]
name = "project-name"           # 项目标识
mode = ""                       # 留空 或 "multi-workspace"

[projects.agent]
type = "claudecode"             # Agent 类型（也支持 "lark" 作为 "feishu" 的别名）
provider_refs = ["anthropic"]   # 引用全局 providers

[projects.agent.options]
work_dir = "/path/to/code"     # 工作目录
mode = "default"                # 权限模式
model = "claude-sonnet-4-20250514"  # 默认模型

[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
app_id = "${FEISHU_APP_ID}"
app_secret = "${FEISHU_APP_SECRET}"
```

### 全局服务商（`[[providers]]`）

定义一次，多项目共享，避免重复配置 API Key：

```toml
[[providers]]
name = "anthropic"
api_key = "${ANTHROPIC_API_KEY}"
agent_types = ["claudecode"]

[[providers]]
name = "custom-provider"
api_key = "${CUSTOM_API_KEY}"
base_url = "https://api.example.com/v1"
model = "claude-sonnet-4-20250514"
agent_types = ["claudecode", "codex"]
```

### 按角色访问控制

```toml
[projects.users]
default_role = "member"

[projects.users.roles.admin]
user_ids = ["user_id_1", "user_id_2"]

[projects.users.roles.member]
user_ids = ["*"]
disabled_commands = ["/shell", "/dir"]
```

### 关键子系统配置

| 子系统 | 配置节 | 说明 |
|-------|-------|------|
| 流式预览 | `[stream_preview]` | 启用/禁用、更新间隔、最小增量 |
| 限流 | `[rate_limit]` + `[outgoing_rate_limit]` | 入站滑动窗口 + 出站令牌桶 |
| 语音转文字 | `[speech]` | 启用/禁用、提供商、语言 |
| 文字转语音 | `[tts]` | 启用/禁用、提供商、语音、模式 |
| Webhook | `[webhook]` | HTTP 端点、端口、认证 |
| 桥接 | `[bridge]` | WebSocket 端点、端口、认证 |
| 管理接口 | `[management]` | REST API 端口、Token |
| 定时任务 | `[cron]` | 静默模式、会话模式 |
| 钩子 | `[[hooks]]` | 事件类型、处理器（command/http） |
| 自定义命令 | `[[commands]]` | 斜杠命令、prompt 模板或 exec |
| 别名 | `[[aliases]]` | 命令别名映射 |

---

## 开发指南

### 添加新平台

1. 创建 `platform/newplatform/newplatform.go`
2. 实现 `core.Platform` 接口（Name、Start、Reply、Send、Stop）
3. 按需实现可选接口（ImageSender、CardSender 等）
4. 在 `init()` 中注册：`core.RegisterPlatform("newplatform", New)`
5. 创建 `cmd/cc-connect/plugin_platform_newplatform.go`：

```go
//go:build !no_newplatform
package main

import _ "github.com/chenhg5/cc-connect/platform/newplatform"
```

6. 在 `Makefile` 的 `ALL_PLATFORMS` 中添加 `"newplatform"`
7. 在 `config.example.toml` 中添加配置示例
8. 编写单元测试

### 添加新 Agent

1. 创建 `agent/newagent/newagent.go` + `session.go`
2. 实现 `core.Agent` 和 `core.AgentSession` 接口
3. 在 `init()` 中注册：`core.RegisterAgent("newagent", New)`
4. 创建 `cmd/cc-connect/plugin_agent_newagent.go`：

```go
//go:build !no_newagent
package main

import _ "github.com/chenhg5/cc-connect/agent/newagent"
```

5. 在 `Makefile` 的 `ALL_AGENTS` 中添加 `"newagent"`
6. 可选实现 `AgentDoctorInfo` 以支持 `cc-connect doctor`
7. 在 `config.example.toml` 中添加配置示例
8. 编写单元测试

### 核心开发规则

1. **core/ 中绝不硬编码平台或 Agent 名称**：使用接口和能力检查，而非 `if p.Name() == "feishu"`
2. **优先使用接口而非类型断言**：当行为因平台/Agent 不同时，在 core 中定义可选接口，由实现方按需实现
3. **配置优于代码**：可变功能应可在 `config.toml` 中配置，新字段应有合理默认值
4. **高内聚低耦合**：每个 `agent/X/` 和 `platform/X/` 包自包含，不跨包引用
5. **错误处理**：始终用 `fmt.Errorf("platform: operation: %w", err)` 包装错误，绝不静默吞掉
6. **并发安全**：用 `sync.Mutex` 或 `atomic` 保护共享状态，用 `context.Context` 传播取消
7. **i18n**：所有用户可见字符串必须通过 `core/i18n.go`，定义 `MsgKey` 常量并提供 5 种语言翻译

### 运行测试

```bash
go test ./...              # 全量测试
go test ./core/ -v         # 单包测试
go test -race ./...        # 竞态检测
make test-fast             # 快速测试（<2min）
make test-full             # 完整测试（<10min）
```

### 构建与发布

```bash
make build                                 # 默认构建（含 Web UI）
make build AGENTS=claudecode,codex         # 仅指定 Agent
make build PLATFORMS_INCLUDE=feishu,telegram  # 仅指定平台
make release TARGET=linux/amd64            # 单平台发布
make release-all                           # 全平台发布
```

---

## 致谢

CC-Connect 是一个开源社区驱动的项目，感谢所有贡献者的付出。

## 许可证

MIT License
