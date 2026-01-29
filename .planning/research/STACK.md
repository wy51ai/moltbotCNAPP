# 技术栈研究: Feishu Webhook 模式支持

**项目:** ClawdBot Bridge
**当前 Go 版本:** 1.21
**研究日期:** 2026-01-29
**置信度:** HIGH

## 执行摘要

为现有的 ClawdBot Bridge 添加 webhook 模式支持,**无需引入任何新的外部依赖**。larksuite/oapi-sdk-go/v3 SDK 已经完整支持 HTTP webhook 事件处理,包括签名验证、加密解密等所有必需功能。使用 Go 标准库 `net/http` 即可完成集成。

**核心发现:**
- ✅ SDK 原生支持 webhook 模式 (通过 `event/dispatcher` 包)
- ✅ 内置签名验证和加密解密
- ✅ 与现有 WebSocket 代码共享同一事件处理器
- ✅ 标准库 `net/http` 足够,无需第三方框架

---

## 问题 1: larksuite/oapi-sdk-go/v3 是否支持 webhook?

### 回答: 完全支持 ✅

**证据来源:** SDK v3.5.3 源码分析

SDK 提供 `event/dispatcher` 包专门处理 HTTP webhook 回调:

```go
// github.com/larksuite/oapi-sdk-go/v3/event/dispatcher
type EventDispatcher struct {
    eventType2EventHandler map[string]larkevent.EventHandler
    verificationToken      string
    eventEncryptKey        string
    *larkcore.Config
}

// 核心方法 - 处理 HTTP 请求
func (dispatcher *EventDispatcher) Handle(ctx context.Context, req *larkevent.EventReq) *larkevent.EventResp
```

**工作机制:**

1. **接收 HTTP 请求** - 通过 `EventReq` 结构体封装 HTTP 请求
   ```go
   type EventReq struct {
       Header     map[string][]string
       Body       []byte
       RequestURI string
   }
   ```

2. **自动验签** - 通过 `X-Lark-Signature` header 验证请求合法性
   ```go
   func (dispatcher *EventDispatcher) VerifySign(ctx context.Context, req *EventReq) error
   ```

3. **解密消息** - 使用 `eventEncryptKey` 自动解密加密消息
   ```go
   func (dispatcher *EventDispatcher) DecryptEvent(ctx context.Context, cipherEventJsonStr string) (str string, er error)
   ```

4. **路由事件** - 将事件分发到已注册的处理器 (与 WebSocket 模式共享)
   ```go
   handler := dispatcher.eventType2EventHandler[eventType]
   err = handler.Handle(ctx, eventMsg)
   ```

5. **返回响应** - 自动构造符合飞书规范的 HTTP 响应
   ```go
   type EventResp struct {
       Header     http.Header
       Body       []byte
       StatusCode int
   }
   ```

**与现有代码的兼容性:**

当前项目使用的 `dispatcher.NewEventDispatcher().OnP2MessageReceiveV1()` 注册的事件处理器可以**直接复用**于 webhook 模式,无需修改。

**置信度:** HIGH - 基于 SDK v3.5.3 源码直接验证

---

## 问题 2: HTTP 服务器选型

### 推荐: 标准库 `net/http` ✅

**不需要引入 gin、echo 等第三方框架。理由如下:**

### 为什么标准库足够?

1. **需求简单** - 只需一个 HTTP endpoint 接收 webhook POST 请求
2. **SDK 已封装复杂逻辑** - 签名验证、加密解密都在 SDK 内部完成
3. **零依赖增长** - 不增加项目复杂度和 vendor 体积
4. **性能充足** - 飞书 webhook 不需要高并发优化

### 集成模式

```go
import (
    "net/http"
    "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
    larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
)

// 创建 dispatcher (复用现有事件处理器)
eventHandler := dispatcher.NewEventDispatcher(verificationToken, encryptKey).
    OnP2MessageReceiveV1(c.handleMessage) // 复用现有 handler

// HTTP handler 适配器
http.HandleFunc("/webhook/feishu", func(w http.ResponseWriter, r *http.Request) {
    // 读取请求
    body, _ := io.ReadAll(r.Body)

    // 构造 EventReq
    req := &larkevent.EventReq{
        Header:     r.Header,
        Body:       body,
        RequestURI: r.RequestURI,
    }

    // SDK 处理 (自动验签、解密、路由)
    resp := eventHandler.Handle(r.Context(), req)

    // 返回响应
    for k, v := range resp.Header {
        w.Header()[k] = v
    }
    w.WriteHeader(resp.StatusCode)
    w.Write(resp.Body)
})

// 启动服务器
http.ListenAndServe(":8080", nil)
```

