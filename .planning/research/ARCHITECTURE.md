# 架构集成设计: Webhook 模式

**项目:** ClawdBot Bridge
**研究时间:** 2026-01-29
**置信度:** HIGH (基于现有代码结构分析)

## 执行摘要

Webhook 模式将作为 WebSocket 模式的**替代方案**集成到现有架构中。两种模式共享核心消息处理逻辑 (`Bridge.HandleMessage()`)，但使用不同的消息接收机制。

**核心设计原则:**
1. **模式切换通过配置** - 用户在 `bridge.json` 中指定 `mode: "websocket" | "webhook"`
2. **共享消息处理逻辑** - 两种模式都调用同一个 `Bridge.HandleMessage()`
3. **最小化代码重复** - Feishu 客户端逻辑抽象为接口，消息发送/更新/删除共用
4. **优雅关闭支持** - HTTP server 支持 context 驱动的优雅关闭

---

## 当前架构分析

### 现有组件

```
cmd/bridge/main.go              # 入口，守护进程生命周期
    ├── cmdRun()                # 核心启动逻辑
    ├── cmdStart()              # 守护进程启动
    └── cmdStop()               # 守护进程停止

internal/config/config.go       # 配置加载
    └── Load() -> Config        # 从 ~/.clawdbot/ 读取配置

internal/feishu/client.go       # Feishu WebSocket 客户端
    ├── Start(ctx)              # WebSocket 连接启动
    ├── handleMessage()         # 接收消息，调用 handler
    ├── SendMessage()           # 发送文本消息
    ├── UpdateMessage()         # 更新已发消息
    └── DeleteMessage()         # 删除消息

internal/bridge/bridge.go       # 消息路由和去重
    └── HandleMessage(msg)      # 核心处理逻辑

internal/clawdbot/client.go     # ClawdBot Gateway 客户端
    └── AskClawdbot(text, key)  # 调用 AI，返回回复
```

### 数据流 (当前 WebSocket 模式)

```
Feishu 服务器
    |
    | (WebSocket)
    v
feishu.Client.Start()
    |
    | (handleMessage)
    v
bridge.HandleMessage()
    |
    | (去重、清理、判断是否响应)
    |
    +--> clawdbot.AskClawdbot() --> ClawdBot Gateway
    |
    +--> feishu.SendMessage()   --> Feishu 服务器
```

### 关键依赖关系

1. **循环依赖解决**: `Bridge` 和 `feishu.Client` 通过延迟注入解耦
   - `Bridge` 构造时接受 `feishuClient` 参数 (可为 nil)
   - `feishu.Client` 构造时接受 `handler MessageHandler`
   - `main.go` 中先创建 Bridge，再创建 feishu.Client，最后 `SetFeishuClient()`

2. **消息处理器签名**: `type MessageHandler func(msg *Message) error`

3. **配置来源**:
   - `~/.clawdbot/clawdbot.json` - Gateway 配置 (ClawdBot 管理)
   - `~/.clawdbot/bridge.json` - Feishu 配置 (用户管理)

---

## Webhook 模式架构设计

### 新增组件

#### 1. `internal/feishu/webhook.go` - Webhook HTTP Server

**职责:**
- 接收 Feishu Webhook 回调请求
- 验证请求签名 (URL Verification + Event Encryption)
- 解析 Webhook 事件，转换为 `feishu.Message`
- 调用 `MessageHandler`
- 返回 HTTP 响应

**接口:**
```go
type WebhookServer struct {
    appID         string
    appSecret     string
    verifyToken   string        // 用于 URL 验证
    encryptKey    string        // 用于事件解密 (可选)
    handler       MessageHandler
    server        *http.Server
}

func NewWebhookServer(appID, appSecret, verifyToken, encryptKey string, handler MessageHandler) *WebhookServer
func (w *WebhookServer) Start(ctx context.Context, addr string) error
```

**关键实现点:**
- **URL 验证处理**: 首次配置时 Feishu 会发送 `url_verification` 事件
- **事件解密**: 如果配置了加密，需要先解密 `encrypt` 字段
- **重放攻击防护**: 检查 `timestamp` 避免旧事件重放
- **快速响应**: Webhook 回调要求 3 秒内响应，异步处理消息

#### 2. `internal/feishu/interface.go` - Feishu 客户端接口抽象

**目的:** 让 `Bridge` 不依赖具体的 WebSocket 或 Webhook 实现

