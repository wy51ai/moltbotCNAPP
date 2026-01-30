# Phase 3: 配置扩展和模式切换 - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

扩展配置结构支持 Webhook 模式，实现 WebSocket/Webhook 模式切换。用户可通过配置文件选择运行模式，Webhook 模式强制要求安全配置。

</domain>

<decisions>
## Implementation Decisions

### 配置格式
- mode 字段放在顶层：`config.mode: "websocket" | "webhook"`
- 配置文件沿用现有：`~/.clawdbot/bridge.json`
- 一次只运行一个模式，不支持双模式同时运行

### 错误信息
- 带修复建议的错误信息风格（错误 + 如何修复的提示）
- Webhook 模式缺少 `verification_token` 或 `encrypt_key` 时拒绝启动
- 错误输出到统一日志系统（slog/zerolog）

### 默认值策略
- 默认 mode: `websocket`（维持现有行为）
- 默认端口: `9090`
- 默认 workers: `10`, queue_size: `100`

### 模式切换逻辑
- 启动时打印带配置摘要的信息（如 `Starting webhook mode on :9090/webhook/feishu (10 workers)`）
- 一次只运行一个模式，配置互斥

### Claude's Discretion
- Webhook 配置位置（顶层 `webhook` 字段 vs 嵌套在 `feishu` 下）
- 是否支持环境变量覆盖敏感配置
- 默认 webhook path
- 多个配置错误时的处理方式（第一个就停 vs 收集所有）
- 切换逻辑组织方式（switch in main vs 工厂函数）
- 无效 mode 值的处理方式

</decisions>

<specifics>
## Specific Ideas

- 启动日志应包含关键配置摘要，方便排查问题
- 错误信息要指向配置路径，如 `~/.clawdbot/bridge.json: webhook.verification_token`

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在 phase 范围内

</deferred>

---

*Phase: 03-config-mode*
*Context gathered: 2026-01-29*
