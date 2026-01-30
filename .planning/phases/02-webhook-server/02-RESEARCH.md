# Phase 2: Webhook Server (含安全) - Research

**Researched:** 2026-01-29
**Domain:** Go HTTP server for Feishu webhook event processing with security
**Confidence:** HIGH

## Summary

本次研究聚焦于实现安全的 HTTP webhook 服务器，用于接收和处理飞书事件回调。核心技术栈为 Go 标准库 `net/http` + 飞书官方 SDK `larksuite/oapi-sdk-go v3.5.3`。

标准做法是使用 SDK 内置的 `dispatcher.NewEventDispatcher` 处理所有安全逻辑（challenge 验证、签名验证、消息解密），而不是手写安全代码。并发控制采用 worker pool + buffered channel 模式，避免简单的 semaphore 方案。HTTP 服务器必须配置 ReadTimeout、WriteTimeout、IdleTimeout 三个超时参数，并使用 `http.MaxBytesReader` 而非 `io.LimitReader` 限制 body 大小。

飞书 webhook 要求在收到事件后立即返回 HTTP 200，超时会触发重试机制（具体超时时间未在官方文档中明确，社区讨论提到 3-10 秒），因此必须采用异步处理模式：收到请求后立刻入队并返回 200，由 worker pool 异步处理业务逻辑。

**Primary recommendation:** 使用 SDK 的 `dispatcher.NewEventDispatcher` + `httpserverext.NewEventHandlerFunc`，配合自定义 worker pool 实现异步处理，确保 3 秒内返回响应。

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http | Go 1.21 stdlib | HTTP server 基础 | Go 官方标准库，无需第三方依赖 |
| github.com/larksuite/oapi-sdk-go/v3 | v3.5.3 | 飞书事件处理 | 飞书官方 SDK，内置签名验证、加密解密 |
| github.com/larksuite/oapi-sdk-go/v3/event/dispatcher | v3.5.3 | 事件分发器 | SDK 核心组件，处理 challenge、签名、解密 |
| github.com/larksuite/oapi-sdk-go/v3/core/httpserverext | v3.5.3 | HTTP handler 适配器 | 将 dispatcher 转换为 http.Handler |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/prometheus/client_golang/prometheus | latest | Metrics 暴露 | Phase 2 需要 `/metrics` 端点（根据 CONTEXT.md） |
| github.com/prometheus/client_golang/prometheus/promhttp | latest | Prometheus HTTP handler | 提供 `/metrics` 端点 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| net/http | gin/echo | 标准库足够，引入框架增加依赖且无显著收益 |
| SDK dispatcher | 手写签名验证 | SDK 已验证过安全性，手写容易出错 |
| buffered channel | github.com/alitto/pond | 简单场景下自己实现更轻量，第三方库适合复杂需求 |

**安装:**
```bash
go get github.com/larksuite/oapi-sdk-go/v3@v3.5.3
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

## Architecture Patterns

### Recommended Project Structure
```
internal/feishu/
├── receiver.go              # FeishuReceiver 接口定义
├── webhook_receiver.go      # HTTP webhook 实现
├── sender.go                # FeishuSender（已存在）
└── message.go               # Message 结构体（已存在）

internal/metrics/
└── metrics.go               # Prometheus metrics 定义
```

### Pattern 1: SDK Dispatcher + HTTP Handler
**What:** 使用 SDK 的 EventDispatcher 处理所有安全逻辑，通过 httpserverext 转换为 http.Handler
**When to use:** 所有飞书 webhook 场景
**Example:**
```go
// Source: https://github.com/larksuite/oapi-sdk-go (v3.5.3)
package main

import (
    "context"
    "fmt"
    "net/http"
    larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
    larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
    "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
    "github.com/larksuite/oapi-sdk-go/v3/core/httpserverext"
    larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func main() {
    // 创建 dispatcher（自动处理 challenge、签名验证、解密）
    handler := dispatcher.NewEventDispatcher(
        "verification_token",  // 从飞书控制台获取
        "event_encrypt_key",   // 从飞书控制台获取（启用加密时）
    ).OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
        fmt.Println(larkcore.Prettify(event))
        return nil
    })

    // 转换为 http.Handler
    http.HandleFunc("/webhook/event",
        httpserverext.NewEventHandlerFunc(handler,
            larkevent.WithLogLevel(larkcore.LogLevelInfo)))

    // 启动服务器（注意：生产环境需要添加超时配置）
    http.ListenAndServe(":8080", nil)
}
```

### Pattern 2: Worker Pool + Buffered Channel
**What:** 固定数量 worker + 有界队列，队列满时拒绝请求
**When to use:** 需要控制并发数和防止内存溢出时
**Example:**
```go
// Source: https://gobyexample.com/worker-pools
package main