```go
// FeishuClient 定义 Feishu 消息发送能力
type FeishuClient interface {
    SendMessage(chatID, text string) (string, error)
    UpdateMessage(messageID, text string) error
    DeleteMessage(messageID string) error
}

// MessageReceiver 定义 Feishu 消息接收能力
type MessageReceiver interface {
    Start(ctx context.Context) error
}
```

**重构现有代码:**
- `feishu.Client` (WebSocket) 实现 `FeishuClient` 和 `MessageReceiver`
- `feishu.WebhookServer` 实现 `FeishuClient` 和 `MessageReceiver`
- `Bridge.feishuClient` 类型改为 `FeishuClient` 接口

#### 3. `internal/feishu/rest.go` - REST API 客户端 (共享)

**职责:** 将消息发送/更新/删除逻辑从 `client.go` 提取为共享函数

```go
// RESTClient 封装 Feishu Open API 调用
type RESTClient struct {
    client *lark.Client
}

func NewRESTClient(appID, appSecret string) *RESTClient
func (r *RESTClient) SendMessage(chatID, text string) (string, error)
func (r *RESTClient) UpdateMessage(messageID, text string) error
func (r *RESTClient) DeleteMessage(messageID string) error
```

**使用场景:**
- `feishu.Client` (WebSocket) 内嵌 `RESTClient`，复用消息发送逻辑
- `feishu.WebhookServer` 内嵌 `RESTClient`，复用消息发送逻辑

### 配置扩展

#### `~/.clawdbot/bridge.json` 新增字段

```json
{
  "mode": "webhook",              // 新增: "websocket" | "webhook"
  "feishu": {
    "app_id": "cli_xxx",
    "app_secret": "xxx",
    "verify_token": "xxx",        // 新增: Webhook URL 验证令牌
    "encrypt_key": ""             // 新增: 事件加密密钥 (可选)
  },
  "webhook": {                    // 新增: Webhook 专属配置
    "listen_addr": ":8080"        // 监听地址，默认 ":8080"
  },
  "thinking_threshold_ms": 3000,
  "agent_id": "main"
}
```

#### `internal/config/config.go` 结构扩展

```go
type Config struct {
    Mode     string            // "websocket" | "webhook"
    Feishu   FeishuConfig
    Webhook  WebhookConfig    // 新增
    Clawdbot ClawdbotConfig
}

type FeishuConfig struct {
    AppID               string
    AppSecret           string
    VerifyToken         string  // 新增
    EncryptKey          string  // 新增
    ThinkingThresholdMs int
}

type WebhookConfig struct {
    ListenAddr string           // 新增，默认 ":8080"
}
```

---

## 数据流对比

### WebSocket 模式 (现有)

```
Feishu 服务器
    |
    | WebSocket 推送
    v
feishu.Client.Start()
    |
    | event dispatcher
    v
handleMessage()
    |
    | 解析事件，构造 feishu.Message
    v
bridge.HandleMessage()
    |
    | 去重、清理、判断响应
    v
clawdbot.AskClawdbot()
    |
    | WebSocket 调用 Gateway
    v
ClawdBot Gateway
    |
    | 返回 AI 回复
    v
feishu.Client.SendMessage()
    |
    | REST API 发送
    v
Feishu 服务器
```

### Webhook 模式 (新增)

```
Feishu 服务器
    |
    | HTTP POST /webhook
    v
feishu.WebhookServer (HTTP Handler)
    |
    | 验证签名、解密、解析
    v
转换为 feishu.Message
    |
    | handler(msg)
    v
bridge.HandleMessage()
    |
    | 去重、清理、判断响应
    v
clawdbot.AskClawdbot()
    |
    | WebSocket 调用 Gateway
    v
ClawdBot Gateway
    |
    | 返回 AI 回复
    v
feishu.RESTClient.SendMessage()
    |
    | REST API 发送
    v
Feishu 服务器
```

**关键差异:**
- **消息接收**: WebSocket 长连接 vs HTTP 短请求
- **消息发送**: 两种模式都使用 REST API (共享代码)
- **核心处理**: 完全相同 (`bridge.HandleMessage()`)

---

## 启动流程变化

### 现有启动流程 (WebSocket)

