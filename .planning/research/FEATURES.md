# Feature Landscape: 飞书 Webhook 支持

**Domain:** 飞书事件订阅 (Webhook 模式)
**Researched:** 2026-01-29
**Confidence:** MEDIUM (基于现有代码分析和训练知识,缺少官方文档验证)

## Table Stakes

用户期望 webhook 模式必须具备的功能。缺少这些功能 webhook 无法正常工作。

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **URL 验证 (Challenge)** | 飞书要求在配置 webhook URL 时进行验证,这是标准流程 | Low | 接收 POST 请求,返回 `challenge` 字段即可 |
| **接收 HTTP POST 事件** | Webhook 本质是 HTTP 回调,必须监听 POST 请求 | Low | 标准 HTTP 服务器,端口可配置 |
| **解析消息事件 payload** | 需要从 HTTP body 解析出消息内容、发送人、会话 ID | Low | JSON unmarshal,结构体字段与 WebSocket 模式一致 |
| **事件类型过滤** | 飞书会推送多种事件,只处理消息事件 (与 WebSocket 模式对齐) | Low | 检查 `event.type` 字段 |
| **返回 HTTP 200** | 飞书要求 3 秒内返回 200 状态码,否则判定失败并重试 | Low | 异步处理消息,立即返回 200 |
| **配置模式切换** | 用户需要通过配置文件选择 WebSocket/Webhook 模式 | Low | 配置项: `mode: "websocket"` 或 `"webhook"` |
| **HTTP 端口配置** | 用户环境端口可能冲突,需要可配置 | Low | 配置项: `webhook.port` |
| **Webhook 路径配置** | 用户需要知道配置到飞书的 URL 路径 | Low | 默认 `/webhook/feishu`,可配置 |

## Differentiators

增强体验但非必需的功能。可以延后或作为优化项。

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **签名验证** | 防止伪造请求,验证事件确实来自飞书 | Medium | 需要使用 Verification Token 或 Encrypt Key 计算签名 |
| **消息解密** | 支持飞书的加密推送模式 | Medium | 需实现 AES 解密逻辑,larksuite SDK 可能已提供 |
| **失败重试处理** | 飞书会在 webhook 失败时重试,需要识别重复事件 | Low | 复用现有 `messageCache` 去重逻辑 (已有) |
| **健康检查端点** | 提供 `/health` 端点方便监控 webhook 服务状态 | Low | 返回简单的 200 OK |
| **事件日志记录** | 记录所有接收到的事件类型,方便调试 | Low | log 增强,记录 `event_id`、`event_type` |
| **TLS/HTTPS 支持** | 飞书可能要求 webhook URL 必须是 HTTPS | High | 需要证书管理,或建议用户使用反向代理 |
| **平滑关闭** | 等待正在处理的事件完成后再停止服务 | Low | context.Context 取消机制 |

## Anti-Features

常见的过度设计,应该明确**不实现**的功能。

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **云函数适配 (FaaS)** | 增加复杂度,当前需求是本地部署 | 文档说明如何用 ngrok/frp 做内网穿透 |
| **多 webhook URL 支持** | 飞书一个应用只需要一个 webhook URL,多个没意义 | 单端点,简单清晰 |
| **事件回放/存储** | 增加存储依赖,超出 bridge 职责 | 依赖飞书的重试机制 |
| **Webhook + WebSocket 同时运行** | 两种模式互斥,同时运行会导致重复消息 | 配置文件强制二选一 |
| **复杂路由逻辑** | 只处理消息事件,不需要复杂路由 | 单 handler 函数,简单 if 判断 |
| **自定义 HTTP middleware** | 过度抽象,当前没有需求 | 直接用 http.HandleFunc |
| **事件转发到多个 ClawdBot** | 增加复杂度,当前一个 bridge 对应一个 ClawdBot | 单 ClawdBot client,保持简单 |

## Feature Dependencies

```
URL 验证 (Challenge)
  ↓
HTTP 服务器启动
  ↓
接收 POST 事件 ← 配置模式切换
  ↓
解析 JSON payload
  ↓
事件类型过滤 (只要消息事件)
  ↓
复用 Bridge.HandleMessage (已有)
  ↓
异步处理 + 立即返回 200
```

**关键依赖项:**
- **复用现有逻辑**: Webhook 解析出 `feishu.Message` 后,直接调用现有 `bridge.HandleMessage` 方法
- **去重机制**: 复用现有 `messageCache`,防止飞书重试导致重复处理
- **配置加载**: 扩展现有 `config.Config` 结构,增加 `Mode` 和 `Webhook` 字段

## MVP Recommendation

为 v1.1 milestone,优先级排序:

### Phase 1: 核心功能 (Table Stakes)
1. **URL 验证** — 必须先通过验证才能接收事件
2. **HTTP 服务器** — 监听端口,处理 POST 请求
3. **事件解析** — 将 JSON 转为 `feishu.Message`
4. **立即返回 200** — 异步处理,避免超时

### Phase 2: 生产可用 (部分 Differentiators)
5. **签名验证** — 生产环境安全必需
6. **配置模式切换** — 用户体验关键
7. **健康检查** — 方便监控

