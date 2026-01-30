# ClawdBot Bridge

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?flat&logo=go)](https://go.dev/)

连接飞书等国内 IM 平台与 ClawdBot AI Agent 的桥接服务。

## 前置要求

- ClawdBot Gateway 正在本地运行（默认端口 18789，配置在 `~/.clawdbot/clawdbot.json` 或者 `~/.openclaw/openclaw.json`）
- 飞书企业自建应用的 App ID 和 App Secret

## 安装

#### 预编译二进制

**Linux (amd64)**
```bash
curl -sLO https://github.com/wy51ai/moltbotCNAPP/releases/latest/download/clawdbot-bridge-linux-amd64 && mv clawdbot-bridge-linux-amd64 clawdbot-bridge && chmod +x clawdbot-bridge
```

**Linux (arm64)**
```bash
curl -sLO https://github.com/wy51ai/moltbotCNAPP/releases/latest/download/clawdbot-bridge-linux-arm64 && mv clawdbot-bridge-linux-arm64 clawdbot-bridge && chmod +x clawdbot-bridge
```

**macOS (arm64 / Apple Silicon)**
```bash
curl -sLO https://github.com/wy51ai/moltbotCNAPP/releases/latest/download/clawdbot-bridge-darwin-arm64 && mv clawdbot-bridge-darwin-arm64 clawdbot-bridge && chmod +x clawdbot-bridge
```

**macOS (amd64 / Intel)**
```bash
curl -sLO https://github.com/wy51ai/moltbotCNAPP/releases/latest/download/clawdbot-bridge-darwin-amd64 && mv clawdbot-bridge-darwin-amd64 clawdbot-bridge && chmod +x clawdbot-bridge
```

**Windows (amd64)**
```powershell
Invoke-WebRequest -Uri https://github.com/wy51ai/moltbotCNAPP/releases/latest/download/clawdbot-bridge-windows-amd64.exe -OutFile clawdbot-bridge.exe
```

也可以直接从 [Releases](https://github.com/wy51ai/moltbotCNAPP/releases) 页面手动下载。

#### 从源码编译

```bash
git clone https://github.com/wy51ai/moltbotCNAPP.git
cd moltbotCNAPP
go build -o clawdbot-bridge ./cmd/bridge/
```

## 使用

### 首次启动

传入飞书凭据，会自动保存到 `~/.clawdbot/bridge.json`：

```bash
./clawdbot-bridge start fs_app_id=cli_xxx fs_app_secret=yyy
```

### 日常管理

凭据保存后，直接使用：

```bash
./clawdbot-bridge start     # 后台启动
./clawdbot-bridge stop      # 停止
./clawdbot-bridge restart   # 重启
./clawdbot-bridge status    # 查看状态
./clawdbot-bridge run       # 前台运行（方便调试）
```

### 可选参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `fs_app_id` | 飞书 App ID | — |
| `fs_app_secret` | 飞书 App Secret | — |
| `agent_id` | ClawdBot Agent ID | `main` |
| `thinking_ms` | 显示"思考中"延迟（毫秒），0 为禁用 | `0` |

### 查看日志

```bash
tail -f ~/.clawdbot/bridge.log
```

## Webhook 模式

### 适用场景

Webhook 模式适用于需要公网访问的生产环境，相比 WebSocket 模式具有以下优势：

- **更高可靠性**：飞书事件订阅机制自动重试，不依赖长连接稳定性
- **更易扩展**：支持多实例水平扩展（通过负载均衡）
- **更低资源消耗**：无需维护长连接，空闲时几乎零开销

**注意**：Webhook 模式需要公网可访问的 URL（生产环境通过域名，开发环境可使用 ngrok）

### 配置字段

| 字段 | 说明 | 默认值 | 必填 |
|------|------|--------|------|
| `mode` | 运行模式 | `websocket` | 否 |
| `port` | HTTP 监听端口 | `8080` | 否 |
| `path` | Webhook 路径 | `/webhook/event` | 否 |
| `verification_token` | 飞书 Verification Token | — | **是** |
| `encrypt_key` | 飞书 Encrypt Key | — | **是** |
| `workers` | 并发处理 Worker 数量 | `10` | 否 |
| `queue_size` | 事件队列大小 | `100` | 否 |

### 完整配置示例

在 `~/.clawdbot/bridge.json` 中配置：

```json
{
  "mode": "webhook",
  "port": 8080,
  "path": "/webhook/event",
  "verification_token": "your_verification_token_from_feishu",
  "encrypt_key": "your_encrypt_key_from_feishu",
  "workers": 10,
  "queue_size": 100,
  "fs_app_id": "cli_xxx",
  "fs_app_secret": "yyy",
  "agent_id": "main",
  "thinking_ms": 0
}
```

### 启动 Webhook 模式

```bash
# 方式 1: 通过 CLI 参数（会自动保存到 bridge.json）
./clawdbot-bridge start mode=webhook verification_token=xxx encrypt_key=yyy

# 方式 2: 直接修改 ~/.clawdbot/bridge.json，然后启动
./clawdbot-bridge start
```

启动后服务监听在 `http://0.0.0.0:8080/webhook/event`。

### 飞书后台配置指南

#### 步骤 1: 获取应用凭据

1. 登录 [飞书开放平台](https://open.feishu.cn/app)
2. 创建企业自建应用（或打开已有应用）
3. 在「凭证与基础信息」页面获取：
   - **App ID**（例如 `cli_xxx`）
   - **App Secret**（点击查看完整值）

#### 步骤 2: 配置事件订阅

1. 进入「事件订阅」页面
2. 配置请求地址：
   - **请求地址 URL**：`https://your-domain.com/webhook/event`（开发环境可使用 ngrok 生成的 URL，见下方 ngrok 指南）
   - **Verification Token**：复制该值，配置到 `bridge.json` 的 `verification_token` 字段
   - **Encrypt Key**：复制该值，配置到 `bridge.json` 的 `encrypt_key` 字段

3. 点击「验证」按钮，确保返回"验证成功"

**注意**：必须先启动 bridge 服务，飞书才能验证 Webhook URL。

#### 步骤 3: 添加事件订阅

在「添加事件」中搜索并添加：

- **im.message.receive_v1** - 接收消息（必须）

#### 步骤 4: 配置权限

在「权限管理」页面申请以下权限：

- **im:message** - 获取与发送单聊、群组消息
- **im:message.group_at_msg** - 获取群组中所有消息（用于 @机器人）
- **im:message.group_at_msg:readonly** - 只读获取用户发给机器人的单聊消息
- **im:message:send_as_bot** - 以应用身份发消息

#### 步骤 5: 发布应用

1. 在「版本管理与发布」页面创建版本
2. 提交审核（企业自建应用通常秒过）
3. 发布到企业

**验证配置**：在飞书中搜索你的应用名称，发送消息，观察 bridge 日志是否收到事件。

### ngrok 本地验收指南

开发环境可使用 ngrok 将本地服务暴露到公网，用于飞书 Webhook 验证。

#### 安装 ngrok

```bash
# macOS
brew install ngrok

# Linux
curl -sLO https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-amd64.tgz
tar -xvf ngrok-v3-stable-linux-amd64.tgz
sudo mv ngrok /usr/local/bin/

# Windows
# 从 https://ngrok.com/download 下载
```

#### 使用 ngrok

```bash
# 1. 启动 bridge (Webhook 模式)
./clawdbot-bridge run mode=webhook verification_token=xxx encrypt_key=yyy

# 2. 在另一个终端启动 ngrok
ngrok http 8080
```

ngrok 会输出类似：`https://abc123.ngrok.io -> http://localhost:8080`

#### 验收步骤

1. **配置飞书 Webhook URL**：在飞书开放平台「事件订阅」中填写 `https://abc123.ngrok.io/webhook/event`
2. **点击验证**：飞书会发送 challenge 请求，观察终端日志是否返回"验证成功"
3. **发送测试消息**：在飞书中 @机器人 发送消息
4. **检查日志**：观察 bridge 日志是否收到事件（`event_id=...`）
5. **验证响应**：确认机器人在飞书中正常回复

### 常见问题 FAQ

#### Q1: 启动报错 "verification_token is required in webhook mode"

**原因**：Webhook 模式缺少必填的安全凭据。

**解决方案**：

1. 确认 `~/.clawdbot/bridge.json` 包含 `verification_token` 和 `encrypt_key` 字段
2. 值需从飞书开放平台「事件订阅」页面复制
3. 重启服务：`./clawdbot-bridge restart`

#### Q2: 飞书验证 Webhook URL 失败

**可能原因**：

- Bridge 服务未启动或端口未开放
- ngrok 未运行（开发环境）
- URL 路径错误（必须是 `/webhook/event`）
- `verification_token` 或 `encrypt_key` 配置错误

**排查步骤**：

```bash
# 1. 确认服务运行中
./clawdbot-bridge status

# 2. 检查日志
tail -f ~/.clawdbot/bridge.log

# 3. 测试本地访问
curl -X POST http://localhost:8080/webhook/event \
  -H "Content-Type: application/json" \
  -d '{"challenge":"test"}'
# 应返回: {"challenge":"test"}

# 4. 检查 ngrok 转发
curl https://your-ngrok-url.ngrok.io/webhook/event
```

#### Q3: 收到消息但没有响应

**排查清单**：

1. **检查队列是否满**：查看 `/metrics` 中的 `feishu_worker_pool_queue_depth`，若等于 `queue_size` 则队列已满
2. **检查 Worker 是否 panic**：搜索日志 `worker panic`
3. **检查 ClawdBot Gateway 可达性**：`curl http://localhost:18789/health`
4. **检查权限**：确认飞书应用有 `im:message:send_as_bot` 权限

#### Q4: 为什么使用 event_id 而不是 message_id 去重？

**原因**：飞书事件订阅机制的设计决定。

- **event_id**：飞书事件投递的全局唯一标识，用于标识"这次投递"
- **message_id**：消息内容的标识，同一条消息可能因重试产生多个 event_id

**场景示例**：

1. 用户发送消息 A（message_id=msg_001）
2. 飞书首次投递事件（event_id=evt_001）
3. Bridge 处理超时，飞书重试投递（event_id=evt_002，但 message_id 仍是 msg_001）

若使用 message_id 去重，重试时会被误判为重复事件而丢弃，导致消息丢失。

**结论**：使用 event_id 确保"每次投递都处理一次"，即使处理失败也能通过重试保证可靠性。

### 监控指标

Webhook 模式在 `http://localhost:8080/metrics` 暴露 Prometheus 指标：

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `feishu_webhook_requests_total` | Counter | Webhook 请求总数（按 status 和 path 标签） |
| `feishu_webhook_request_duration_seconds` | Histogram | HTTP 请求处理时间 |
| `feishu_message_processing_duration_seconds` | Histogram | 消息处理执行时间（handler 层） |
| `feishu_webhook_signature_failures_total` | Counter | 签名验证失败次数 |
| `feishu_worker_pool_queue_depth` | Gauge | 当前队列深度 |
| `feishu_worker_pool_queue_capacity` | Gauge | 队列容量 |

**使用示例**：

```bash
# 查看队列深度
curl -s http://localhost:8080/metrics | grep feishu_worker_pool_queue_depth

# 查看签名失败次数
curl -s http://localhost:8080/metrics | grep feishu_webhook_signature_failures_total
```

## 开发

```bash
# 前台运行（日志直接输出到终端）
./clawdbot-bridge run

# 编译所有平台
./scripts/build.sh
```

## 贡献

欢迎提交 Issue 和 Pull Request！。

## 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件