```go
// cmd/bridge/main.go :: cmdRun()
cfg := config.Load()
clawdbotClient := clawdbot.NewClient(...)
bridgeInstance := bridge.NewBridge(nil, clawdbotClient, ...)
feishuClient := feishu.NewClient(..., bridgeInstance.HandleMessage)
bridgeInstance.SetFeishuClient(feishuClient)

ctx, cancel := context.WithCancel(context.Background())
go feishuClient.Start(ctx)

// 等待信号
<-sigChan
cancel()
```

### 新启动流程 (Webhook)

```go
// cmd/bridge/main.go :: cmdRun()
cfg := config.Load()
clawdbotClient := clawdbot.NewClient(...)
bridgeInstance := bridge.NewBridge(nil, clawdbotClient, ...)

ctx, cancel := context.WithCancel(context.Background())

var receiver feishu.MessageReceiver

switch cfg.Mode {
case "webhook":
    webhookServer := feishu.NewWebhookServer(
        cfg.Feishu.AppID,
        cfg.Feishu.AppSecret,
        cfg.Feishu.VerifyToken,
        cfg.Feishu.EncryptKey,
        bridgeInstance.HandleMessage,
    )
    bridgeInstance.SetFeishuClient(webhookServer)
    receiver = webhookServer

    log.Printf("[Main] Starting webhook server on %s", cfg.Webhook.ListenAddr)
    go receiver.Start(ctx, cfg.Webhook.ListenAddr)

case "websocket":
default:
    feishuClient := feishu.NewClient(
        cfg.Feishu.AppID,
        cfg.Feishu.AppSecret,
        bridgeInstance.HandleMessage,
    )
    bridgeInstance.SetFeishuClient(feishuClient)
    receiver = feishuClient

    log.Println("[Main] Starting websocket client")
    go receiver.Start(ctx)
}

// 等待信号
<-sigChan
cancel()

// 等待 HTTP server 优雅关闭 (如果是 webhook 模式)
time.Sleep(time.Second)
```

---

## 优雅关闭处理

### WebSocket 模式

现有代码已经支持:
```go
ctx, cancel := context.WithCancel(...)
go feishuClient.Start(ctx)

<-sigChan
cancel()  // 触发 ctx.Done()，WebSocket 客户端退出
```

### Webhook 模式

HTTP server 需要优雅关闭:

```go
// internal/feishu/webhook.go
func (w *WebhookServer) Start(ctx context.Context, addr string) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/webhook", w.handleWebhook)

    w.server = &http.Server{
        Addr:    addr,
        Handler: mux,
    }

    // 启动服务器
    go func() {
        if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Printf("[Webhook] Server error: %v", err)
        }
    }()

    // 等待 context 取消
    <-ctx.Done()

    // 优雅关闭，5 秒超时
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    return w.server.Shutdown(shutdownCtx)
}
```

**关键点:**
- `Shutdown()` 会等待现有连接处理完成
- 超时后强制关闭
- 返回错误供调用者记录

---

## 集成点总结

### 新增组件

| 组件 | 路径 | 职责 |
|------|------|------|
| `WebhookServer` | `internal/feishu/webhook.go` | HTTP 服务器，接收 Webhook 回调 |
| `RESTClient` | `internal/feishu/rest.go` | 封装 Feishu REST API 调用 |
| `FeishuClient` 接口 | `internal/feishu/interface.go` | 抽象消息发送能力 |
| `MessageReceiver` 接口 | `internal/feishu/interface.go` | 抽象消息接收能力 |

### 修改组件

| 组件 | 修改内容 |
|------|----------|
| `internal/config/config.go` | 添加 `Mode`, `WebhookConfig`, `VerifyToken`, `EncryptKey` |
| `internal/feishu/client.go` | 提取 REST 调用到 `RESTClient`，实现 `FeishuClient` 接口 |
| `internal/bridge/bridge.go` | `feishuClient` 类型改为 `FeishuClient` 接口 |
| `cmd/bridge/main.go` | 根据 `cfg.Mode` 选择启动 WebSocket 或 Webhook |

### 不修改组件

| 组件 | 原因 |
|------|------|
| `internal/clawdbot/client.go` | 与 Feishu 模式无关 |
| `internal/bridge/bridge.go` 核心逻辑 | `HandleMessage()` 逻辑通用，两种模式共享 |

---

## 建议的构建顺序

### Phase 1: 接口抽象和 REST 提取

**目标:** 重构现有代码，为 Webhook 模式铺路

1. 创建 `internal/feishu/interface.go`
   - 定义 `FeishuClient` 接口
   - 定义 `MessageReceiver` 接口

