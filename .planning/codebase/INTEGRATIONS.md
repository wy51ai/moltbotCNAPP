# External Integrations

**Analysis Date:** 2026-01-29

## APIs & External Services

**Feishu (Lark) - Enterprise Messaging:**
- Service: Feishu/Lark enterprise messaging platform (Chinese domestic alternative to Slack)
- What it's used for: Receiving messages from users via WebSocket, sending replies, updating/deleting messages
- SDK/Client: `github.com/larksuite/oapi-sdk-go/v3` v3.5.3
- Implementation: `internal/feishu/client.go`
- Auth: App ID (`fs_app_id`) and App Secret (`fs_app_secret`) configured in `~/.clawdbot/bridge.json`
- Endpoints:
  - WebSocket for event streaming (P2MessageReceiveV1)
  - REST API for message operations (create, update, delete)
- Message types supported: Text messages in both P2 (direct) and group chats
- Event handling: Receives real-time messages via WebSocket dispatcher

**ClawdBot Gateway - Local AI Agent:**
- Service: Local ClawdBot AI agent gateway (not an external cloud service, runs locally)
- What it's used for: Processing user messages and generating AI responses
- Connection: WebSocket (`ws://127.0.0.1:18789` by default)
- Implementation: `internal/clawdbot/client.go`
- Auth: Token-based authentication configured in `~/.clawdbot/clawdbot.json`
- Protocol: Binary WebSocket with JSON messaging
- Flow:
  1. Connect to gateway via WebSocket
  2. Respond to `connect.challenge` event with authentication
  3. Send `agent` request with message and session key
  4. Receive streaming responses via `agent` events
  5. Wait for `lifecycle.end` event or timeout (5 minutes)
- Session management: Session keys are scoped as `feishu:{chat_id}` to maintain conversation context

## Data Storage

**Databases:**
- None detected - This is a stateless bridge service

**File Storage:**
- Local filesystem only
- Config location: `~/.clawdbot/` directory (home directory)
- Log location: `~/.clawdbot/bridge.log` (when run as daemon)
- PID file: `~/.clawdbot/bridge.pid` (daemon management)

**Caching:**
- In-memory message cache in `internal/bridge/bridge.go`:
  - Purpose: Prevent duplicate message processing
  - TTL: 10 minutes
  - Cleanup: Automatic goroutine-based cleanup runs every 1 minute
  - Used to track seen Feishu message IDs

## Authentication & Identity

**Auth Provider:**
- Custom token-based approach
- Feishu: OAuth2-style App ID + App Secret (managed by Feishu platform)
  - Location: `internal/feishu/client.go` - Used in `lark.NewClient()` initialization
  - Config source: `~/.clawdbot/bridge.json`

- ClawdBot Gateway: Token-based authentication
  - Location: `internal/clawdbot/client.go` - Sent in `ConnectParams.Auth.Token`
  - Config source: `~/.clawdbot/clawdbot.json`

**User Mentions:**
- Feishu message mentions are parsed and extracted but not used for authentication
- Mention format: `@_user_{id}` patterns removed before sending to ClawdBot (`internal/bridge/bridge.go` - `removeMentions()`)

## Monitoring & Observability

**Error Tracking:**
- None detected - No external error tracking service integrated

**Logs:**
- Standard Go `log` package
- Log file: `~/.clawdbot/bridge.log` (when running as daemon via `bridge start`)
- Console output: When running with `bridge run` command
- Log locations in code:
  - `cmd/bridge/main.go` - Process lifecycle logging
  - `internal/feishu/client.go` - Feishu event and error logging
  - `internal/clawdbot/client.go` - Gateway communication logging
  - `internal/bridge/bridge.go` - Message processing logging
- Flags: `log.LstdFlags | log.Lshortfile` (timestamp + short file path)

## CI/CD & Deployment

**Hosting:**
- GitHub Releases - Binary distribution platform (`.github/workflows/release.yml`)
- Self-hosted deployment (binary runs on user's machine)

**CI Pipeline:**
- GitHub Actions (`.github/workflows/release.yml`)
- Trigger: Git tags matching `v*` pattern
- Steps:
  1. Checkout code
  2. Setup Go 1.21
  3. Build for all platforms using `scripts/build.sh`
  4. Upload binaries to GitHub Release
- Platforms: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64, arm64)

## Environment Configuration

**Required env vars:**
None - Configuration is file-based, not environment variable-based

**Config files required:**
- `~/.clawdbot/clawdbot.json` - Managed by ClawdBot (must be created by ClawdBot)
  - Structure: `{gateway: {port: int, auth: {token: string}}}`

- `~/.clawdbot/bridge.json` - Created/managed by bridge
  - Structure: `{feishu: {app_id: string, app_secret: string}, agent_id?: string, thinking_threshold_ms?: int}`

**Secrets location:**
- `~/.clawdbot/bridge.json` - Contains Feishu App Secret (plaintext JSON)
- File permissions: 0600 (read/write owner only) set in `cmd/bridge/main.go`
- `~/.clawdbot/clawdbot.json` - Contains gateway auth token (managed by ClawdBot)

## Webhooks & Callbacks

**Incoming:**
- Feishu WebSocket events (event-driven, not webhook-based)
  - Event: `p2_message_receive_v1` - Triggered when user sends direct message
  - Handler: `internal/feishu/client.go` - `handleMessage()`
  - Processed by: `internal/bridge/bridge.go` - `HandleMessage()`

**Outgoing:**
- Feishu REST API calls only (no webhook callbacks)
- Message operations:
  - Create message: `POST /open-apis/im/v1/messages`
  - Update message: `PATCH /open-apis/im/v1/messages/{message_id}`
  - Delete message: `DELETE /open-apis/im/v1/messages/{message_id}`

## Protocol Details

**Feishu WebSocket:**
- Connection managed by SDK
- Event dispatcher pattern from `larksuite/oapi-sdk-go/v3/event/dispatcher`
- Automatic reconnection and heartbeat handled by SDK

**ClawdBot Gateway WebSocket:**
- Custom protocol with JSON frames
- Request/Response pattern with message IDs for correlation
- Streaming responses via event payload mechanism
- Session-aware (session keys group related messages)

---

*Integration audit: 2026-01-29*
