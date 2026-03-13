# Architecture

**Analysis Date:** 2026-01-29

## Pattern Overview

**Overall:** Synchronous Bridge with Async Message Handling

**Key Characteristics:**
- Single-purpose daemon: connects Feishu messaging platform with ClawdBot AI Gateway
- Event-driven message processing via WebSocket subscriptions
- Layered architecture with clear separation of concerns
- Daemon lifecycle management with foreground/background execution modes

## Layers

**Command Layer (Entry Point):**
- Purpose: CLI command router and daemon lifecycle management
- Location: `cmd/bridge/main.go`
- Contains: Command parsing (start, stop, restart, status, run), daemon spawning, PID file management, configuration argument parsing
- Depends on: config, bridge, feishu, clawdbot packages
- Used by: System commands via executable

**Bridge Layer (Message Orchestration):**
- Purpose: Core message routing and processing logic between Feishu and ClawdBot
- Location: `internal/bridge/bridge.go`
- Contains: Message deduplication, message filtering (group chat triggers), async processing coordination, response caching
- Depends on: feishu, clawdbot clients
- Used by: Command layer and Feishu message handler

**Feishu Integration Layer:**
- Purpose: WebSocket client for Feishu API, message reception and sending
- Location: `internal/feishu/client.go`
- Contains: Feishu WebSocket connection, event dispatcher, message send/update/delete operations
- Depends on: larksuite/oapi-sdk-go/v3
- Used by: Bridge layer, command layer

**ClawdBot Integration Layer:**
- Purpose: WebSocket client for ClawdBot Gateway, request/response protocol handling
- Location: `internal/clawdbot/client.go`
- Contains: Gateway authentication handshake, agent request protocol, event streaming
- Depends on: gorilla/websocket, google/uuid
- Used by: Bridge layer

**Configuration Layer:**
- Purpose: Configuration loading and validation
- Location: `internal/config/config.go`
- Contains: Config struct definitions, file I/O from `~/.clawdbot/`, validation logic
- Depends on: Standard library only
- Used by: Command layer, all other layers indirectly

## Data Flow

**Incoming Message Flow (Feishu → ClawdBot → Feishu):**

1. **Reception**: Feishu WebSocket client receives message via `P2MessageReceiveV1` event
2. **Deduplication**: Bridge checks if message ID already seen (10-minute TTL cache)
3. **Filtering**: Bridge evaluates whether to respond based on chat type and content triggers
4. **Processing**: Message sent to `bridge.processMessage()` asynchronously
5. **Query**: Bridge sends message to ClawdBot via WebSocket with session key `feishu:{chatID}`
6. **Response Collection**: ClawdBot streams response via `agent` events with `assistant` stream data
7. **Thinking Placeholder**: If response takes longer than `thinkingMs`, a "思考中..." message is sent first
8. **Update/Send**: Response either updates the placeholder message or sends new message
9. **Cleanup**: If response is empty or "NO_REPLY", placeholder is deleted and nothing sent

**State Management:**
- Message deduplication state: In-memory map with periodic TTL cleanup (1-minute interval, 10-minute TTL)
- Client connections: Long-lived WebSocket connections maintained for Feishu and per-request for ClawdBot
- No persistent state between requests

## Key Abstractions

**Bridge:**
- Purpose: Orchestrates message flow, handles deduplication and response coordination
- Examples: `Bridge.HandleMessage()`, `Bridge.processMessage()`
- Pattern: Observer pattern - receives callbacks from Feishu, calls ClawdBot, sends responses back to Feishu

**Message Cache:**
- Purpose: Prevents duplicate message processing within a 10-minute window
- Examples: `messageCache.has()`, `messageCache.add()`
- Pattern: Time-based LRU with background cleanup goroutine

**Feishu Client:**
- Purpose: Wraps Feishu SDK with consistent interface for message operations
- Examples: `SendMessage()`, `UpdateMessage()`, `DeleteMessage()`
- Pattern: Facade over larksuite SDK, presents simplified API

**ClawdBot Client:**
- Purpose: Implements ClawdBot Gateway WebSocket protocol (connect → agent → stream)
- Examples: `AskClawdbot()` - synchronous wrapper over async protocol
- Pattern: Protocol state machine - handles 3-step handshake and event streaming

## Entry Points

**CLI Entry Point:**
- Location: `cmd/bridge/main.go` function `main()`
- Triggers: Direct execution of binary with command argument
- Responsibilities: Route command (start/stop/restart/status/run), apply configuration, manage daemon lifecycle

**Run Mode Entry:**
- Location: `cmd/bridge/main.go` function `cmdRun()`
- Triggers: `start` or `run` command, or internal re-exec via daemon
- Responsibilities: Initialize clients, start Feishu WebSocket, connect signal handler, run until shutdown

**Message Handler:**
- Location: `internal/bridge/bridge.go` function `HandleMessage()`
- Triggers: Feishu WebSocket event dispatcher
- Responsibilities: Validate message, check duplicates, filter group messages, enqueue async processing

## Error Handling

**Strategy:** Layered error propagation with logging at each layer

**Patterns:**
- Fatal errors: Config loading, WebSocket connection failures → log and exit
- Recoverable errors: Message processing failures → log, send error message to user, continue
- Timeout: ClawdBot responses timeout after 5 minutes → return error string to user
- Graceful degradation: If thinking message can't be deleted, still sends final response

## Cross-Cutting Concerns

**Logging:**
- Tool: Standard `log` package with custom prefix: `[Main]`, `[Feishu]`, `[Bridge]`, `[ClawdBot]`
- Pattern: Log at INFO level (SDK default), use Printf for key lifecycle events

**Validation:**
- Config validation: `config.Load()` validates required Feishu credentials before daemon spawn
- Message validation: Only process text messages, skip if content empty after cleaning

**Authentication:**
- Feishu: SDK handles app credentials, passed as `appID` and `appSecret`
- ClawdBot: Token-based auth via gateway token, sent in connect handshake

---

*Architecture analysis: 2026-01-29*