2. 创建 `internal/feishu/rest.go`
   - 实现 `RESTClient` 结构体
   - 从 `client.go` 迁移 `SendMessage/UpdateMessage/DeleteMessage`

3. 重构 `internal/feishu/client.go`
   - 内嵌 `RESTClient`
   - 实现 `FeishuClient` 和 `MessageReceiver` 接口

4. 更新 `internal/bridge/bridge.go`
   - `feishuClient` 类型改为 `FeishuClient` 接口

**验证:** WebSocket 模式功能不受影响

### Phase 2: Webhook Server 实现

**目标:** 实现 Webhook HTTP 服务器

1. 创建 `internal/feishu/webhook.go`
   - 实现 `WebhookServer` 结构体
   - 实现 URL 验证逻辑
   - 实现事件解析逻辑
   - 实现 `Start()` 和优雅关闭

2. 实现事件解密 (如果需要)
   - 参考 Feishu 文档的 AES 解密算法

**验证:** 单独测试 Webhook 接收和解析

### Phase 3: 配置扩展

**目标:** 支持模式选择配置

1. 更新 `internal/config/config.go`
   - 添加 `Mode`, `WebhookConfig` 结构
   - 添加 `VerifyToken`, `EncryptKey` 字段
   - 添加默认值处理

2. 更新配置验证逻辑
   - Webhook 模式必须提供 `verify_token`
   - `listen_addr` 默认 `:8080`

**验证:** 配置加载和验证正确

### Phase 4: 主程序集成

**目标:** 在 main.go 中集成模式切换

1. 更新 `cmd/bridge/main.go :: cmdRun()`
   - 添加 `switch cfg.Mode` 逻辑
   - 创建对应的 `MessageReceiver`
   - 统一启动和关闭流程

**验证:**
- WebSocket 模式功能正常
- Webhook 模式启动成功
- 优雅关闭正常

### Phase 5: 端到端测试

**目标:** 验证 Webhook 模式完整流程

1. 配置 Feishu 应用为 Webhook 模式
2. 启动 Bridge (webhook 模式)
3. 发送测试消息
4. 验证 AI 回复

---

## 架构决策记录

### ADR-1: 为什么使用接口抽象而不是类型断言？

**决策:** 使用 `FeishuClient` 接口抽象消息发送能力

**理由:**
- **类型安全**: 编译时检查，避免运行时错误
- **可测试性**: 可以轻松 mock `FeishuClient` 进行单元测试
- **扩展性**: 未来可能支持其他 IM 平台 (如钉钉、企业微信)
- **清晰职责**: 接口明确定义了 Bridge 需要的能力

**替代方案:**
- 使用 `interface{}` + 类型断言: 类型不安全，易出错
- 直接依赖 `feishu.Client`: 耦合紧，难以扩展

### ADR-2: 为什么 REST 客户端独立？

**决策:** 将消息发送逻辑提取为独立的 `RESTClient`

**理由:**
- **代码复用**: WebSocket 和 Webhook 模式都需要发送消息
- **单一职责**: `Client` 负责接收，`RESTClient` 负责发送
- **测试隔离**: 可以独立测试发送逻辑

**替代方案:**
- 在 `client.go` 和 `webhook.go` 中重复代码: 维护成本高
- 使用全局函数: 缺少状态管理 (如 lark.Client 实例)

### ADR-3: 为什么 Webhook 处理器快速返回？

**决策:** Webhook HTTP handler 收到请求后快速返回，异步处理消息

**理由:**
- **Feishu 要求**: Webhook 回调要求 3 秒内响应
- **避免超时**: AI 处理可能超过 3 秒
- **提升吞吐**: 不阻塞 HTTP 连接

**实现:**
```go
func (w *WebhookServer) handleWebhook(rw http.ResponseWriter, req *http.Request) {
    // 1. 验证签名
    // 2. 解析事件
    msg := parseMessage(event)

    // 3. 快速响应
    rw.WriteHeader(http.StatusOK)
    rw.Write([]byte(`{"ok":true}`))

    // 4. 异步处理 (和 WebSocket 模式一致)
    if w.handler != nil {
        go w.handler(msg)
    }
}
```

### ADR-4: 为什么配置模式字段是字符串而不是枚举？

**决策:** `mode: "websocket"` 而不是 `mode: 1`

