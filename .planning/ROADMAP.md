# Milestone v1.1 Roadmap

**Milestone:** 飞书 Webhook 支持
**Goal:** 添加 webhook 接入方式，让无法使用 WebSocket 的环境也能运行 bridge
**Created:** 2026-01-29
**Reviewed:** 2026-01-29 (codex 技术评审通过)

## Codex 评审决策

| 决策项 | 结论 |
|--------|------|
| 接口设计 | 方案 B：`FeishuSender` + `FeishuReceiver` 分离 |
| 签名验证 | 默认强制，缺少 `verification_token` 或 `encrypt_key` 拒绝启动 |
| 并发控制 | Worker pool + 有界队列（非简单 semaphore） |
| HTTP 安全 | 超时配置 + body 大小限制 + 仅 POST |
| Phase 合并 | 原 Phase 2/3 合并，安全默认开启 |

## Phase Overview

| Phase | 名称 | 工作量 | 交付物 | 状态 |
|-------|------|--------|--------|------|
| 1 | 接口抽象和 REST 提取 | 1 天 | FeishuSender/Receiver 接口分离 | **Complete** ✓ |
| 2 | Webhook Server (含安全) | 1.5 天 | HTTP 服务器 + 验签 + 解密 | **Complete** ✓ |
| 3 | 配置扩展和模式切换 | 0.5 天 | 配置支持 + 启动逻辑 | Pending |
| 4 | 端到端测试和文档 | 1 天 | 测试覆盖 + 用户文档 | Pending |

**总工作量:** 4 开发日

---

## Phase 1: 接口抽象和 REST 提取

**优先级:** P0 (基础重构)
**状态:** Complete ✓
**Plans:** 3 plans
**Completed:** 2026-01-29

### Goal
采用方案 B 重构：将 Feishu 客户端拆分为 `FeishuSender`（发送）和 `FeishuReceiver`（接收）两个接口，Bridge 只依赖 Sender。

### Deliverables
1. 创建 `internal/feishu/sender.go`：
   - `FeishuSender` 接口：`SendMessage`, `UpdateMessage`, `DeleteMessage`
   - `RESTSender` 实现：封装 lark.Client 的 REST 调用
2. 创建 `internal/feishu/receiver.go`：
   - `FeishuReceiver` 接口：`Start(ctx context.Context) error`
   - `MessageHandler` 类型：`func(ctx context.Context, msg *Message)`
3. 重构 `client.go`：
   - 重命名为 `ws_receiver.go`
   - 实现 `FeishuReceiver` 接口
   - 内嵌 `RESTSender`
4. 更新 `bridge.go`：
   - `feishuClient` 改为 `FeishuSender` 接口类型
   - 删除 `SetFeishuClient` 后置注入模式

### Plans
- [x] 01-01-PLAN.md — 创建 FeishuSender/RESTSender 和 FeishuReceiver 接口
- [x] 01-02-PLAN.md — 重构 client.go 为 ws_receiver.go，内嵌 RESTSender
- [x] 01-03-PLAN.md — 更新 Bridge 使用接口，删除后置注入

### Verification
- [x] WebSocket 模式功能不受影响
- [x] `go build` 成功
- [x] Bridge 只依赖 FeishuSender 接口

### Pitfall Avoidance
- 接口方法数不超过 5 个，避免过度抽象

---

## Phase 2: Webhook Server (含安全)

**优先级:** P0 (核心功能 + 安全)
**状态:** Complete ✓
**Plans:** 3 plans in 3 waves
**Completed:** 2026-01-29
**Covers:** REQ-01, REQ-02, REQ-03, REQ-07, REQ-08

### Goal
实现 HTTP 服务器接收飞书 webhook 回调，**第一版就包含签名验证和消息解密**，安全默认开启。

### Deliverables

#### 2.1 HTTP Server 基础
1. 创建 `internal/feishu/webhook_receiver.go`
2. 实现 `FeishuReceiver` 接口
3. HTTP Server 安全配置：
   - `ReadTimeout`: 10s
   - `WriteTimeout`: 10s
   - `IdleTimeout`: 60s
   - Body 限制：1MB (`http.MaxBytesReader`)
   - 仅允许 POST 方法

#### 2.2 事件处理
1. Challenge 验证逻辑 (`type: "url_verification"`)
2. 签名验证（使用 SDK `VerifySign`）
3. 消息解密（使用 SDK 内置 AES-CBC）
4. 事件解析和 `Message` 转换
5. 返回策略：
   - 验签失败：返回 401
   - 非消息事件：返回 200（避免重试风暴）
   - 消息事件：立即返回 200，异步处理

#### 2.3 并发控制
1. 固定 worker pool（默认 10 个 worker）
2. 有界队列（默认 100 容量）
3. 队列满时返回 503 触发飞书重试
4. 可配置：`webhook.workers`, `webhook.queue_size`

#### 2.4 优雅关闭
1. `http.Server.Shutdown(ctx)` 等待处理中请求
2. 关闭 worker pool，等待队列清空

