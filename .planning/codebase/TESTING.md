# Testing Patterns

**Analysis Date:** 2026-01-29

## Test Framework

**Runner:**
- `go test` - Standard Go testing framework
- Config: No config file needed (Go built-in)
- Version: Go 1.21+ (from `go.mod`)

**Assertion Library:**
- None used - Standard Go `testing.T` assertion patterns

**Run Commands:**
```bash
make test                    # Run all tests: go test -v ./...
go test ./...              # Run all tests
go test -v ./...           # Run all tests with verbose output
go test ./internal/bridge  # Test specific package
go test -run TestName ./... # Run specific test
```

## Test File Organization

**Location:**
- **CRITICAL FINDING:** No `*_test.go` files present in codebase
- No test files detected in any packages: `internal/bridge/`, `internal/clawdbot/`, `internal/config/`, `internal/feishu/`
- No test utilities or fixtures defined

**Naming:**
- Convention would be `filename_test.go` (follows Go standard)
- No tests currently exist to reference

**Structure:**
```
package_name/
├── source.go          # Implementation
└── source_test.go     # Tests (NOT PRESENT - MISSING)
```

## Test Structure

**Suite Organization:**
- No test files currently exist
- Standard Go pattern would be:
```go
package bridge

import "testing"

func TestHandleMessage(t *testing.T) {
    // Test implementation
}

func TestProcessMessage(t *testing.T) {
    // Test implementation
}
```

**Patterns (Recommended for implementation):**
- Use `func TestXxx(t *testing.T)` naming convention
- Use `t.Run()` for subtests when testing multiple scenarios
- Setup/teardown via functions or `t.Cleanup()`
- Table-driven tests for multiple input/output combinations

## Mocking

**Framework:** None currently used

**Recommended Approach:**
- Use interfaces for mockable components
- Example from existing code - interfaces already support mocking:
  - `MessageHandler` in `internal/feishu/client.go`: `type MessageHandler func(msg *Message) error`
  - Could mock `Client` types by creating test implementations

**Current Architecture (Supports Mocking):**
- WebSocket client can be mocked via dependency injection
- Bridge accepts clients in constructor: `NewBridge(feishuClient, clawdbotClient, thinkingMs)`
- Feishu client accepts handler function: `NewClient(appID, appSecret, handler MessageHandler)`

**What to Mock:**
- External service clients (Feishu API, ClawdBot WebSocket gateway)
- File system operations (config loading)
- WebSocket connections
- HTTP/API responses

**What NOT to Mock:**
- Business logic in bridge (message routing, deduplication)
- JSON marshaling/unmarshaling
- Error handling paths
- Time-based operations (use `time.NewTicker` stubs)

## Fixtures and Factories

**Test Data:**
- No fixtures or factory functions currently implemented
- Would need to create test doubles for:
  - Message fixtures with various content/mentions
  - Config fixtures for different scenarios
  - Mock WebSocket responses

**Recommended Fixtures (to implement):**
```go
// In internal/feishu/client_test.go (proposed)
func createTestMessage(content string) *Message {
    return &Message{
        MessageID: "msg_test_123",
        ChatID:    "chat_test_456",
        ChatType:  "p2p",
        Content:   content,
    }
}

// In internal/config/config_test.go (proposed)
func createTestConfig() *Config {
    return &Config{
        Feishu: FeishuConfig{
            AppID:               "test_app_id",
            AppSecret:           "test_app_secret",
            ThinkingThresholdMs: 1000,
        },
        Clawdbot: ClawdbotConfig{
            GatewayPort:  18789,
            GatewayToken: "test_token",
            AgentID:      "main",
        },
    }
}
```

**Location:**
- Would be in same package as source: `internal/feishu/fixtures.go` (proposed)
- Alternative: use `*_test.go` suffix for test utilities

## Coverage

**Requirements:** No coverage enforced currently

**Recommended Target:** Add tests for:
- `internal/config/config.go` - Config loading and validation
- `internal/bridge/bridge.go` - Message routing and deduplication logic
- `internal/feishu/client.go` - Message parsing and field extraction
- `internal/clawdbot/client.go` - WebSocket protocol handling

**View Coverage (Not Currently Available):**
```bash
go test -cover ./...                    # Show coverage percentage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out       # View coverage in HTML
```

## Test Types

