# Milestone v1.1 Roadmap

**Milestone:** 飞书 Webhook 支持
**Goal:** 添加 webhook 接入方式，让无法使用 WebSocket 的环境也能运行 bridge
**Created:** 2026-01-29

## Phase Overview

| Phase | 名称 | 工作量 | 交付物 | 状态 |
|-------|------|--------|--------|------|
| 1 | 接口抽象和 REST 提取 | 1 天 | 重构现有代码，为 Webhook 铺路 | Pending |
| 2 | Webhook Server 核心实现 | 1 天 | HTTP 服务器 + 事件处理 | Pending |
| 3 | 签名验证和安全增强 | 0.5 天 | 签名验证 + 消息解密 | Pending |
| 4 | 配置扩展和模式切换 | 0.5 天 | 配置支持 + 启动逻辑 | Pending |
| 5 | 端到端测试和文档 | 1 天 | 测试覆盖 + 用户文档 | Pending |

**总工作量:** 3-4 开发日

---

## Phase 1: 接口抽象和 REST 提取

**优先级:** P0 (基础重构)
**Covers:** 为后续 Phase 铺路

### Goal
重构现有 Feishu 客户端代码，提取接口抽象和共享的 REST 客户端，使 Webhook 和 WebSocket 模式能共用消息发送逻辑。

### Deliverables
1. 创建 `internal/feishu/interface.go` 定义 `FeishuClient` 和 `MessageReceiver` 接口
2. 创建 `internal/feishu/rest.go` 封装消息发送逻辑 (`SendMessage`, `UpdateMessage`, `DeleteMessage`)
3. 重构 `client.go` 内嵌 `RESTClient`
4. 更新 `bridge.go` 使用接口类型

### Verification
- [ ] WebSocket 模式功能不受影响
- [ ] `go build` 成功
- [ ] 现有测试通过

### Pitfall Avoidance
- 接口方法数不超过 5 个，避免过度抽象

---

## Phase 2: Webhook Server 核心实现

**优先级:** P0 (核心功能)
**Covers:** REQ-01, REQ-02, REQ-03

### Goal
实现 HTTP 服务器接收飞书 webhook 回调，处理 Challenge 验证和消息事件。

### Deliverables
1. 创建 `internal/feishu/webhook.go`
2. 实现 Challenge 验证逻辑 (`type: "url_verification"`)
3. 实现事件解析和 `Message` 转换
4. 实现立即返回 200 + 异步处理模式
5. 实现优雅关闭 (`http.Server.Shutdown`)

### Verification
- [ ] Challenge 请求返回正确响应
- [ ] 普通消息事件正确解析
- [ ] 响应时间 < 100ms

### Pitfall Avoidance
- 陷阱 1 (响应超时): 立即返回 200，异步处理
- 陷阱 3 (Challenge 验证): 第一个测试案例

---

## Phase 3: 签名验证和安全增强

**优先级:** P0 (生产必需)
**Covers:** REQ-07, REQ-08

### Goal
使用 SDK 内置功能实现签名验证和消息解密，确保生产环境安全。

### Deliverables
1. 集成 SDK 的签名验证方法
2. 实现 `encrypt_key` 配置和解密逻辑
3. 添加签名验证失败日志
4. 添加时间戳检查 (防重放攻击)

### Verification
- [ ] 正确签名的请求通过
- [ ] 错误签名的请求返回 401
- [ ] 加密消息正确解密

### Pitfall Avoidance
- 陷阱 4 (签名验证缺失): 使用 SDK，不手写

---

## Phase 4: 配置扩展和模式切换

**优先级:** P0 (用户体验)
**Covers:** REQ-04, REQ-05, REQ-06

### Goal
扩展配置结构支持 Webhook 模式，实现启动时根据配置选择模式。

### Deliverables
1. 扩展 `config.Config` 添加 `Mode`, `WebhookConfig` 字段
2. 在 `main.go` 实现 `switch cfg.Mode` 逻辑
3. 添加配置互斥检查 (禁止同时启用两种模式)
4. 添加默认值处理 (mode 默认 "websocket")

### Verification
- [ ] 配置 `mode: "webhook"` 启动 Webhook 模式
- [ ] 配置 `mode: "websocket"` 启动 WebSocket 模式
- [ ] 缺少 mode 配置默认使用 websocket

### Pitfall Avoidance
- 陷阱 5 (双模式运行): 配置互斥检查

---

## Phase 5: 端到端测试和文档

**优先级:** P1 (质量保障)
**Covers:** REQ-09, REQ-10

### Goal
完成测试覆盖和用户文档，确保功能可用且用户能自助配置。

### Deliverables
1. Challenge 请求测试 (单元测试)
2. 重复事件测试 (相同 message_id)
3. 签名验证测试 (正确/错误签名)
4. 健康检查端点 `/health`
5. 使用 ngrok 进行真实飞书环境测试
6. 更新 README 添加 Webhook 配置说明

### Verification
- [ ] 所有测试通过
- [ ] 真实飞书应用可正常收发消息
- [ ] 文档可供用户自助配置

### Pitfall Avoidance
- 陷阱 2 (重复处理): 测试重试场景

---

## Dependencies

```
Phase 1 (接口抽象)
    ↓
Phase 2 (Webhook Server) ← 依赖 Phase 1 的接口
    ↓
Phase 3 (签名验证) ← 依赖 Phase 2 的 HTTP handler
    ↓
Phase 4 (配置模式) ← 依赖 Phase 1-3 的所有组件
    ↓
Phase 5 (测试文档) ← 可与 Phase 4 部分并行
```

---

## Success Criteria

Milestone v1.1 完成标准:

1. **功能完整性:**
   - [ ] Webhook 模式可接收飞书消息
   - [ ] 消息正确转发到 ClawdBot 并返回响应
   - [ ] 配置文件可切换 WebSocket/Webhook 模式

2. **安全性:**
   - [ ] 签名验证正常工作
   - [ ] 支持加密消息解密

3. **稳定性:**
   - [ ] 响应时间 < 3 秒 (飞书要求)
   - [ ] 重复事件正确去重
   - [ ] 优雅关闭无数据丢失

4. **可用性:**
   - [ ] README 包含 Webhook 配置说明
   - [ ] 健康检查端点可用

---
*Roadmap created: 2026-01-29*
