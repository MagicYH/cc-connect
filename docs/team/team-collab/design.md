# cc-connect 团队协作技能 — 技术设计

## 目标

为 cc-connect 项目创建一个 `.claude-plugin` 插件包，提供团队协作技能，让 Leader Agent 能够通过飞书群聊协调多个子 Agent 的行为：
1. 统一工作目录（`/dir`）
2. 全队暂停（`/stop`）
3. 全队新上下文（`/new`）

## 背景：关键约束

**cc-connect relay 的限制**：当前 `RelayManager.Send()` → `targetEngine.HandleRelay()` 路径**不会**触发 slash command 处理。relay 消息会直接发送给目标 Agent 的 session，而非经过 `handleMessage` → `handleCommand` 管道。

因此，Leader Agent 通过 relay 发送 `/dir /path` 给子 Agent 时，子 Agent 收到的是**普通文本**，不会执行 cc-connect 的 slash command 逻辑。

**解决方案**：技能需要指导 Leader Agent 将 slash command 作为**直接消息**发送到群聊中（而非通过 relay），让每个 Agent 自己的 `handleMessage` 管道处理这些命令。具体来说，Leader Agent 需要使用 `cc-connect relay send` 向每个子 Agent 发送一条指令，让子 Agent **在飞书中自行发送对应的 slash command**。

或者，更简单的方案：**Leader Agent 直接在群聊中发送 `/dir`、`/stop`、`/new` 命令**——但这样只影响 Leader 自己的 session。

**最终可行方案**：通过 relay 告知每个子 Agent 执行对应的动作。子 Agent 收到 relay 消息后，作为普通 prompt 理解并执行对应操作（如切换目录、停止工作、新建会话）。对于 `/dir`，Agent 可以通过 bash `cd` 来实现；对于 `/stop` 和 `/new`，需要 Agent 在飞书中回复对应的 slash command。

**实际最佳方案**：利用 cc-connect 的 cron/timer 机制或直接在飞书群中由 Leader 向每个 Agent 发消息。但由于 cc-connect 当前 relay 不支持转发 slash command，技能的核心逻辑应为：

1. Leader 通过 relay 向每个子 Agent 发送明确的自然语言指令
2. 子 Agent 理解指令后，通过 `cc-connect` CLI 工具执行对应操作
3. 具体执行方式：子 Agent 调用 `cc-connect` 的 HTTP API 或 Unix socket 命令来触发 slash command

**但最实用的方案**：Leader 在群聊中直接 @ 每个子 Agent，发送带有明确指令的消息，子 Agent 解析后自行执行 `/dir`、`/stop`、`/new`。

## 方案设计

### 技能结构

```
.claude-plugin/
  marketplace.json
plugins/
  cc-connect-team/
    .claude-plugin/
      plugin.json
    skills/
      team-sync-dir/
        SKILL.md
      team-stop/
        SKILL.md
      team-new-session/
        SKILL.md
      team-overview/
        SKILL.md
```

### 技能详细设计

#### 1. team-sync-dir（统一工作目录）

**触发条件**：当用户要求统一团队工作目录、切换团队目录、或提到"大家切到某个目录"时。

**逻辑**：
- Leader 收到用户指定的目录路径
- Leader 通过 `cc-connect relay send` 向每个已绑定的子 Agent 发送指令
- 指令内容：让子 Agent 执行 `/dir {path}` 命令
- 由于 relay 不直接支持 slash command，Leader 向子 Agent 发送的消息格式为：

```
请立即执行 /dir {absolute_path} 切换你的工作目录。这是团队协作指令，执行后确认。
```

- 子 Agent 在自己的 session 中收到此消息后，会理解这是一个需要执行 cc-connect slash command 的请求

**重要发现**：实际上，通过 relay 发送的消息，Agent 会将其当作普通 prompt 处理。Agent 自身无法触发自己所在 cc-connect 的 slash command。因此需要另一种机制。

**真正可行的方案**：使用 `cc-connect` 的 HTTP API / Unix socket 直接发送 slash command：

```bash
# 通过 Unix socket API 发送命令
echo '{"command":"/dir /path/to/project"}' | socat - UNIX-CONNECT:/path/to/cc-connect.sock
```

**最简可行方案**：Leader 在飞书群聊中，向每个子 Agent **分别发送**消息，消息内容就是 slash command 本身。当飞书群中的用户（包括 bot）发送 `/dir /path` 时，cc-connect 的 `handleMessage` 会处理该命令。

但问题是：bot 在群聊中发送 `/dir` 不会被其他 bot 的 cc-connect 实例捕获——只有**用户直接发送**的 `/dir` 才会触发对应 bot 的 `handleMessage`。

**最终方案**：利用 cc-connect 已有的 **timer** 和 **cron** 机制。Leader 可以通过 `cc-connect timer add` 给每个子 Agent 设置即时触发的 timer，timer 的 prompt 内容包含 slash command 的执行指引。

**实际上最简单且可靠的方案**：

技能指导 Leader 在飞书中 **@ 每个子 Agent 并发送指令**，子 Agent 理解后**自行执行 bash 命令**来实现等效效果：

- `/dir /path` → 子 Agent 执行 `cd /path && pwd` 验证
- `/stop` → 无法通过 bash 等效，需要通过 cc-connect API
- `/new` → 无法通过 bash 等效，需要通过 cc-connect API

**结论**：需要在 cc-connect 层面增加支持，或者采用"通过 HTTP API 发送 slash command"的方案。考虑到当前技能不应修改 cc-connect 代码，我采用以下务实方案：

