---
phase: 04-e2e-testing-docs
plan: "02"
subsystem: observability
tags:
  - prometheus
  - metrics
  - logging
  - webhook
dependency-graph:
  requires:
    - phase: 02
      plan: "03"
      provides: Prometheus metrics基础设施
    - phase: 04
      plan: "01"
      provides: Webhook receiver tests
  provides:
    - handler-execution-duration-metric
    - signature-failure-counter
    - structured-key-value-logging
  affects:
    - phase: 04
      plan: "03+"
      note: Metrics and logging patterns for E2E testing
tech-stack:
  added: []
  patterns:
    - handler-execution-time-measurement
    - structured-key-value-logging
    - metrics-observability-testing
key-files:
  created: []
  modified:
    - internal/feishu/webhook_receiver.go
    - internal/feishu/webhook_receiver_test.go
decisions:
  - what: Handler 执行耗时与 HTTP 请求耗时分离
    why: Codex 评审要求区分两种耗时口径 (HTTP 入站->返回 vs handler 执行)
    chosen: feishu_message_processing_duration_seconds 专门测量 handler 执行时间
    alternatives: []
  - what: 日志格式统一为 key=value
    why: 便于日志解析和查询，提升可观测性
    chosen: "event=duplicate/queue_full/enqueued/processed event_id=xxx message_id=xxx duration_ms=xxx"
    alternatives: []
metrics:
  tasks: 3
  commits: 3
  duration: ~3 minutes
  completed: 2026-01-29
---

# Phase 04 Plan 02: Webhook Receiver Observability

**One-liner:** Handler 执行耗时直方图和签名失败计数器，增强 key=value 结构化日志

## What Was Built

为 Webhook 接收器新增可观测性能力，区分 HTTP 请求耗时和 handler 执行耗时：

1. **Handler 执行耗时指标** - feishu_message_processing_duration_seconds
   - Prometheus Histogram 类型
   - 使用 DefaultBuckets (0.005s ~ 10s)
   - 在 handler 闭包中测量 time.Since(start)
   - 只测量 handler 执行时间，不包括入队延迟

2. **签名验证失败计数器** - feishu_webhook_signature_failures_total
   - Prometheus Counter 类型
   - 在 webhookHandler 检测 "signature verification failed" 时递增
   - 帮助排查签名配置问题

3. **结构化日志增强**
   - event=duplicate: 重复事件被去重
   - event=queue_full: 队列满拒绝
   - event=enqueued: 成功入队
   - event=processed: handler 执行完成 (含 duration_ms)
   - 所有日志包含 event_id, message_id 字段

## Technical Details

### Metrics Implementation

**指标定义和注册:**
```go
messageProcessingDuration = prometheus.NewHistogram(
    prometheus.HistogramOpts{
        Name:    "feishu_message_processing_duration_seconds",
        Help:    "Histogram of message handler execution duration",
        Buckets: prometheus.DefBuckets,
    },
)
signatureFailuresTotal = prometheus.NewCounter(
    prometheus.CounterOpts{
        Name: "feishu_webhook_signature_failures_total",
        Help: "Total number of signature verification failures",
    },
)
```

**Handler 执行耗时测量:**
```go
job := Job{
    EventID: eventID,
    Handler: func() error {
        start := time.Now()
        err := wr.handler(msg)
        duration := time.Since(start)
        messageProcessingDuration.Observe(duration.Seconds())
        log.Printf("[Webhook] event=processed event_id=%s message_id=%s duration_ms=%d",
            eventID, messageID, duration.Milliseconds())
        return err
    },
}
```

**签名失败计数:**
```go
if contains(bodyStr, "signature verification failed") {
    signatureFailuresTotal.Inc()
    log.Printf("[Webhook] Signature verification failed from %s", r.RemoteAddr)
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
```

### Logging Format

**Before:**
```
[Webhook] Duplicate event ignored: evt_xxx
[Webhook] Queue full, event evt_xxx will be retried
```

**After:**
```
[Webhook] event=duplicate event_id=evt_xxx
[Webhook] event=queue_full event_id=evt_xxx
[Webhook] event=enqueued event_id=evt_xxx message_id=msg_xxx
[Webhook] event=processed event_id=evt_xxx message_id=msg_xxx duration_ms=45
```

### Observability Tests

**TestWebhookReceiver_Observability:**
1. **metrics_registered** - 验证指标变量不为 nil
2. **processing_duration_observed** - 验证 handler 执行时记录耗时无 panic
   - 使用 atomic.Int32 追踪 handler 调用
   - 等待异步处理完成
   - 验证日志输出包含正确字段

**Test Output:**
```
[Webhook] event=enqueued event_id=test_event_observability message_id=msg_test_123
[Webhook] event=processed event_id=test_event_observability message_id=msg_test_123 duration_ms=11
```

## Commits

| Hash    | Message                                                              |
|---------|----------------------------------------------------------------------|
| 2f0b0f2 | feat(04-02): add handler execution and signature failure metrics     |
| ae41d1e | feat(04-02): enhance logging with key=value format                   |
| ec53f72 | test(04-02): add observability regression tests                      |

## Deviations from Plan

None - plan executed exactly as written.

## Verification

```bash
# Compilation passed
go build ./...

# All tests passed
go test ./internal/feishu/...
ok  	github.com/wy51ai/moltbotCNAPP/internal/feishu	0.209s

# Observability test output shows structured logging
=== RUN   TestWebhookReceiver_Observability/processing_duration_observed
[Webhook] event=enqueued event_id=test_event_observability message_id=msg_test_123
[Webhook] event=processed event_id=test_event_observability message_id=msg_test_123 duration_ms=11
--- PASS: TestWebhookReceiver_Observability/processing_duration_observed (0.05s)
```

## Next Phase Readiness

**Blockers:** None

**Concerns:** None

**Recommended Next Steps:**
1. Phase 04 Plan 03: E2E integration testing
2. Document metrics查询示例 (Prometheus/Grafana)

## Lessons Learned

### What Worked Well
- Handler 闭包模式记录耗时简洁清晰
- key=value 日志格式便于解析和搜索
- Observability 测试验证指标记录无 panic

### Production Usage

**Metrics available at /metrics:**
```
# HELP feishu_message_processing_duration_seconds Histogram of message handler execution duration
# TYPE feishu_message_processing_duration_seconds histogram
feishu_message_processing_duration_seconds_bucket{le="0.005"} 0
...

# HELP feishu_webhook_signature_failures_total Total number of signature verification failures
# TYPE feishu_webhook_signature_failures_total counter
feishu_webhook_signature_failures_total 0
```

**Log queries:**
```bash
# 查看所有处理事件耗时
grep 'event=processed' app.log | awk '{print $NF}'

# 查看队列满拒绝情况
grep 'event=queue_full' app.log

# 查看重复事件去重
grep 'event=duplicate' app.log
```

### Tools Used
- Prometheus client_golang
- time.Since() for duration measurement
- atomic.Int32 for async handler verification in tests
