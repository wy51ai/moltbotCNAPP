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