type WorkerPool struct {
    workers   int
    jobQueue  chan Job
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
}

type Job struct {
    EventID string
    Data    interface{}
}

func NewWorkerPool(workers, queueSize int) *WorkerPool {
    ctx, cancel := context.WithCancel(context.Background())
    return &WorkerPool{
        workers:  workers,
        jobQueue: make(chan Job, queueSize),
        ctx:      ctx,
        cancel:   cancel,
    }
}

func (p *WorkerPool) Start(handler func(Job) error) {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go func(workerID int) {
            defer p.wg.Done()
            for {
                select {
                case job := <-p.jobQueue:
                    // Panic recovery（生产环境必须）
                    func() {
                        defer func() {
                            if r := recover(); r != nil {
                                log.Printf("worker %d panic: %v", workerID, r)
                            }
                        }()
                        if err := handler(job); err != nil {
                            log.Printf("worker %d error: %v", workerID, err)
                        }
                    }()
                case <-p.ctx.Done():
                    return
                }
            }
        }(i)
    }
}

func (p *WorkerPool) Submit(job Job) error {
    select {
    case p.jobQueue <- job:
        return nil
    default:
        return errors.New("queue full")
    }
}

func (p *WorkerPool) Shutdown(timeout time.Duration) error {
    close(p.jobQueue)              // 停止接收新任务
    done := make(chan struct{})
    go func() {
        p.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        p.cancel()  // 强制取消
        return errors.New("shutdown timeout")
    }
}
```

### Pattern 3: Production HTTP Server Configuration
**What:** 配置所有必要的超时参数和安全限制
**When to use:** 所有生产环境 HTTP 服务器
**Example:**
```go
// Source: https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
// Source: https://adam-p.ca/blog/2022/01/golang-http-server-timeouts/
package main

import (
    "net/http"
    "time"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/webhook/event", webhookHandler)

    srv := &http.Server{
        Addr:              ":8080",
        Handler:           mux,
        ReadTimeout:       10 * time.Second,  // 读取请求头+body 的最大时间
        WriteTimeout:      10 * time.Second,  // 写响应的最大时间
        IdleTimeout:       60 * time.Second,  // keepalive 空闲超时
        ReadHeaderTimeout: 5 * time.Second,   // 读取请求头的最大时间
    }

    if err := srv.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
    // 限制 body 大小（使用 MaxBytesReader 而非 io.LimitReader）
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
    defer r.Body.Close()

    // 仅允许 POST
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // 处理逻辑...
}
```

### Pattern 4: Graceful Shutdown
**What:** 使用 signal.NotifyContext + Server.Shutdown 优雅关闭
**When to use:** 所有生产环境 HTTP 服务器
**Example:**
```go
// Source: https://dev.to/mokiat/proper-http-shutdown-in-go-3fji
// Source: https://www.rudderstack.com/blog/implementing-graceful-shutdown-in-go/
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    srv := &http.Server{
        Addr:         ":8080",
        Handler:      http.DefaultServeMux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // 使用 context 捕获 SIGINT/SIGTERM
    ctx, stop := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)
    defer stop()

    // 启动服务器（在 goroutine 中）
    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("server error: %v", err)
        }
    }()

    log.Println("server started")

    // 等待信号
    <-ctx.Done()
    log.Println("shutting down gracefully...")

    // 优雅关闭（设置超时）
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        log.Fatalf("shutdown error: %v", err)
    }

    log.Println("server stopped")
}
```

### Pattern 5: Prometheus Metrics Endpoint
**What:** 暴露 `/metrics` 端点用于 Prometheus 抓取
**When to use:** 需要监控时（Phase 2 CONTEXT.md 要求）
**Example:**
```go
// Source: https://prometheus.io/docs/guides/go-application/
// Source: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp
package main

import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    // 定义 metrics
    webhookRequests = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "webhook_requests_total",
            Help: "Total number of webhook requests",
        },
        []string{"status"},
    )

    webhookDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "webhook_request_duration_seconds",
            Help:    "Webhook request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"endpoint"},
    )

    queueDepth = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "worker_queue_depth",
            Help: "Current depth of worker queue",
        },
    )
)

