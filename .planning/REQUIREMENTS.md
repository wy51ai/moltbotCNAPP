# Milestone v1.1 Requirements

**Milestone:** 飞书 Webhook 支持
**Created:** 2026-01-29

## Table Stakes（必须实现）

| REQ-ID | 功能 | 说明 | 来源 |
|--------|------|------|------|
| REQ-01 | URL 验证 (Challenge) | 飞书首次配置时发送 challenge 请求，必须正确响应 | WEBHOOK-03 |
| REQ-02 | HTTP POST 处理 | 接收飞书 webhook 回调，解析消息事件 payload | WEBHOOK-01, WEBHOOK-02 |
| REQ-03 | 3 秒内返回 200 | 必须立即返回 HTTP 200，异步处理消息 | 飞书要求 |
| REQ-04 | 配置模式切换 | 通过 `mode: "websocket" \| "webhook"` 选择模式 | CONFIG-01 |
| REQ-05 | HTTP 端口配置 | `webhook.port` 配置项，避免端口冲突 | 用户体验 |
| REQ-06 | Webhook 路径配置 | `webhook.path` 配置项，默认 `/webhook/feishu` | 用户体验 |
| REQ-07 | 签名验证 | 验证请求来源合法性，防止伪造请求 | 安全要求 |

## Differentiators（增强体验）

| REQ-ID | 功能 | 说明 | 来源 |
|--------|------|------|------|
| REQ-08 | 消息解密 | 支持飞书加密推送模式 | SDK 支持 |
| REQ-09 | 健康检查端点 | `/health` 方便监控服务状态 | 运维需求 |
| REQ-10 | 优雅关闭 | HTTP 服务器平滑停止，等待处理中的请求 | 稳定性 |

## Anti-Features（不做）

| 功能 | 原因 |
|------|------|
| 云函数适配 | 当前只支持本地 HTTP 服务器，用户自行配置网络穿透 |
| 多 URL 支持 | 一个实例一个 webhook 地址 |
| 双模式同时运行 | 简化架构，一个实例只运行一种模式 |
| 事件回放/存储 | 超出 bridge 职责，依赖飞书重试机制 |

## Mapping to PROJECT.md Requirements

| PROJECT.md | REQUIREMENTS.md |
|------------|-----------------|
| WEBHOOK-01 | REQ-02 |
| WEBHOOK-02 | REQ-02 |
| WEBHOOK-03 | REQ-01 |
| CONFIG-01 | REQ-04, REQ-05, REQ-06 |

---
*Requirements defined: 2026-01-29*