**理由:**
- **可读性**: JSON 配置文件中字符串更清晰
- **可扩展**: 未来可能添加 `"http-long-polling"` 等模式
- **兼容性**: 向后兼容，默认值为 `"websocket"`

**验证:**
```go
switch cfg.Mode {
case "webhook":
    // ...
case "websocket", "":  // 空值默认 WebSocket
default:
    return fmt.Errorf("unsupported mode: %s", cfg.Mode)
}
```

---

## 风险和缓解

### 风险 1: Webhook 签名验证复杂

**风险:** Feishu Webhook 签名算法可能实现错误，导致验证失败

**缓解:**
1. 参考 Feishu SDK 官方代码 (larksuite/oapi-sdk-go)
2. 先实现无加密模式 (仅 URL 验证)
3. 单独测试签名验证逻辑

**检测:** 如果 Webhook 总是返回 401/403，检查签名逻辑

### 风险 2: 接口抽象过度

**风险:** 过度设计接口导致代码复杂

**缓解:**
- 只抽象当前需要的能力 (`SendMessage`, `UpdateMessage`, `DeleteMessage`)
- 不提前设计"未来可能需要"的接口
- 保持接口简单，最多 3-5 个方法

**检测:** 如果接口有超过 5 个方法，重新审视设计

### 风险 3: HTTP Server 端口冲突

**风险:** 默认端口 8080 可能被占用

**缓解:**
1. 配置文件支持自定义端口
2. 启动失败时给出明确错误提示
3. 文档说明如何修改端口

**检测:** 启动日志中显示实际监听地址

### 风险 4: 优雅关闭不完整

**风险:** Webhook 模式下有正在处理的请求时关闭服务

**缓解:**
- 使用 `http.Server.Shutdown()` 而不是 `Close()`
- 设置 5 秒超时，避免无限等待
- 记录未完成的请求数量

**检测:**
```go
// 在 Shutdown 前记录活跃连接
log.Printf("[Webhook] Shutting down, active connections: %d", activeConns)
```

---

## 测试策略

### 单元测试

| 组件 | 测试重点 |
|------|----------|
| `RESTClient` | Mock Feishu API，验证请求构造 |
| `WebhookServer` | Mock HTTP 请求，验证签名和解析 |
| `config.Load()` | 验证配置字段解析和默认值 |

### 集成测试

1. **WebSocket 模式不受影响**
   - 重构后 WebSocket 功能仍正常

2. **Webhook 接收消息**
   - 模拟 Feishu Webhook 请求
   - 验证 `HandleMessage()` 被调用

3. **模式切换**
   - 修改配置文件
   - 重启服务
   - 验证使用正确的模式

### 端到端测试

1. 配置真实 Feishu 应用 (测试环境)
2. 启动 Webhook 模式 Bridge
3. 通过 ngrok 暴露本地端口
4. 在 Feishu 中发送消息
5. 验证 AI 回复

---

## 性能考量

### Webhook 模式优势

- **无长连接开销**: 不需要维持 WebSocket 连接
- **横向扩展友好**: 可以启动多个实例，通过负载均衡分发

### Webhook 模式劣势

- **网络延迟**: 每次消息都是独立 HTTP 请求
- **需要公网地址**: 本地开发需要 ngrok 等工具

### 性能优化建议

1. **消息去重缓存**: 现有 `messageCache` 已支持，Webhook 模式复用
2. **并发限制**: 如果消息量大，考虑限制并发处理数
3. **连接池**: `RESTClient` 内部的 `lark.Client` 应复用 HTTP 连接

---

## 文档和运维

### 用户文档需更新

1. **README.md**
   - 添加 Webhook 模式配置说明
   - 添加 Feishu 应用配置步骤 (URL 验证、事件订阅)

2. **配置示例**
   - 提供 `bridge.json` 的 Webhook 模式模板

3. **故障排查**
   - Webhook 无法接收消息 → 检查网络和防火墙
   - URL 验证失败 → 检查 `verify_token`

### 运维监控

1. **日志增强**
   - Webhook 模式: 记录请求来源 IP
   - 记录签名验证结果
   - 记录每个 Webhook 请求的处理时间

2. **健康检查**
   - 添加 `/health` 端点 (仅 Webhook 模式)
   - 返回服务状态和版本信息

---

## 总结

### 关键集成点