func init() {
    // 注册 metrics
    prometheus.MustRegister(webhookRequests)
    prometheus.MustRegister(webhookDuration)
    prometheus.MustRegister(queueDepth)
}

func main() {
    // 暴露 /metrics 端点
    http.Handle("/metrics", promhttp.Handler())

    // 其他路由...
    http.ListenAndServe(":8080", nil)
}
```

### Anti-Patterns to Avoid
- **使用 http.ListenAndServe 不配置超时**: 会导致连接泄漏，生产环境必须使用 http.Server 并配置超时
- **使用 io.LimitReader 限制 body**: 应该用 http.MaxBytesReader，后者在超出限制时返回明确错误并关闭连接
- **在 webhook handler 中同步处理耗时逻辑**: 会超过飞书的超时限制导致重试，必须异步处理
- **忘记 worker panic recovery**: worker panic 会导致整个程序崩溃，必须在每个 worker 中 defer recover
- **队列满时阻塞等待**: 会导致请求超时，应该立即返回 503 触发飞书重试

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 飞书签名验证 | 自己实现 HMAC-SHA256 | SDK `dispatcher.NewEventDispatcher` | SDK 已包含完整实现，自己写容易出安全漏洞 |
| 消息解密 | 自己实现 AES-CBC | SDK 内置解密 | 加密细节复杂（padding、IV），SDK 已处理 |
| Challenge 验证 | 手写 url_verification 处理 | SDK 自动处理 | dispatcher 会自动识别并响应 challenge |
| HTTP body 限制 | 使用 io.LimitReader | `http.MaxBytesReader` | MaxBytesReader 在超限时返回明确错误并关闭连接 |
| Worker pool | 从头实现 | 基于 channel 的简单实现 | 简单场景下 buffered channel 足够，复杂需求考虑 github.com/alitto/pond |
| 事件去重 | 自己实现 LRU cache | 简单 map + 时间窗口 | 简单场景下 map[string]time.Time 足够 |

**Key insight:** 飞书 SDK 已经过充分验证，安全相关代码不应该自己实现。HTTP 层面的安全限制应该使用标准库的专用 API（如 MaxBytesReader）而非通用工具（如 LimitReader）。

## Common Pitfalls

### Pitfall 1: 使用 io.LimitReader 而非 http.MaxBytesReader
**What goes wrong:** 当 body 超过限制时，LimitReader 只返回 EOF，无法区分正常结束和截断，导致处理不完整的 JSON payload 时报错不清晰
**Why it happens:** LimitReader 是通用 Reader 限制工具，不了解 HTTP 语义
**How to avoid:** 始终使用 `http.MaxBytesReader(w, r.Body, maxBytes)`，它会在超限时返回 `*http.MaxBytesError` 并告知 ResponseWriter 关闭连接
**Warning signs:** 日志中出现 "unexpected EOF" 或 JSON 解析错误，但没有明确的 "request too large" 错误
**Source:** https://groups.google.com/g/golang-dev/c/ZxxcGhCmIe8

### Pitfall 2: 忘记配置 ReadHeaderTimeout
**What goes wrong:** 恶意客户端可以慢速发送请求头（Slowloris 攻击），耗尽服务器文件描述符
**Why it happens:** ReadTimeout 包含整个请求处理时间，长时间运行的 handler 需要更大的 ReadTimeout，导致无法有效防护慢速攻击
**How to avoid:** 独立配置 `ReadHeaderTimeout: 5 * time.Second`，它仅限制读取请求头的时间，不受 handler 执行时间影响
**Warning signs:** 服务器在负载不高时出现 "too many open files" 错误
**Source:** https://adam-p.ca/blog/2022/01/golang-http-server-timeouts/

### Pitfall 3: 在 Webhook Handler 中同步处理耗时任务
**What goes wrong:** 飞书 webhook 有超时限制（社区讨论提到 3-10 秒），处理超时会触发重试，导致重复处理
**Why it happens:** 开发者习惯同步处理请求，没有意识到 webhook 的超时要求
**How to avoid:** 收到 webhook 后立即入队并返回 200，由 worker pool 异步处理。队列满时返回 503 触发飞书重试
**Warning signs:** 日志显示同一个 event_id 被处理多次
**Source:** https://www.feishu.cn/content/49fq0rvm (飞书社区讨论)

### Pitfall 4: Worker Panic 导致程序崩溃
**What goes wrong:** Worker goroutine 中的 panic 会导致整个程序崩溃，而不仅仅是当前任务失败
**Why it happens:** Go 的 goroutine panic 会向上传播，不像某些语言的线程异常是隔离的
**How to avoid:** 每个 worker 的任务处理必须包装在 `defer recover()` 中，捕获 panic 后记录日志并继续处理下一个任务
**Warning signs:** 生产环境偶发性整体服务崩溃，重启后恢复
**Source:** https://www.dolthub.com/blog/2026-01-09-golang-panic-recovery/

### Pitfall 5: 队列满时阻塞等待
**What goes wrong:** 使用阻塞的 `jobQueue <- job` 会导致 HTTP handler 长时间阻塞，超过飞书的超时限制
**Why it happens:** 直接使用 channel send 的默认行为是阻塞
**How to avoid:** 使用 `select` + `default` 非阻塞发送，队列满时立即返回 503 状态码
**Warning signs:** 高负载时大量请求超时，但 worker 并未满载
**Source:** Go worker pool 最佳实践

### Pitfall 6: 忘记事件去重
**What goes wrong:** 飞书重试机制可能导致同一事件被多次推送，如果不去重会重复处理消息
**Why it happens:** 网络故障或超时导致飞书认为推送失败，触发重试
**How to avoid:** 使用 event_id 或 message_id 做去重，维护最近 N 个 ID 的内存缓存（如最近 1000 个，TTL 10 分钟）
**Warning signs:** 用户报告收到重复回复
**Source:** 飞书开发者社区经验分享

## Code Examples

### Complete Webhook Receiver Implementation Pattern
```go
// Source: 综合多个最佳实践
package feishu