### Defer to v1.2 (其他 Differentiators)
- 消息解密 — 大部分用户不启用加密模式
- TLS 支持 — 建议用户用 nginx 反向代理
- 平滑关闭 — 当前影响较小

## Implementation Notes

### 与现有 WebSocket 模式对齐

**相同点:**
- 消息结构 `feishu.Message` 不变
- 去重逻辑 `messageCache` 复用
- 消息处理 `bridge.HandleMessage` 复用
- 飞书 API 调用 (SendMessage, UpdateMessage, DeleteMessage) 不变

**差异点:**
- 事件来源: WebSocket 主动拉取 → Webhook 被动接收
- 连接方式: 长连接 → 短连接 (HTTP)
- 启动流程: `wsClient.Start()` → `http.ListenAndServe()`

### 配置示例

**WebSocket 模式 (现有):**
```json
{
  "feishu": {
    "app_id": "cli_xxx",
    "app_secret": "xxx"
  },
  "mode": "websocket"
}
```

**Webhook 模式 (新增):**
```json
{
  "feishu": {
    "app_id": "cli_xxx",
    "app_secret": "xxx",
    "verification_token": "xxx"
  },
  "mode": "webhook",
  "webhook": {
    "port": 8080,
    "path": "/webhook/feishu"
  }
}
```

### URL 验证流程 (Challenge)

飞书在用户配置 webhook URL 时会发送验证请求:

```json
POST /webhook/feishu
{
  "challenge": "xxx",
  "token": "verification_token",
  "type": "url_verification"
}
```

服务器需返回:
```json
{
  "challenge": "xxx"
}
```

### 消息事件 Payload 结构 (推断)

基于 larksuite SDK 的 WebSocket 模式,webhook 消息事件可能是:

```json
{
  "schema": "2.0",
  "header": {
    "event_id": "xxx",
    "event_type": "im.message.receive_v1",
    "create_time": "1609304491000",
    "token": "xxx"
  },
  "event": {
    "message": {
      "message_id": "om_xxx",
      "chat_id": "oc_xxx",
      "chat_type": "group",
      "message_type": "text",
      "content": "{\"text\":\"hello\"}",
      "mentions": [...]
    }
  }
}
```

**关键字段映射:**
- `header.event_type` → 过滤 `"im.message.receive_v1"`
- `event.message` → 映射到 `feishu.Message` 结构体
- `header.event_id` → 用于去重 (复用 messageCache)

## Security Considerations

| 威胁 | 缓解措施 | 优先级 |
|------|---------|--------|
| 伪造请求 | 签名验证 (verification_token 或 encrypt_key) | HIGH |
| 重放攻击 | event_id 去重 + 时间戳检查 | MEDIUM |
| DDoS | 限流 (当前不实现,依赖用户网络层防护) | LOW (out of scope) |
| 中间人攻击 | HTTPS (建议用户用反向代理) | MEDIUM |

## Testing Strategy

| 测试场景 | 方法 | 备注 |
|---------|------|------|
| URL 验证 | 手动 curl 发送 challenge 请求 | 本地测试 |
| 消息事件 | 手动 curl 发送模拟 payload | 本地测试 |
| 签名验证 | 单元测试 + 手动 curl (带正确/错误签名) | 单元测试 |
| 去重逻辑 | 发送相同 event_id 两次,验证只处理一次 | 单元测试 |
| 异步处理 | 验证 HTTP 响应在 3 秒内返回 | 压力测试 |
| 模式切换 | 修改配置,验证启动正确模式 | 手动测试 |

## Open Questions (需在实现前验证)

由于无法访问飞书官方文档,以下问题需要在实现时通过实验或查阅文档确认:

1. **签名算法细节** — 是用 verification_token 还是 encrypt_key?签名放在哪个 HTTP header?
2. **加密模式详情** — 如果启用加密,payload 是全加密还是部分加密?
3. **重试策略** — 飞书在 webhook 失败时重试几次?间隔多久?
4. **HTTPS 要求** — 飞书是否强制要求 webhook URL 必须是 HTTPS?
5. **超时时间** — 官方文档说 3 秒,是否准确?
6. **Header 字段** — 飞书会在 HTTP header 中放哪些信息 (如 event_id, timestamp)?

**建议:** 实现 Phase 1 后,用实际飞书应用测试,根据真实 HTTP 请求调整实现。

## Sources

**HIGH Confidence:**
- 项目现有代码: `/Users/cookie/GolangProject/moltbotCNAPP/internal/feishu/client.go`
- 项目需求文档: `/Users/cookie/GolangProject/moltbotCNAPP/.planning/PROJECT.md`
- larksuite SDK 使用模式 (从代码推断)

**MEDIUM Confidence:**
- 训练知识中的飞书 webhook 标准流程 (2025年1月前)
- 常见 webhook 实现模式 (challenge 验证、签名校验、异步处理)

**LOW Confidence (需验证):**
- 签名算法细节
- 加密模式实现
- HTTP header 字段名

**重要提示:** 由于 WebSearch 和 WebFetch 不可用,本文档基于现有代码分析和训练知识编写。建议在实现前验证飞书官方文档 (https://open.feishu.cn/document) 中关于事件订阅的章节。
