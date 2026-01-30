# ClawdBot Bridge

## What This Is

ClawdBot Bridge 是一个消息桥接服务，连接飞书企业消息平台与本地 ClawdBot AI Gateway。用户在飞书发送消息，bridge 转发到 ClawdBot 获取 AI 响应，再回复到飞书。

## Core Value

**用户在飞书能与 ClawdBot AI 顺畅对话** — 消息必须可靠送达、响应必须正确返回。

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

- ✓ **CONN-01**: WebSocket 模式连接飞书 — v0.1
- ✓ **MSG-01**: 接收用户私聊/群聊消息 — v0.1
- ✓ **MSG-02**: 转发消息到 ClawdBot Gateway — v0.1
- ✓ **MSG-03**: 将 AI 响应发送回飞书 — v0.1
- ✓ **UX-01**: 长响应时显示"思考中..."占位符 — v0.1
- ✓ **DAEMON-01**: 后台守护进程运行 — v0.1

### Active

<!-- Current scope. Building toward these. -->

- [ ] **WEBHOOK-01**: Webhook 模式接收飞书消息
- [ ] **WEBHOOK-02**: HTTP 服务器监听 webhook 回调
- [ ] **WEBHOOK-03**: 飞书 URL 验证 (challenge 校验)
- [ ] **CONFIG-01**: 配置文件切换 WebSocket/Webhook 模式

### Out of Scope

<!-- Explicit boundaries. Includes reasoning to prevent re-adding. -->

- 云函数部署适配 — 当前只支持本地 HTTP 服务器，用户自行配置网络穿透
- 非消息类事件处理 — webhook 模式只处理消息事件，与 WebSocket 模式对齐
- 多模式同时运行 — 一个实例只运行一种模式，简化架构

## Context

**技术背景：**
- 飞书提供两种事件订阅方式：WebSocket 长连接和 HTTP Webhook 回调
- 当前 bridge 只支持 WebSocket 模式
- 部分部署环境无法建立出站 WebSocket 连接，需要 webhook 作为替代方案

**已有代码：**
- `internal/feishu/client.go` — WebSocket 模式实现
- `internal/bridge/bridge.go` — 消息处理核心逻辑（可复用）
- `internal/config/config.go` — 配置加载

**参考资料：**
- 飞书官方文档（需要在研究阶段详细查阅）

## Constraints

- **Tech stack**: Go 1.21+，复用现有依赖（larksuite SDK、标准库 net/http）
- **兼容性**: 与现有 WebSocket 模式 API 兼容，用户只需改配置即可切换
- **审批流程**: 技术方案需与 codex (AI 助手) 讨论评审，达成共识后才执行

## Key Decisions

<!-- Decisions that constrain future work. Add throughout project lifecycle. -->

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| 配置文件切换模式而非命令行参数 | 与现有配置模式一致，部署脚本更简单 | — Pending |
| 只支持本地 HTTP 服务器 | 降低复杂度，serverless 适配留给未来 | — Pending |
| 消息处理逻辑复用 | webhook 和 websocket 都走同一个 Bridge.HandleMessage | — Pending |

---

## Current Milestone: v1.1 飞书 Webhook 支持

**Goal:** 添加 webhook 接入方式，让无法使用 WebSocket 的环境也能运行 bridge

**Target features:**
- HTTP 服务器监听飞书 webhook
- URL 验证 (challenge) 处理
- 消息事件解析和处理
- 配置文件模式切换

---
*Last updated: 2026-01-29 after milestone v1.1 started*