import (
    "context"
    "errors"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"

    larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
    larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
    "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
    "github.com/larksuite/oapi-sdk-go/v3/core/httpserverext"
    larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type WebhookReceiver struct {
    verificationToken string
    encryptKey        string
    handler           MessageHandler
    workerPool        *WorkerPool
    dedupeCache       sync.Map // event_id -> time.Time
}

func NewWebhookReceiver(verificationToken, encryptKey string, handler MessageHandler) *WebhookReceiver {
    wr := &WebhookReceiver{
        verificationToken: verificationToken,
        encryptKey:        encryptKey,
        handler:           handler,
        workerPool:        NewWorkerPool(10, 100), // 10 workers, 100 queue size
    }

    // 启动 worker pool
    wr.workerPool.Start(func(job Job) error {
        // TODO: 调用实际的 handler
        return nil
    })

    // 启动去重缓存清理
    go wr.cleanupDedupeCache()

    return wr
}

func (wr *WebhookReceiver) Start(ctx context.Context) error {
    // 创建 SDK dispatcher
    eventDispatcher := dispatcher.NewEventDispatcher(
        wr.verificationToken,
        wr.encryptKey,
    ).OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
        // 异步处理：入队后立即返回
        eventID := event.EventV2Base.Header.EventId

        // 去重检查
        if _, exists := wr.dedupeCache.LoadOrStore(*eventID, time.Now()); exists {
            log.Printf("duplicate event ignored: %s", *eventID)
            return nil // 重复事件，直接返回成功
        }

        // 提交到队列
        job := Job{EventID: *eventID, Data: event}
        if err := wr.workerPool.Submit(job); err != nil {
            log.Printf("queue full, event %s will be retried by Feishu", *eventID)
            return errors.New("queue full") // SDK 会返回 503
        }

        return nil
    })

    // 配置 HTTP server
    mux := http.NewServeMux()
    mux.HandleFunc("/webhook/event", func(w http.ResponseWriter, r *http.Request) {
        // 限制 body 大小
        r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
        defer r.Body.Close()

        // 仅允许 POST
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }

        // 委托给 SDK handler
        httpHandler := httpserverext.NewEventHandlerFunc(
            eventDispatcher,
            larkevent.WithLogLevel(larkcore.LogLevelInfo),
        )
        httpHandler(w, r)
    })

    srv := &http.Server{
        Addr:              ":8080",
        Handler:           mux,
        ReadTimeout:       10 * time.Second,
        WriteTimeout:      10 * time.Second,
        IdleTimeout:       60 * time.Second,
        ReadHeaderTimeout: 5 * time.Second,
    }

    // 优雅关闭
    go func() {
        <-ctx.Done()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        if err := srv.Shutdown(shutdownCtx); err != nil {
            log.Printf("shutdown error: %v", err)
        }

        // 关闭 worker pool
        wr.workerPool.Shutdown(30 * time.Second)
    }()

    log.Printf("webhook server started on :8080")
    return srv.ListenAndServe()
}