1. **接口抽象**: `FeishuClient` 和 `MessageReceiver` 解耦模式
2. **代码复用**: `RESTClient` 和 `Bridge.HandleMessage()` 共享
3. **配置驱动**: 通过 `mode` 字段切换模式
4. **优雅关闭**: 两种模式都支持 context 驱动的关闭

### 建议的实现顺序

1. **Phase 1**: 接口抽象和 REST 提取 (重构现有代码)
2. **Phase 2**: Webhook Server 实现 (新增功能)
3. **Phase 3**: 配置扩展 (支持模式选择)
4. **Phase 4**: 主程序集成 (模式切换逻辑)
5. **Phase 5**: 端到端测试 (验证完整流程)

### 成功标准

- [ ] WebSocket 模式功能不受影响
- [ ] Webhook 模式可以正常接收消息
- [ ] 两种模式共享消息处理逻辑
- [ ] 配置切换模式无需修改代码
- [ ] 优雅关闭在两种模式下都正常
- [ ] 代码无重复 (DRY 原则)

---

## 附录: 代码示例

### A. FeishuClient 接口定义

```go
// internal/feishu/interface.go
package feishu

import "context"

// FeishuClient 定义 Feishu 消息发送能力
type FeishuClient interface {
    SendMessage(chatID, text string) (messageID string, err error)
    UpdateMessage(messageID, text string) error
    DeleteMessage(messageID string) error
}

// MessageReceiver 定义 Feishu 消息接收能力
type MessageReceiver interface {
    Start(ctx context.Context) error
}

// 确保类型实现接口 (编译时检查)
var _ FeishuClient = (*Client)(nil)
var _ MessageReceiver = (*Client)(nil)
var _ FeishuClient = (*WebhookServer)(nil)
var _ MessageReceiver = (*WebhookServer)(nil)
```

### B. Webhook Server 核心结构

```go
// internal/feishu/webhook.go
package feishu

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"
)

type WebhookServer struct {
    restClient  *RESTClient
    verifyToken string
    encryptKey  string
    handler     MessageHandler
    server      *http.Server
}

func NewWebhookServer(appID, appSecret, verifyToken, encryptKey string, handler MessageHandler) *WebhookServer {
    return &WebhookServer{
        restClient:  NewRESTClient(appID, appSecret),
        verifyToken: verifyToken,
        encryptKey:  encryptKey,
        handler:     handler,
    }
}

func (w *WebhookServer) Start(ctx context.Context) error {
    // 从配置获取监听地址
    addr := ":8080"  // 实际应从 config 传入

    mux := http.NewServeMux()
    mux.HandleFunc("/webhook", w.handleWebhook)
    mux.HandleFunc("/health", w.handleHealth)

    w.server = &http.Server{
        Addr:         addr,
        Handler:      mux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
    }

    errChan := make(chan error, 1)

    // 启动服务器
    go func() {
        log.Printf("[Webhook] Listening on %s", addr)
        if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            errChan <- err
        }
    }()

    // 等待 context 取消或启动失败
    select {
    case <-ctx.Done():
        log.Println("[Webhook] Shutting down...")
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        return w.server.Shutdown(shutdownCtx)
    case err := <-errChan:
        return err
    }
}

func (w *WebhookServer) handleWebhook(rw http.ResponseWriter, req *http.Request) {
    // 只接受 POST
    if req.Method != http.MethodPost {
        http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // 读取请求体
    body, err := io.ReadAll(req.Body)
    if err != nil {
        log.Printf("[Webhook] Failed to read body: %v", err)
        http.Error(rw, "Bad request", http.StatusBadRequest)
        return
    }
    defer req.Body.Close()

    // 解析事件
    var event struct {
        Challenge string `json:"challenge"` // URL 验证
        Type      string `json:"type"`
        Event     struct {
            Message struct {
                MessageID   string `json:"message_id"`
                ChatID      string `json:"chat_id"`
                ChatType    string `json:"chat_type"`
                MessageType string `json:"message_type"`
                Content     string `json:"content"`
            } `json:"message"`
        } `json:"event"`
    }

    if err := json.Unmarshal(body, &event); err != nil {
        log.Printf("[Webhook] Failed to parse event: %v", err)
        http.Error(rw, "Bad request", http.StatusBadRequest)
        return
    }

    // 处理 URL 验证
    if event.Challenge != "" {
        rw.Header().Set("Content-Type", "application/json")
        json.NewEncoder(rw).Encode(map[string]string{"challenge": event.Challenge})
        return
    }

    // 快速响应 (必须在 3 秒内)
    rw.WriteHeader(http.StatusOK)
    rw.Write([]byte(`{"ok":true}`))

    // 异步处理消息
    if event.Type == "im.message.receive_v1" {
        msg := w.parseMessage(&event)
        if w.handler != nil {
            go w.handler(msg)
        }
    }
}

func (w *WebhookServer) parseMessage(event *struct {
    Type  string
    Event struct {
        Message struct {
            MessageID   string
            ChatID      string
            ChatType    string
            MessageType string
            Content     string
        }
    }
}) *Message {
    // 解析消息内容
    var content struct {
        Text string `json:"text"`
    }
    json.Unmarshal([]byte(event.Event.Message.Content), &content)

    return &Message{
        MessageID: event.Event.Message.MessageID,
        ChatID:    event.Event.Message.ChatID,
        ChatType:  event.Event.Message.ChatType,
        Content:   content.Text,
        Mentions:  []Mention{},  // Webhook 事件中包含 mentions，需解析
    }
}

func (w *WebhookServer) handleHealth(rw http.ResponseWriter, req *http.Request) {
    rw.Header().Set("Content-Type", "application/json")
    json.NewEncoder(rw).Encode(map[string]string{
        "status":  "ok",
        "version": "0.1.0",
    })
}

// 实现 FeishuClient 接口
func (w *WebhookServer) SendMessage(chatID, text string) (string, error) {
    return w.restClient.SendMessage(chatID, text)
}

func (w *WebhookServer) UpdateMessage(messageID, text string) error {
    return w.restClient.UpdateMessage(messageID, text)
}

func (w *WebhookServer) DeleteMessage(messageID string) error {
    return w.restClient.DeleteMessage(messageID)
}
```

