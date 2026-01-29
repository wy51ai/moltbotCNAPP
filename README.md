# ClawdBot Bridge

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?flat&logo=go)](https://go.dev/)

连接飞书等国内 IM 平台与 ClawdBot AI Agent 的桥接服务。

## 前置要求

- ClawdBot Gateway 正在本地运行（默认端口 18789，配置在 `~/.clawdbot/clawdbot.json`）
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

## 开发

```bash
# 前台运行（日志直接输出到终端）
./clawdbot-bridge run

# 编译所有平台
./scripts/build.sh
```

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件
