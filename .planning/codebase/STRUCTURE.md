# Codebase Structure

**Analysis Date:** 2026-01-29

## Directory Layout

```
moltbotCNAPP/
├── cmd/
│   └── bridge/              # Command line interface and daemon lifecycle
│       ├── main.go          # CLI router, daemon spawning, config parsing
│       ├── daemon_unix.go    # Unix/Linux daemon helpers (setsid, signal)
│       └── daemon_windows.go # Windows daemon helpers (stub)
├── internal/
│   ├── bridge/              # Message orchestration layer
│   │   └── bridge.go        # Bridge: message routing, deduplication, response coordination
│   ├── clawdbot/            # ClawdBot Gateway integration
│   │   └── client.go        # WebSocket client, protocol handshake, agent requests
│   ├── config/              # Configuration management
│   │   └── config.go        # Config loading from ~/.clawdbot/, validation
│   └── feishu/              # Feishu IM integration
│       └── client.go        # WebSocket client, message send/update/delete
├── scripts/                 # Build scripts
│   └── build.sh             # Cross-platform compilation script
├── .github/
│   └── workflows/           # GitHub Actions CI/CD
├── go.mod                   # Go module definition
├── go.sum                   # Dependency lock file
├── Makefile                 # Development targets
├── README.md                # Documentation (Chinese)
└── LICENSE                  # MIT License
```

## Directory Purposes

**cmd/bridge:**
- Purpose: Entry point, command routing, daemon lifecycle management
- Contains: Go source files implementing CLI commands (start, stop, restart, status, run)
- Key files: `main.go` (210 lines), `daemon_unix.go` (29 lines), `daemon_windows.go` (stub)
- Produces: Executable binary `clawdbot-bridge`

**internal/bridge:**
- Purpose: Core message orchestration between Feishu and ClawdBot
- Contains: Bridge struct, message deduplication cache, message filtering logic
- Key files: `bridge.go` (262 lines)
- Dependencies: feishu, clawdbot packages

**internal/clawdbot:**
- Purpose: ClawdBot Gateway integration via WebSocket
- Contains: Client implementation, protocol types, gateway communication
- Key files: `client.go` (276 lines)
- Protocols: Custom JSON-RPC over WebSocket (connect → agent → streaming)

**internal/config:**
- Purpose: Configuration loading and validation
- Contains: Config structs, file loading from `~/.clawdbot/`, field validation
- Key files: `config.go` (121 lines)
- Config files read: `~/.clawdbot/clawdbot.json`, `~/.clawdbot/bridge.json`

**internal/feishu:**
- Purpose: Feishu messaging platform integration via WebSocket
- Contains: Client implementation, message types, Feishu API operations
- Key files: `client.go` (214 lines)
- Wraps: larksuite/oapi-sdk-go/v3 (Feishu official SDK)

## Key File Locations

**Entry Points:**
- `cmd/bridge/main.go`: Binary entry point, accepts commands: start, stop, restart, status, run

**Configuration:**
- `~/.clawdbot/clawdbot.json`: ClawdBot Gateway settings (managed by ClawdBot, read by bridge)
- `~/.clawdbot/bridge.json`: Bridge-specific Feishu credentials and options
- `go.mod`: Dependency declarations

**Core Logic:**
- `internal/bridge/bridge.go`: Message routing, deduplication, filtering, response handling
- `internal/feishu/client.go`: Feishu WebSocket events and message operations
- `internal/clawdbot/client.go`: ClawdBot Gateway protocol implementation

**Testing:**
- No dedicated test files found in codebase

**Build/Deploy:**
- `scripts/build.sh`: Cross-platform build script
- `Makefile`: Development build targets
- `.github/workflows/`: GitHub Actions release automation

## Naming Conventions

**Files:**
- `snake_case.go` for all Go files
- Platform-specific files: `daemon_unix.go`, `daemon_windows.go` (build tags: `//go:build !windows`)

**Directories:**
- `cmd/`: Command-line entry points
- `internal/`: Non-exportable packages (cannot be imported by external projects)