### 为什么不用 gin/echo?

| 框架 | 优点 | 缺点 | 是否需要 |
|------|------|------|---------|
| **net/http** | 零依赖、SDK 原生支持 | 无路由增强 | ✅ 推荐 |
| **gin** | 路由、中间件丰富 | 额外依赖、本项目用不到这些特性 | ❌ 过度设计 |
| **echo** | 性能好、轻量 | 仍是额外依赖、性能非瓶颈 | ❌ 不必要 |

**决策:** 使用标准库 `net/http`

**置信度:** HIGH - 基于需求分析和 SDK API 设计

---

## 问题 3: Webhook 签名验证

### 回答: SDK 已内置,开箱即用 ✅

**验证机制 (由 SDK 自动执行):**

```go
// 来源: event/event.go
func Signature(timestamp string, nonce string, eventEncryptKey string, body string) string {
    var b strings.Builder
    b.WriteString(timestamp)
    b.WriteString(nonce)
    b.WriteString(eventEncryptKey)
    b.WriteString(body)
    bs := []byte(b.String())
    h := sha256.New()
    _, _ = h.Write(bs)
    bs = h.Sum(nil)
    return fmt.Sprintf("%x", bs)
}

// 来源: event/dispatcher/dispatcher.go
func (dispatcher *EventDispatcher) VerifySign(ctx context.Context, req *EventReq) error {
    requestTimestamp := req.Header["X-Lark-Request-Timestamp"][0]
    requestNonce := req.Header["X-Lark-Request-Nonce"][0]

    // 计算期望签名
    targetSign := larkevent.Signature(requestTimestamp, requestNonce,
        dispatcher.eventEncryptKey, string(req.Body))

    // 对比请求签名
    sourceSign := req.Header["X-Lark-Signature"][0]

    if targetSign == sourceSign {
        return nil
    }
    return errors.New("the result of signature verification failed")
}
```

**所需 Headers (飞书自动发送):**

- `X-Lark-Request-Timestamp` - 请求时间戳
- `X-Lark-Request-Nonce` - 随机字符串
- `X-Lark-Signature` - SHA256 签名

**开发者需要做的:**

1. 在飞书开放平台配置 `Encrypt Key` (作为签名密钥)
2. 在代码中传入该 key:
   ```go
   dispatcher.NewEventDispatcher(verificationToken, encryptKey)
   ```

**SDK 会自动:**
- 从 HTTP headers 提取签名信息
- 计算本地签名
- 对比验证
- 验证失败时返回错误响应

**跳过验证 (仅开发环境):**

```go
eventHandler.InitConfig(larkevent.WithSkipSignVerify(true))
```

**置信度:** HIGH - 基于 SDK 源码验证

---

## 问题 4: 额外依赖

### 回答: 无需任何新依赖 ✅

**当前 go.mod 依赖:**

```go
require (
    github.com/google/uuid v1.6.0
    github.com/gorilla/websocket v1.5.1
)

require (
    github.com/gogo/protobuf v1.3.2 // indirect
    github.com/larksuite/oapi-sdk-go/v3 v3.5.3
    golang.org/x/net v0.17.0 // indirect
)
```

**Webhook 模式所需的包 (已存在):**

- ✅ `github.com/larksuite/oapi-sdk-go/v3/event` - 已依赖
- ✅ `github.com/larksuite/oapi-sdk-go/v3/event/dispatcher` - 已依赖
- ✅ `net/http` - Go 标准库
- ✅ `crypto/sha256` - Go 标准库 (SDK 已使用)
- ✅ `crypto/aes` - Go 标准库 (SDK 已使用)

**不需要添加:**

- ❌ HTTP 框架 (gin/echo/fiber) - 标准库足够
- ❌ 签名验证库 - SDK 已内置
- ❌ 加密库 - SDK 已内置

**置信度:** HIGH - 基于 go.mod 和 SDK 依赖分析

---

## 推荐技术栈 (Webhook 新增部分)

### 核心组件

| 组件 | 库/包 | 版本 | 说明 |
|------|-------|------|------|
| HTTP 服务器 | `net/http` | Go 标准库 | 接收 webhook POST 请求 |
| 事件处理 | `github.com/larksuite/oapi-sdk-go/v3/event/dispatcher` | v3.5.3 (已有) | 与 WebSocket 共享 |
| 签名验证 | SDK 内置 | v3.5.3 | 自动验证 `X-Lark-Signature` |
| 消息解密 | SDK 内置 | v3.5.3 | AES-CBC 解密 |

