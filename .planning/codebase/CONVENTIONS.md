# Coding Conventions

**Analysis Date:** 2026-01-29

## Naming Patterns

**Files:**
- Lowercase with underscores: `daemon_unix.go`, `daemon_windows.go`
- Descriptive purpose-based names: `client.go`, `config.go`, `bridge.go`
- Test files not present in codebase

**Functions:**
- PascalCase for exported functions: `NewClient()`, `Load()`, `Start()`, `HandleMessage()`
- camelCase for unexported functions: `isRunning()`, `readPID()`, `escapeJSON()`, `getStringValue()`
- Descriptive verb-based names: `Send`, `Update`, `Delete`, `Handle`, `Process`

**Variables:**
- camelCase: `cfg`, `err`, `ctx`, `msg`, `resp`, `chatID`, `messageID`
- Short names in local scopes: `p` (process), `b` (bridge), `c` (client), `mc` (messageCache)
- Descriptive names for important state: `placeholderID`, `sessionKey`, `responseChan`

**Types:**
- PascalCase struct names: `Client`, `Bridge`, `Config`, `Message`, `FeishuConfig`, `ClawdbotConfig`
- PascalCase type aliases: `MessageHandler`, `ConnectParams`, `ErrorInfo`
- Private types use lowercase: `messageCache`, `bridgeConfigJSON`, `clawdbotJSON`, `bridgeJSON`

**Constants:**
- No explicit constants found in codebase
- Magic numbers used directly: `10 * time.Minute`, `1 * time.Minute`, `5 * time.Minute`, `18789` (default port)

## Code Style

**Formatting:**
- Use `go fmt ./...` - standard Go formatting via Makefile
- Line length: appears to follow Go's standard (no specific limit enforced)
- Indentation: tabs (Go standard)
- Struct tags use underscore-separated keys in JSON: `app_id`, `app_secret`, `chat_id`, `message_id`

**Linting:**
- Tool: `go vet ./...` - standard Go linter via Makefile
- Run via `make lint` which includes both `fmt` and `vet`
- No custom linting rules observed
- Project enforces both formatting and vet checks

## Import Organization

**Order:**
1. Standard library imports: `context`, `encoding/json`, `fmt`, `log`, `os`, `sync`, `time`
2. External SDK imports: `github.com/google/uuid`, `github.com/gorilla/websocket`, `github.com/larksuite/oapi-sdk-go/v3`
3. Internal package imports: `github.com/wy51ai/moltbotCNAPP/internal/...`

**Path Aliases:**
- Feishu SDK: `lark "github.com/larksuite/oapi-sdk-go/v3"`
- Feishu core: `larkcore "github.com/larksuite/oapi-sdk-go/v3/core"`
- Feishu WebSocket: `larkws "github.com/larksuite/oapi-sdk-go/v3/ws"`
- Feishu IM: `larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"`

## Error Handling

**Patterns:**
- All functions that can fail return `(T, error)` or `error`
- Errors are wrapped with context using `fmt.Errorf(...: %w", err)` for stack trace preservation
- Error messages are descriptive and include operation context: `"failed to connect to gateway: %w"`, `"failed to send message: %w"`
- Errors logged with structured prefix indicating module: `[Main]`, `[Bridge]`, `[Feishu]`, `[Clawdbot]`
- Functions return early on error (guard clauses)

**Example from `internal/feishu/client.go`:**
```go
resp, err := c.client.Im.Message.Create(context.Background(), req)
if err != nil {
    return "", fmt.Errorf("failed to send message: %w", err)
}

if !resp.Success() {
    return "", fmt.Errorf("failed to send message: %s", resp.Msg)
}
```

**Example from `internal/config/config.go`:**
```go
if brCfg.Feishu.AppID == "" {
    return nil, fmt.Errorf("feishu.app_id is required in ~/.clawdbot/bridge.json")
}
```

## Logging

**Framework:** Standard Go `log` package

**Patterns:**
- Initialize once in main: `log.SetFlags(log.LstdFlags | log.Lshortfile)`
- Module-prefixed log messages: `[ModuleName]` prefix in each log line
- Info/debug logs: `log.Printf("[Module] Message: %v", value)`
- Fatal logs: `log.Fatalf("[Module] Message: %v", err)` - exits program
- Contextual logging includes relevant IDs: `chatID`, `messageID`, `sessionKey`

