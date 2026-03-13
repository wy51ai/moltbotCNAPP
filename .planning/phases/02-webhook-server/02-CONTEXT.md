# Phase 2: Webhook Server (含安全) - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

实现 HTTP 服务器接收飞书 webhook 回调，包含签名验证和消息解密，安全默认开启。涵盖：
- HTTP Server 基础配置（超时、body 限制、仅 POST）
- Challenge 验证 + 签名验证 + 消息解密
- Worker pool 并发控制 + 有界队列
- 优雅关闭

不包含：配置结构扩展（Phase 3）、测试和文档（Phase 4）

</domain>

<decisions>
## Implementation Decisions

### 日志与可观测性
- 日志格式：Claude 根据现有代码风格决定
- Request ID：使用飞书的 event_id，不额外生成 UUID
- 安全日志：验签失败时记录请求来源 IP
- Metrics：暴露 `/metrics` 端点（Prometheus 格式），包含请求数、延迟、队列深度

### 错误响应设计
- 401 响应体：Claude 根据安全最佳实践决定
- 503 响应体：Claude 根据 HTTP 最佳实践决定
- 非消息事件处理：Claude 根据实际情况决定
- 解密失败状态码：Claude 根据 HTTP 语义决定

### 队列行为
- Panic 处理：Claude 根据 Go 最佳实践决定
- 队列告警：Claude 根据实际情况决定
- 优雅关闭：Claude 根据生产环境最佳实践决定
- 重复事件：使用内存去重，记录最近 N 个 message_id

### 启动与健康检查
- Health 内容：Claude 根据 Kubernetes 最佳实践决定
- 探针类型：单一 `/health` 端点，不区分 liveness/readiness
- 启动信息：显示配置摘要（端口、路径、worker 数等关键配置）
- 启动失败：显示明确原因（如 "port 8080 already in use"）

### Claude's Discretion
- 日志格式（JSON vs 纯文本）
- 401/503 响应体内容
- 非消息事件处理方式
- 解密失败状态码
- Worker panic 处理
- 队列告警机制
- 优雅关闭等待策略
- /health 端点具体内容

</decisions>

<specifics>
## Specific Ideas

- 使用飞书 SDK 的 VerifySign 和内置 AES-CBC 解密，不手写
- 内存去重用于处理飞书重试
- 启动时打印配置摘要方便运维排查

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在 Phase 2 范围内

</deferred>

---

*Phase: 02-webhook-server*
*Context gathered: 2026-01-29*