**Unit Tests (To Implement):**
- **Scope:** Individual functions and methods
- **Approach:**
  - Test message deduplication in `Bridge.HandleMessage()`
  - Test config validation in `Config.Load()`
  - Test message parsing in `Feishu.handleMessage()`
  - Test trigger detection in `shouldRespondInGroup()`

**Example Test Case (Proposed):**
```go
func TestBridgeHandleMessageDeduplication(t *testing.T) {
    b := NewBridge(nil, nil, 0)

    msg := &Message{
        MessageID: "msg_123",
        ChatID:    "chat_456",
        Content:   "test message",
    }

    // First call should succeed
    err := b.HandleMessage(msg)
    if err != nil {
        t.Fatalf("first HandleMessage failed: %v", err)
    }

    // Second call with same ID should be skipped (no error)
    err = b.HandleMessage(msg)
    if err != nil {
        t.Fatalf("second HandleMessage failed: %v", err)
    }
}
```

**Integration Tests (To Implement):**
- **Scope:** Multiple packages/modules working together
- **Approach:**
  - Test config loading and bridge initialization
  - Test message flow from Feishu handler to ClawdBot client
  - Test error propagation through layers
  - Requires mocking external services (Feishu API, ClawdBot gateway)

**Example Integration Test (Proposed):**
```go
func TestBridgeMessageFlow(t *testing.T) {
    // Setup mock clients
    mockClawdbotClient := &MockClawdbotClient{}
    mockFeishuClient := &MockFeishuClient{}

    b := NewBridge(mockFeishuClient, mockClawdbotClient, 0)

    // Send message
    msg := createTestMessage("test question?")
    err := b.HandleMessage(msg)

    // Verify ClawdBot was called
    if !mockClawdbotClient.AskClawdbotCalled {
        t.Fatal("ClawdBot not called")
    }
}
```

**E2E Tests:** Not implemented

## Concurrency Testing

**Current Code (Requires Testing):**
- Message cache cleanup goroutine in `messageCache.cleanup()`
- Async message processing in `Bridge.processMessage()`
- Multiple concurrent goroutines in `AskClawdbot()` WebSocket handling

**Recommended Approach:**
```go
func TestMessageCacheCleanup(t *testing.T) {
    mc := newMessageCache(100 * time.Millisecond)

    mc.add("msg_1")
    if !mc.has("msg_1") {
        t.Fatal("message not added")
    }

    // Wait for TTL to expire
    time.Sleep(150 * time.Millisecond)

    // Message should be cleaned up
    if mc.has("msg_1") {
        t.Fatal("message not cleaned up")
    }
}
```

## Error Testing

**Current Error Paths (Need Tests):**
- Config load failures: missing files, invalid JSON, missing required fields
- WebSocket connection failures
- Message send/update/delete failures
- JSON unmarshaling errors
- Process management (start, stop, status) errors

**Recommended Pattern:**
```go
func TestConfigLoadMissingAppID(t *testing.T) {
    // Mock filesystem to return config without app_id
    cfg := &bridgeJSON{
        Feishu: struct {
            AppID     string
            AppSecret string
        }{
            AppID:     "",
            AppSecret: "secret",
        },
    }

    // Load should fail
    _, err := unmarshalConfig(cfg)
    if err == nil || !strings.Contains(err.Error(), "app_id") {
        t.Fatalf("expected app_id error, got: %v", err)
    }
}
```

## Critical Testing Gaps

**Missing Test Coverage:**
1. **`internal/bridge/bridge.go`** - Core business logic for message routing and deduplication
2. **`internal/config/config.go`** - Configuration validation and loading
3. **`internal/feishu/client.go`** - Message parsing and field extraction from Feishu SDK responses
4. **`internal/clawdbot/client.go`** - WebSocket protocol handling (complex multi-step handshake)
5. **`cmd/bridge/main.go`** - Daemon operations (start, stop, restart, status) with PID file management
6. **Error paths** - All error returns lack test coverage
7. **Concurrency safety** - Mutex-protected operations not verified
8. **Message filtering** - `shouldRespondInGroup()` logic needs test coverage for various triggers

**Priority for Test Implementation:**
1. **High:** Config loading validation (guards against startup failures)
2. **High:** Message deduplication (prevents infinite loops)
3. **High:** WebSocket protocol in ClawdBot client (critical for main functionality)
4. **Medium:** Feishu message parsing (prevents data corruption)
5. **Medium:** Message filtering logic (controls bot behavior)
6. **Low:** Daemon lifecycle operations (platform-specific, less critical path)

---

*Testing analysis: 2026-01-29*