### 务实方案：利用 cc-connect CLI 发送命令

cc-connect 的 timer/cron 机制可以通过 `--exec` 直接执行 shell 命令。而 cc-connect 的 API 端点（Unix socket）可以接收命令。

但更直接的方式是：**Leader 使用 `cc-connect relay send` 向每个子 Agent 发送指令，指令中包含具体的操作步骤，子 Agent 通过自身 cc-connect 的 API 来执行 slash command**。

**最终采用的方案**：

技能中指导 Leader Agent 按以下流程操作：

1. **查询群组中的绑定 bot 列表**：使用 `cc-connect relay list` 或检查 `/bind` 结果
2. **向每个子 Agent 发送 relay 消息**，内容为结构化指令：
   - 对于 `/dir`：发送消息让子 Agent 调用 `cc-connect` CLI 执行目录切换
   - 对于 `/stop`：发送消息让子 Agent 调用 `cc-connect` CLI 停止当前任务
   - 对于 `/new`：发送消息让子 Agent 调用 `cc-connect` CLI 创建新会话

实际上，更简单的做法是：**子 Agent 收到 relay 消息后，通过 cc-connect 的 HTTP API 触发命令**。但子 Agent 是 Claude Code，它本身就在 cc-connect 的 session 中运行，它可以通过 `cc-connect send` 或直接回复 `/dir` 来触发。

**最终最终方案**：经过深入分析，最实用的方式是：

Leader Agent 通过 relay 向子 Agent 发送消息，子 Agent 收到后，**在回复中发送 slash command**。因为子 Agent 的回复是通过 cc-connect 的 `agentSession.Send()` 发送的，这不会触发 `handleCommand`。

**真正的解决方案**：修改 cc-connect 代码，使 `HandleRelay` 支持转发 slash command。但在本技能中，我们不修改 cc-connect 代码，而是采用**工作流约定**的方式：

### 采用方案：基于 cc-connect relay + Agent 自治

技能的核心思路是：**Leader 通过 relay 向子 Agent 发送协作指令，子 Agent 理解并自主执行对应动作**。对于需要 cc-connect slash command 支持的操作（如 `/dir`、`/stop`、`/new`），子 Agent 通过以下方式实现：

1. **`/dir`**：子 Agent 通过 bash `cd` 命令切换工作目录（功能等效）
2. **`/stop`**：子 Agent 立即停止当前工作，回复确认（功能等效——停止生成输出）
3. **`/new`**：子 Agent 清理上下文，开始全新对话（功能等效——忽略之前对话历史）

这是最务实的方案，不需要修改 cc-connect 代码，也不需要子 Agent 有特殊权限。

## 技能文件内容

### marketplace.json

```json
{
  "$schema": "https://anthropic.com/claude-code/marketplace.schema.json",
  "name": "cc-connect-team",
  "description": "Team collaboration skills for cc-connect multi-agent workflows",
  "owner": {
    "name": "Hao Chen",
    "email": "chenhao.magic@bytedance.com"
  },
  "plugins": [
    {
      "name": "cc-connect-team",
      "source": "./plugins/cc-connect-team",
      "description": "Team collaboration skills for cc-connect: sync working directories, stop all agents, reset sessions",
      "version": "0.1.0",
      "author": { "name": "Hao Chen", "email": "chenhao.magic@bytedance.com" },
      "homepage": "https://github.com/chenhg5/cc-connect",
      "repository": "https://github.com/chenhg5/cc-connect.git",
      "license": "MIT",
      "keywords": ["team", "collaboration", "multi-agent", "cc-connect"],
      "category": "workflow",
      "tags": ["team", "collaboration", "multi-agent"],
      "strict": false
    }
  ]
}
```

### plugin.json

```json
{
  "name": "cc-connect-team",
  "version": "0.1.0",
  "description": "Team collaboration skills for cc-connect: sync working directories, stop all agents, reset sessions",
  "author": { "name": "Hao Chen", "url": "https://github.com/chenhg5" },
  "homepage": "https://github.com/chenhg5/cc-connect",
  "repository": "https://github.com/chenhg5/cc-connect.git",
  "license": "MIT",
  "keywords": ["team", "collaboration", "multi-agent", "cc-connect"]
}
```

### SKILL.md 文件内容

见下方实现部分。

## 风险与代价

1. **relay 不支持 slash command**：子 Agent 无法通过 relay 直接触发 `/dir`、`/stop`、`/new`，需要通过 bash 命令模拟等效行为
2. **`/dir` 的 bash `cd` 等效性**：`cd` 只改变 Agent 的 shell 工作目录，不等同于 cc-connect 的 `/dir` 命令（后者还会更新 session 的 workdir 配置，影响后续工具调用的默认路径）
3. **`/new` 的等效性有限**：Agent "忽略之前对话历史" 只是语义约定，实际上上下文仍在 session 中
4. **子 Agent 可能不理解指令**：如果子 Agent 没有安装此技能，可能不理解 Leader 发来的协作指令

## 验收标准

1. `.claude-plugin/marketplace.json` 格式正确，能被 cc-connect 识别
2. `plugins/cc-connect-team/.claude-plugin/plugin.json` 格式正确
3. 每个技能的 SKILL.md 有正确的 frontmatter（name、description）
4. 技能内容清晰、可操作，Leader Agent 能按步骤执行
5. 技能考虑了 relay 的限制，提供了可行的替代方案
6. 在 cc-connect 群聊中，Leader Agent 能通过技能指导完成三个协作操作