### C. RESTClient 提取

```go
// internal/feishu/rest.go
package feishu

import (
    "context"
    "encoding/json"
    "fmt"

    lark "github.com/larksuite/oapi-sdk-go/v3"
    larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
    larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type RESTClient struct {
    client *lark.Client
}

func NewRESTClient(appID, appSecret string) *RESTClient {
    client := lark.NewClient(appID, appSecret,
        lark.WithLogLevel(larkcore.LogLevelInfo),
    )

    return &RESTClient{client: client}
}

func (r *RESTClient) SendMessage(chatID, text string) (string, error) {
    req := larkim.NewCreateMessageReqBuilder().
        ReceiveIdType("chat_id").
        Body(larkim.NewCreateMessageReqBodyBuilder().
            ReceiveId(chatID).
            MsgType("text").
            Content(fmt.Sprintf(`{"text":"%s"}`, escapeJSON(text))).
            Build()).
        Build()

    resp, err := r.client.Im.Message.Create(context.Background(), req)
    if err != nil {
        return "", fmt.Errorf("failed to send message: %w", err)
    }

    if !resp.Success() {
        return "", fmt.Errorf("failed to send message: %s", resp.Msg)
    }

    messageID := ""
    if resp.Data != nil && resp.Data.MessageId != nil {
        messageID = *resp.Data.MessageId
    }

    return messageID, nil
}

func (r *RESTClient) UpdateMessage(messageID, text string) error {
    req := larkim.NewUpdateMessageReqBuilder().
        MessageId(messageID).
        Body(larkim.NewUpdateMessageReqBodyBuilder().
            MsgType("text").
            Content(fmt.Sprintf(`{"text":"%s"}`, escapeJSON(text))).
            Build()).
        Build()

    resp, err := r.client.Im.Message.Update(context.Background(), req)
    if err != nil {
        return fmt.Errorf("failed to update message: %w", err)
    }

    if !resp.Success() {
        return fmt.Errorf("failed to update message: %s", resp.Msg)
    }

    return nil
}

func (r *RESTClient) DeleteMessage(messageID string) error {
    req := larkim.NewDeleteMessageReqBuilder().
        MessageId(messageID).
        Build()

    resp, err := r.client.Im.Message.Delete(context.Background(), req)
    if err != nil {
        return fmt.Errorf("failed to delete message: %w", err)
    }

    if !resp.Success() {
        return fmt.Errorf("failed to delete message: %s", resp.Msg)
    }

    return nil
}

func escapeJSON(s string) string {
    b, _ := json.Marshal(s)
    if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
        return string(b[1 : len(b)-1])
    }
    return string(b)
}
```