### 配置变更

需要在 `~/.clawdbot/bridge.json` 添加:

```json
{
  "feishu": {
    "app_id": "cli_xxx",
    "app_secret": "xxx",
    "mode": "webhook",              // 新增: webhook | websocket
    "webhook": {                     // 新增配置块
      "port": 8080,
      "verification_token": "xxx",   // 飞书开放平台获取
      "encrypt_key": "xxx"           // 飞书开放平台获取
    }
  }
}
```

### 代码结构建议

```
internal/feishu/
├── client.go           # 现有 WebSocket 客户端
├── webhook.go          # 新增 HTTP webhook 服务器
└── handler.go          # 共享消息处理逻辑 (重构提取)
```

**重用策略:**

1. **事件处理器共享** - `handleMessage()` 逻辑两种模式通用
2. **API 调用共享** - `SendMessage()` / `UpdateMessage()` 等方法通用
3. **配置共享** - `appID` / `appSecret` 两种模式通用

---

## 不推荐的替代方案

### ❌ 方案 A: 使用 Gin 框架

**为什么不推荐:**

- 引入 10+ 个传递依赖 (httprouter, go-playground/validator 等)
- 本项目只需一个 endpoint,用不到路由、中间件、参数绑定等特性
- 增加学习成本和维护负担

**什么时候可以考虑:**
- 如果未来需要实现 10+ 个 HTTP endpoint
- 需要复杂的中间件链 (认证、限流、日志等)

### ❌ 方案 B: 使用 Echo 框架

**为什么不推荐:**

- 虽然比 Gin 轻量,但仍是额外依赖
- SDK 的 `EventReq` / `EventResp` 结构已经封装了所有需要的东西
- Echo 的高性能在 webhook 场景下无意义 (飞书 webhook QPS 很低)

### ❌ 方案 C: 自己实现签名验证

**为什么不推荐:**

- SDK 已经实现且经过验证
- 重复造轮子,容易引入安全漏洞
- 飞书签名算法有细节 (timestamp + nonce + encryptKey + body),手写容易出错

---

## 安全考虑

### 必须启用的安全措施

1. **签名验证** (默认启用)
   ```go
   // 生产环境必须验签
   dispatcher.NewEventDispatcher(token, encryptKey)
   ```

2. **消息加密** (推荐)
   ```go
   // 在飞书开放平台启用"消息加密"
   // SDK 会自动解密
   ```

3. **HTTPS** (生产环境必须)
   ```go
   // 使用 Let's Encrypt 或云服务商证书
   http.ListenAndServeTLS(":443", "cert.pem", "key.pem", nil)
   ```

### 开发环境简化选项

```go
// 仅用于本地开发,生产必须关闭
eventHandler.InitConfig(larkevent.WithSkipSignVerify(true))
```

---

## 性能评估

### 预期负载

- **典型场景:** 10-100 消息/小时
- **峰值场景:** 1000 消息/小时
- **并发连接:** 1-5 (飞书 webhook 串行发送)

### 标准库性能

- `net/http` 单机可处理 10,000+ req/s
- 本项目瓶颈在 ClawdBot Gateway 调用,不在 HTTP 接收
- **结论:** 标准库性能远超需求

### 资源占用

- **内存:** 与 WebSocket 模式相同 (~50MB)
- **CPU:** webhook 模式更低 (无需保持长连接)
- **网络:** 相同 (都是接收消息 + 调用 API)

---

## 开发工作量估算

### 核心实现

| 任务 | 工作量 | 说明 |
|------|--------|------|
| HTTP server 启动 | 30 行 | `net/http` 标准模板 |
| dispatcher 集成 | 10 行 | 复用现有事件处理器 |
| 配置读取 | 20 行 | 扩展 `config.Config` 结构 |
| 模式切换逻辑 | 30 行 | 根据配置启动不同模式 |
| **总计** | **~100 行** | 不含测试 |

### 测试工作

- 本地测试: 使用 ngrok 暴露本地端口给飞书
- 签名验证测试: 使用飞书开放平台"测试推送"功能
- 降级测试: 验证 webhook 模式下 API 调用正常

---

## 迁移建议

### Phase 1: 配置扩展