#### 2.5 可观测性
1. `/health` 端点返回服务状态
2. `/metrics` 端点返回 Prometheus 格式指标

### Plans
- [x] 02-01-PLAN.md — Worker Pool 实现 (Wave 1)
- [x] 02-02-PLAN.md — WebhookReceiver 核心实现 (Wave 2)
- [x] 02-03-PLAN.md — Health/Metrics 端点 (Wave 3)

### Wave Structure
```
Wave 1: 02-01 (Worker Pool - 独立模块)
Wave 2: 02-02 (WebhookReceiver - 依赖 02-01)
         02-03 (Health/Metrics - 依赖 02-01，可与 02-02 并行)
```

### Verification
- [x] Challenge 请求返回正确响应
- [x] 正确签名的请求通过，错误签名返回 401
- [x] 加密消息正确解密
- [x] 响应时间 < 100ms
- [x] 队列满时返回 503
- [x] /health 和 /metrics 端点可用

### Pitfall Avoidance
- 陷阱 1 (响应超时): 立即返回 200，异步处理
- 陷阱 3 (Challenge 验证): 第一个测试案例
- 陷阱 4 (签名验证缺失): 使用 SDK，不手写

---

## Phase 3: 配置扩展和模式切换

**优先级:** P0 (用户体验)
**Covers:** REQ-04, REQ-05, REQ-06

### Goal
扩展配置结构支持 Webhook 模式，**Webhook 模式强制要求 verification_token 和 encrypt_key**。

### Deliverables
1. 扩展 `config.Config`：
   ```go
   type Config struct {
       Mode    string        `json:"mode"`    // "websocket" | "webhook"
       Feishu  FeishuConfig  `json:"feishu"`
       Webhook WebhookConfig `json:"webhook"`
   }

   type WebhookConfig struct {
       Port              int    `json:"port"`               // 默认 8080
       Path              string `json:"path"`               // 默认 "/webhook/feishu"
       VerificationToken string `json:"verification_token"` // 强制
       EncryptKey        string `json:"encrypt_key"`        // 强制
       Workers           int    `json:"workers"`            // 默认 10
       QueueSize         int    `json:"queue_size"`         // 默认 100
   }
   ```
2. 配置验证：
   - Webhook 模式缺少 `verification_token` 或 `encrypt_key`：报错退出
   - 错误信息指向配置路径：`~/.clawdbot/bridge.json: webhook.verification_token`
3. 在 `main.go` 实现 `switch cfg.Mode` 逻辑
4. 默认值处理：mode 默认 "websocket"

### Verification
- [ ] 配置 `mode: "webhook"` 启动 Webhook 模式
- [ ] 缺少必填字段时报错信息清晰
- [ ] 缺少 mode 配置默认使用 websocket

### Pitfall Avoidance
- 陷阱 5 (双模式运行): 配置互斥检查

---

## Phase 4: 端到端测试和文档

**优先级:** P1 (质量保障)
**Covers:** REQ-09, REQ-10

### Goal
完成测试覆盖和用户文档，确保功能可用且用户能自助配置。

### Deliverables
1. 单元测试：
   - Challenge 请求测试
   - 签名验证测试（正确/错误签名）
   - 重复事件测试（相同 message_id）
   - 队列满测试（返回 503）
2. 健康检查端点 `/health`
3. 可观测性：
   - 记录 `event_id`, `message_id`
   - 验签失败计数
   - 处理耗时日志
4. 使用 ngrok 进行真实飞书环境测试
5. 更新 README：
   - Webhook 配置说明
   - 飞书后台配置步骤截图
   - 常见问题排查

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
Phase 2 (Webhook Server + 安全) ← 依赖 Phase 1 的接口
    ↓
Phase 3 (配置模式) ← 依赖 Phase 1-2 的所有组件
    ↓
Phase 4 (测试文档) ← 可与 Phase 3 部分并行
```

---

## Success Criteria

Milestone v1.1 完成标准:

1. **功能完整性:**
   - [ ] Webhook 模式可接收飞书消息
   - [ ] 消息正确转发到 ClawdBot 并返回响应
   - [ ] 配置文件可切换 WebSocket/Webhook 模式

2. **安全性:**
   - [ ] 签名验证默认强制开启
   - [ ] 支持加密消息解密
   - [ ] 缺少安全配置拒绝启动

3. **稳定性:**
   - [ ] 响应时间 < 100ms (远低于飞书 3 秒要求)
   - [ ] 重复事件正确去重
   - [ ] Worker pool 防止资源耗尽
   - [ ] 优雅关闭无数据丢失

4. **可用性:**
   - [ ] README 包含 Webhook 配置说明
   - [ ] 健康检查端点可用
   - [ ] 错误信息清晰指向配置问题

---
*Roadmap created: 2026-01-29*
*Codex reviewed: 2026-01-29*
*Phase 1 planned: 2026-01-29*
*Phase 1 complete: 2026-01-29*
*Phase 2 planned: 2026-01-29*
*Phase 2 complete: 2026-01-29*