**Example from `cmd/bridge/main.go`:**
```go
log.SetFlags(log.LstdFlags | log.Lshortfile)
log.Println("[Main] Starting ClawdBot Bridge...")
log.Printf("[Main] Loaded config: AppID=%s, Gateway=127.0.0.1:%d, AgentID=%s",
    cfg.Feishu.AppID, cfg.Clawdbot.GatewayPort, cfg.Clawdbot.AgentID)
```

**Example from `internal/bridge/bridge.go`:**
```go
log.Printf("[Bridge] Processing message from %s: %s", msg.ChatID, text)
log.Printf("[Bridge] ClawdBot raw reply: %q", reply)
```

## Comments

**When to Comment:**
- Function-level documentation comments: Every exported function has a comment line starting with function name
- Explanation of non-obvious algorithm steps (e.g., multi-step WebSocket protocol in `internal/clawdbot/client.go`)
- Section headers for logical groupings: `// Step 1: Handle connect challenge`

**JSDoc/TSDoc:**
- Not applicable (Go project)
- Documentation follows Go conventions with function-level comments
- Example from `internal/bridge/bridge.go`:
  ```go
  // Bridge connects Feishu and ClawdBot
  type Bridge struct { ... }

  // NewBridge creates a new bridge
  func NewBridge(...) *Bridge { ... }

  // HandleMessage processes a message from Feishu
  func (b *Bridge) HandleMessage(msg *feishu.Message) error { ... }
  ```

## Function Design

**Size:** Functions range from 10-100 lines
- Handler functions: ~15-25 lines
- Client methods: ~30-50 lines for complex protocol handlers
- Utility functions: 5-15 lines
- Message processing: ~50-80 lines (`processMessage` in bridge.go with async behavior)

**Parameters:**
- Maximum 3-4 parameters typically
- Use structs for related parameters: `ConnectParams`, `AgentParams`, `Config` structs
- Receiver pattern for methods: `func (c *Client)`, `func (b *Bridge)`, `func (mc *messageCache)`

**Return Values:**
- Pattern: `(result T, err error)` or just `error`
- Errors always last return value
- Example: `func (c *Client) AskClawdbot(text, sessionKey string) (string, error)`

**Concurrency:**
- Use `sync.Mutex` for protecting shared state: `messageCache` uses `sync.RWMutex`
- Use `sync.Lock()` and `defer sync.Unlock()` pattern
- Goroutines spawned for async operations: message processing, cleanup routines
- Channels for communication: `responseChan`, `errorChan`
- Context for cancellation: `context.Context` parameter in Start/Begin methods

## Module Design

**Exports:**
- Exported names (PascalCase) are public API surface
- All main types export constructors: `NewClient()`, `NewBridge()`
- Setter methods for deferred initialization: `SetFeishuClient()`
- Helper functions exported when part of public API

**Barrel Files:**
- No barrel files (index.go) pattern observed
- Each package contains only necessary public exports in one or few files
- Example: `internal/bridge/` contains only `bridge.go`

**Package Organization:**
- Separation by domain: `bridge`, `clawdbot`, `config`, `feishu`
- Clear responsibilities: Each package handles one external service or concern
- Internal packages under `internal/` to prevent external imports

## Configuration Patterns

**Config Loading:**
- Config loaded from JSON files in `~/.clawdbot/` directory
- Separate files for different concerns: `clawdbot.json`, `bridge.json`
- Config structs with JSON tags: fields tagged with `json:"field_name"` for unmarshaling
- Validation during load: checks for required fields and provides helpful error messages

**Example from `internal/config/config.go`:**
```go
type FeishuConfig struct {
    AppID               string
    AppSecret           string
    ThinkingThresholdMs int
}

// Load reads configuration from ~/.clawdbot/ config files
func Load() (*Config, error) {
    // ... validation with helpful messages
    if brCfg.Feishu.AppID == "" {
        return nil, fmt.Errorf("feishu.app_id is required in ~/.clawdbot/bridge.json")
    }
}
```

---

*Convention analysis: 2026-01-29*