func (wr *WebhookReceiver) cleanupDedupeCache() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        now := time.Now()
        wr.dedupeCache.Range(func(key, value interface{}) bool {
            if ts, ok := value.(time.Time); ok {
                if now.Sub(ts) > 10*time.Minute {
                    wr.dedupeCache.Delete(key)
                }
            }
            return true
        })
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 使用 botframework-go | 使用 oapi-sdk-go/v3 | ~2021 | 旧 SDK 已废弃，v3 是官方维护版本 |
| 手写 AES 解密 | SDK 内置 DecryptEvent | v3.0 | 简化开发，减少安全漏洞 |
| 使用 gin/echo 框架 | 标准库 net/http | 持续 | 简单 webhook 不需要框架，减少依赖 |
| 全局 semaphore 限流 | Worker pool + 有界队列 | 最佳实践演进 | 更好的背压控制和可观测性 |

**Deprecated/outdated:**
- **botframework-go**: 已被 oapi-sdk-go 替代，GitHub 仓库标记为 deprecated
- **oapi-sdk-go v2**: v3 是当前版本，v2 不再推荐使用
- **长连接模式（WebSocket）**: 虽然仍可用，但 webhook 模式更简单且更符合 Phase 2 需求

## Open Questions

1. **飞书 webhook 的确切超时时间**
   - What we know: 社区讨论提到 3-10 秒，官方文档未明确
   - What's unclear: 确切的超时值和重试策略
   - Recommendation: 按照 3 秒设计，确保立即返回 200

2. **事件去重的最佳窗口大小**
   - What we know: 需要去重，可以用 event_id
   - What's unclear: 重试窗口有多大，需要保留多久的历史
   - Recommendation: 保留最近 10 分钟的 event_id（估计值），可配置

3. **Metrics 的详细规格**
   - What we know: CONTEXT.md 要求暴露 `/metrics` 端点（Prometheus 格式）
   - What's unclear: 具体需要哪些 metrics（已知：请求数、延迟、队列深度）
   - Recommendation: 先实现基础 metrics，后续根据需要扩展

## Sources

### Primary (HIGH confidence)
- pkg.go.dev/github.com/larksuite/oapi-sdk-go/v3/event/dispatcher - EventDispatcher 函数签名和用法
- pkg.go.dev/github.com/larksuite/oapi-sdk-go/v3/core - SDK 核心类型和错误处理
- https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/ - Go HTTP 超时完整指南
- https://adam-p.ca/blog/2022/01/golang-http-server-timeouts/ - HTTP server 超时最佳实践
- https://gobyexample.com/worker-pools - Worker pool 模式示例
- https://prometheus.io/docs/guides/go-application/ - Prometheus Go 应用集成指南
- https://groups.google.com/g/golang-dev/c/ZxxcGhCmIe8 - MaxBytesReader 设计讨论

### Secondary (MEDIUM confidence)
- https://dev.to/mokiat/proper-http-shutdown-in-go-3fji - Go HTTP 优雅关闭
- https://www.rudderstack.com/blog/implementing-graceful-shutdown-in-go/ - 优雅关闭实现
- https://www.dolthub.com/blog/2026-01-09-golang-panic-recovery/ - Panic recovery 最佳实践
- https://github.com/larksuite/oapi-sdk-go - 飞书 SDK 官方仓库
- https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp - Prometheus HTTP handler
- https://medium.com/@arjun.devb25/efficient-deduplication-in-go-with-the-new-unique-package-a8fd9e0c79af - Go 1.23 去重新特性

### Tertiary (LOW confidence)
- https://www.feishu.cn/content/49fq0rvm - 飞书 webhook 社区讨论（超时相关）
- https://pypi.org/project/feishu-python-sdk/ - Python SDK 文档（间接参考 challenge 处理）
- https://dev.to/clanic/effortless-nats-message-deduplication-in-go-2ohl - NATS 消息去重（模式参考）

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 飞书官方 SDK + Go 标准库，文档完整
- Architecture: HIGH - SDK 示例代码清晰，HTTP 超时配置有权威来源
- Pitfalls: MEDIUM - 部分来自社区经验（如飞书超时时间），但核心问题（MaxBytesReader、panic recovery）有官方文档支持

**Research date:** 2026-01-29
**Valid until:** 2026-02-28 (30 days) - SDK 和 Go 标准库相对稳定