1. 扩展 `config.Config` 支持 webhook 配置
2. 添加 `mode` 字段选择运行模式

### Phase 2: 代码重构

1. 提取共享消息处理逻辑到独立函数
2. 保持 API 调用方法不变 (SendMessage 等)

### Phase 3: Webhook 实现

1. 创建 `internal/feishu/webhook.go`
2. 实现 HTTP server 和 dispatcher 集成
3. 添加模式切换逻辑

### Phase 4: 文档更新

1. 更新 README 添加 webhook 配置说明
2. 添加 ngrok 本地测试教程

---

## 最终推荐

```go
// 无需修改 go.mod - 不添加任何新依赖

// internal/feishu/webhook.go
package feishu

import (
    "context"
    "io"
    "log"
    "net/http"

    larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
    "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
)

type WebhookServer struct {
    port              int
    verificationToken string
    encryptKey        string
    handler           MessageHandler
}

func (s *WebhookServer) Start(ctx context.Context) error {
    // 创建 dispatcher (复用 WebSocket 的事件处理器)
    eventHandler := dispatcher.NewEventDispatcher(
        s.verificationToken,
        s.encryptKey,
    ).OnP2MessageReceiveV1(s.handleMessage)

    // HTTP handler
    http.HandleFunc("/webhook/feishu", func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        defer r.Body.Close()

        req := &larkevent.EventReq{
            Header:     r.Header,
            Body:       body,
            RequestURI: r.RequestURI,
        }

        resp := eventHandler.Handle(r.Context(), req)

        for k, v := range resp.Header {
            w.Header()[k] = v
        }
        w.WriteHeader(resp.StatusCode)
        w.Write(resp.Body)
    })

    log.Printf("[Feishu] Starting webhook server on :%d", s.port)
    return http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil)
}

// handleMessage 与 WebSocket 的实现完全相同
func (s *WebhookServer) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
    // 复用现有逻辑
}
```

**优势总结:**

- ✅ 零新依赖
- ✅ 代码量少 (~100 行)
- ✅ 复用现有逻辑
- ✅ SDK 原生支持
- ✅ 安全性有保障

---

## 信心评估

| 领域 | 置信度 | 依据 |
|------|--------|------|
| SDK 支持 webhook | **HIGH** | 源码直接验证,API 完整 |
| 标准库足够 | **HIGH** | 需求简单,SDK 已封装复杂逻辑 |
| 签名验证机制 | **HIGH** | SDK 内置实现,源码可见 |
| 无需新依赖 | **HIGH** | go.mod 分析 + SDK 包结构 |
| 开发工作量 | **MEDIUM** | 基于估算,实际可能有配置细节 |

---

## 遗留问题

### 需要在实现阶段验证的细节

1. **Challenge 验证流程**
   - 飞书 webhook 配置时会发送 challenge 请求
   - SDK 的 `AuthByChallenge()` 方法已处理,需测试确认

2. **超时配置**
   - 飞书要求 webhook 在 3 秒内响应
   - 如果 ClawdBot Gateway 调用超时,需要异步处理

3. **重试机制**
   - 飞书会重试失败的 webhook
   - 需要确保消息处理幂等性

4. **端口选择**
   - 生产环境建议 443 (HTTPS)
   - 开发环境可用 ngrok 转发

---

## 参考资源

### SDK 官方文档

- [Handle Events](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/server-side-sdk/golang-sdk-guide/handle-events)
- [SDK GitHub](https://github.com/larksuite/oapi-sdk-go)

### 飞书开放平台

- [事件订阅概述](https://open.feishu.cn/document/ukTMukTMukTM/uUTNz4SN1MjL1UzM)
- [Webhook 配置指南](https://open.feishu.cn/document/ukTMukTMukTM/uYDNxYjL2QTM24iN0EjN)

### Go 标准库

- [net/http](https://pkg.go.dev/net/http@go1.21)
- [crypto/sha256](https://pkg.go.dev/crypto/sha256)

---

## 结论

为 ClawdBot Bridge 添加 webhook 模式是一个**低风险、低成本**的增强,完全基于现有技术栈即可实现。

**关键决策:**
- ✅ 使用 `net/http` 标准库
- ✅ 复用 SDK 的 `event/dispatcher`
- ✅ 不引入任何新依赖
- ✅ 共享 WebSocket 的事件处理逻辑

**下一步:**
- 扩展配置结构支持 webhook 参数
- 实现 HTTP server 和 dispatcher 集成
- 使用 ngrok 进行本地测试
