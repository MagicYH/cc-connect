# Bitable Claim Token Lock（认领令牌锁）

多个 agent 会话并发消费同一张 bitable 任务表时的防重复认领机制。bitable 无事务/CAS，用**随机令牌 + 写后回读校验**实现乐观锁。

## 为什么需要

cc-connect 的并发模型是 per-session：每消息一个 goroutine（`core/engine.go:2801`）+ per-session 锁（`:2749 TryLock`），**不同 chat 的会话真并发**。同一 bot 被群@ 与 cron 同时唤醒 = 两个并发子进程，都可能认领同一行。

## 协议

```
1. 读目标行；状态 ≠ 待办 → 放弃
2. 生成随机 nonce
3. 一次 update：状态=进行中, 认领人=角色名, 认领令牌=nonce, 认领时间=now
4. 随机抖动 0.5~2s
5. 重读；认领令牌==nonce → 认领成功；否则败者退让
```

**Fencing（执行期复核）**：刷心跳、置完成、建后继行之前必须重读校验 `认领令牌==自己的 nonce`，不匹配即静默放弃——防止"回收后原 agent 还活着"的双执行。

**心跳回收**：`状态=进行中 AND 心跳时间 早于 M 分钟前`（M=15）→ 回收=写新 nonce+回读校验（同认领锁），不是直接接管。执行中"每子步骤且至少每 10 分钟"刷心跳。

## 实测（2026-07-15）

双群同时 @ 同一 bot 制造真并发（日志证两 session 0.4s 内先后唤起）：唯一认领人、唯一令牌，败者session 跑了但未污染行。心跳回收实测换新令牌成功。

Cross-references: [Group @-Mention Wake](group-at-wake.md), [Cron Self-Check](cron-self-check.md), [Concurrency Safety](../concurrency-safety.md)