**Functions:**
- Exported: `PascalCase` (e.g., `NewClient`, `HandleMessage`, `SendMessage`)
- Unexported: `camelCase` (e.g., `handleMessage`, `processMessage`, `newMessageCache`)

**Types/Structs:**
- `PascalCase` (e.g., `Bridge`, `Client`, `Message`, `Config`)
- JSON tags: `snake_case` (e.g., `"app_id"`, `"gateway_port"`)

**Constants/Variables:**
- Package-level unexported: `camelCase` (e.g., `questionWords`, `actionVerbs`)

## Where to Add New Code

**New Feature (e.g., additional message filtering):**
- Primary code: `internal/bridge/bridge.go` - add logic to `shouldRespondInGroup()` function
- Tests: Create `internal/bridge/bridge_test.go` (currently missing)
- Config if needed: Add fields to `FeishuConfig` or `ClawdbotConfig` structs in `internal/config/config.go`

**New Command (e.g., new CLI command):**
- Implementation: Add `cmd*()` function in `cmd/bridge/main.go` (around line 60-164)
- CLI routing: Add case to switch statement in `main()` function (lines 28-57)
- Daemon helpers if needed: Add logic to `daemon_unix.go` and `daemon_windows.go`

**New Platform Integration:**
- New client: Create `internal/{platform}/client.go` (follow pattern in `internal/feishu/` or `internal/clawdbot/`)
- Interface: Define `MessageHandler` callback similar to feishu's (line 16 of `internal/feishu/client.go`)
- Bridge support: Extend `Bridge` struct to hold new client in `internal/bridge/bridge.go`

**Utility Helpers:**
- String manipulation: `internal/bridge/bridge.go` (see `removeMentions()`, `shouldRespondInGroup()`)
- Config helpers: `internal/config/config.go` (see `Dir()` function)

## Special Directories

**internal/**
- Purpose: Private packages not exported outside project
- Generated: No
- Committed: Yes

**.planning/codebase/**
- Purpose: Architecture documentation and planning guides
- Generated: Yes (via `/gsd:map-codebase`)
- Committed: Yes

**.github/workflows/**
- Purpose: GitHub Actions CI/CD automation
- Generated: No (manually maintained)
- Committed: Yes

**scripts/**
- Purpose: Development and build automation scripts
- Generated: No
- Committed: Yes

## Import Organization

**Standard Pattern (observed across all files):**

```go
import (
    // 1. Standard library packages
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    // ... other stdlib

    // 2. External dependencies (blank line separator)
    "github.com/gorilla/websocket"
    "github.com/google/uuid"
    "github.com/larksuite/oapi-sdk-go/v3"
    // ... other external

    // 3. Internal packages
    "github.com/wy51ai/moltbotCNAPP/internal/bridge"
    "github.com/wy51ai/moltbotCNAPP/internal/config"
)
```

**No path aliases used** - direct imports with full module path

## Module Organization

**cmd/bridge:**
- Single package `main` with no subpackages
- Exports nothing (it's an executable, not a library)
- Functions: `main()`, `cmdStart()`, `cmdRun()`, `cmdStop()`, `cmdStatus()`

**internal/bridge:**
- Single package `bridge`
- Exports: `Bridge`, `NewBridge()`, `HandleMessage()`, `SetFeishuClient()`
- Unexported helpers: `messageCache`, `processMessage()`, `shouldRespondInGroup()`, `removeMentions()`

**internal/feishu:**
- Single package `feishu`
- Exports: `Client`, `Message`, `Mention`, `NewClient()`, methods on `Client`
- Unexported helpers: `getStringValue()`, `escapeJSON()`

**internal/clawdbot:**
- Single package `clawdbot`
- Exports: `Client`, request/response types, `NewClient()`, `AskClawdbot()` method
- Unexported: Protocol state machine logic in `AskClawdbot()` function

**internal/config:**
- Single package `config`
- Exports: `Config`, `FeishuConfig`, `ClawdbotConfig`, `Dir()`, `Load()`
- Unexported: `clawdbotJSON`, `bridgeJSON` (internal JSON representations)

---

*Structure analysis: 2026-01-29*
