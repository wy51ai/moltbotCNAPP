# Codebase Concerns

**Analysis Date:** 2026-01-29

## Tech Debt

**Ignored JSON Unmarshal Errors in Config Loading:**
- Issue: In `cmd/bridge/main.go` line 256, `json.Unmarshal` error is discarded when reading existing config without validation
- Files: `cmd/bridge/main.go:256`
- Impact: Silent failures during config migration; corrupted or malformed JSON files are ignored, potentially losing user config state
- Fix approach: Check error return from `json.Unmarshal` and log warning/handle gracefully. Validate config structure before applying defaults.

**Missing Error Handling in Message Reader Goroutine:**
- Issue: In `internal/clawdbot/client.go` lines 127-130, the message reader goroutine silently returns on `ReadMessage()` error without cleanup or error reporting
- Files: `internal/clawdbot/client.go:127-129`
- Impact: WebSocket connection loss goes undetected; client hangs in select waiting for response that will never come, blocking for 5 minutes until timeout
- Fix approach: Send error to `errorChan` when `ReadMessage()` fails so the caller knows immediately the connection died

**Race Condition in Bridge Message Processing:**
- Issue: In `internal/bridge/bridge.go`, the `done` variable is shared between timer callback and main goroutine without proper synchronization in all code paths
- Files: `internal/bridge/bridge.go:126-162`
- Impact: Timer callback may send "thinking..." message after response already sent; placeholder message ID could be corrupted if timer fires during final mu.Lock section
- Fix approach: Use atomic.Bool or wrap all access to `done` flag with consistent mutex locking

**Unbounded Goroutine in Message Cache Cleanup:**
- Issue: In `internal/bridge/bridge.go:37`, cleanup goroutine is spawned with no lifecycle management; will leak if bridge is recreated
- Files: `internal/bridge/bridge.go:30-40`
- Impact: Long-lived processes accumulate goroutines; test suites may accumulate orphaned cleanup goroutines
- Fix approach: Accept context in `newMessageCache()` and stop cleanup goroutine when context is cancelled

**Hardcoded localhost in ClawdBot Client:**
- Issue: In `internal/clawdbot/client.go:112`, WebSocket URL hardcodes `127.0.0.1` instead of making it configurable
- Files: `internal/clawdbot/client.go:112`
- Impact: Cannot test against remote gateway; Docker deployments or alternative network configurations cannot work
- Fix approach: Add `GatewayHost` to `ClawdbotConfig` struct, default to "127.0.0.1"

## Known Bugs

**PID File Not Cleaned on Daemon Start Failure:**
- Symptoms: If daemon process crashes immediately after startup, PID file is never removed; subsequent `start` command incorrectly reports "Already running"
- Files: `cmd/bridge/main.go:100-114`
- Trigger: Start daemon, let it fail with error before logging first message, then try to start again
- Workaround: Manually delete `~/.clawdbot/bridge.pid` before retrying

**Config File Permissions Inconsistency:**
- Symptoms: PID file created with 0644 (world-readable), but sensitive config file created with 0600 (restrictive); inconsistent security posture
- Files: `cmd/bridge/main.go:111` (PID 0644), `cmd/bridge/main.go:276` (config 0600)
- Trigger: View file permissions after running daemon
- Workaround: Manually chmod 600 the PID file; improve documentation about security implications

**Thinking Message May Persist on Timeout:**
- Symptoms: If ClawdBot response times out after "thinking..." message sent, the placeholder message is never deleted; lingers in chat showing outdated status
- Files: `internal/bridge/bridge.go:172-184`
- Trigger: Send message that takes >5 minutes to respond, waits for response
- Workaround: Manually delete the "正在思考…" message from chat

## Security Considerations

**Credentials Stored in Plaintext Config File:**
- Risk: Feishu App Secret stored in `~/.clawdbot/bridge.json` with only user-readable permissions (0600), but on shared systems or with file backup tools, credentials could be leaked
- Files: `internal/config/config.go:77`, `cmd/bridge/main.go:276`
- Current mitigation: File created with 0600 permissions (user-only readable)
- Recommendations:
  - Consider OS keyring integration (macOS Keychain, Linux Secret Service, Windows Credential Manager)
  - Document security implications in README
  - Add warning if ~/.clawdbot directory has overly permissive permissions

**Message Content Passed Through ClawdBot Without Input Validation:**
- Risk: User messages from Feishu are forwarded directly to ClawdBot without sanitization; large messages, binary data, or malformed UTF-8 could cause issues
- Files: `internal/bridge/bridge.go:102-108`
- Current mitigation: Basic regex mention removal; chat type check
- Recommendations:
  - Add message size limit (e.g., 10KB max)
  - Validate UTF-8 encoding before sending to gateway
  - Add rate limiting per chat to prevent flooding

**Feishu Client Credentials Not Verified:**
- Risk: If App ID or App Secret is invalid, failure only discovered at runtime when Start() is called; error messages could leak valid app structure to logs
- Files: `internal/config/config.go:88-93`
- Current mitigation: Validation at config load time
- Recommendations:
  - Add optional credential verification step before daemon starts
  - Sanitize error messages in logs to not reveal if AppID exists

**JSON Escaping Implementation:**
- Risk: In `internal/feishu/client.go:206-212`, custom JSON escaping uses `json.Marshal` and string slicing, which could be error-prone for edge cases (null bytes, surrogates)
- Files: `internal/feishu/client.go:206-212`
- Current mitigation: Uses json.Marshal which handles UTF-8 correctly
- Recommendations:
  - Consider using dedicated JSON encoding library builder instead of raw string concat
  - Add tests for edge cases (emoji, RTL text, control characters)

## Performance Bottlenecks

**Synchronous WebSocket Message Reading:**
- Problem: ClawdBot client creates new WebSocket connection per message; no connection pooling or reuse
- Files: `internal/clawdbot/client.go:108-117`
- Cause: Each `AskClawdbot()` call dials fresh connection, performs full authentication handshake (step 1-3), then sends message
- Improvement path: Maintain persistent WebSocket connection pool; reuse authenticated connections across multiple requests; implement connection health checks

**Message Cache Cleanup Every Minute Regardless of Load:**
- Problem: `cleanup()` goroutine wakes up every minute even if no messages are cached; constant timer allocation and lock contention
- Files: `internal/bridge/bridge.go:57-71`
- Cause: Fixed 1-minute ticker with no adaptive behavior
- Improvement path: Use separate cleanup triggers (e.g., mark entries for deletion on access, cleanup only when cache exceeds threshold)

**Sequential Message Processing in Bridge:**
- Problem: Messages are processed asynchronously but synchronously within each goroutine; if ClawdBot is slow, chat backlog builds up
- Files: `internal/bridge/bridge.go:121`
- Cause: No concurrency limit on `go b.processMessage()` spawned goroutines
- Improvement path: Add worker pool pattern to limit concurrent processing; implement message queue with metrics

**5-Minute Hardcoded Timeout:**
- Problem: ClawdBot response timeout is hardcoded to 5 minutes; too long for user perception, no configurability
- Files: `internal/clawdbot/client.go:272`
- Cause: No timeout configuration parameter
- Improvement path: Make timeout configurable via environment or config file; consider smaller default (30s-1min) with user-visible "still thinking..." updates

## Fragile Areas

**Main Process Management:**
- Files: `cmd/bridge/main.go:22-58`
- Why fragile: Command parsing is simplistic string switch; no validation of command syntax; easy to add new commands that break existing behavior
- Safe modification: Add structured command dispatcher; validate all arguments before action; add integration tests for each command path
- Test coverage: No tests for daemon lifecycle (start, stop, restart, status)

**Bridge Message Handling State Machine:**
- Files: `internal/bridge/bridge.go:126-210`
- Why fragile: Complex timing logic with multiple goroutines, channels, and mutexes; easy to introduce deadlocks or race conditions in future changes
- Safe modification: Extract thinking message logic into separate function with its own lifecycle; add extensive comments documenting intended behavior; write concurrent unit tests
- Test coverage: No tests for concurrent message processing, timeout scenarios, or message update failures

**ClawdBot Protocol Handshake:**
- Files: `internal/clawdbot/client.go:137-216`
- Why fragile: Protocol depends on exact event sequence and response structure; missing or out-of-order events silently ignored; protocol version hardcoded to 3
- Safe modification: Add protocol state machine; validate transitions; log unexpected events; extract into separate protocol handler module
- Test coverage: No tests for gateway communication; no mock WebSocket server for offline testing

**Config Loading from Dual Files:**
- Files: `internal/config/config.go:60-120`
- Why fragile: Depends on both `clawdbot.json` (managed by external tool) and `bridge.json` (managed by this tool); inconsistent schema handling with defaulting logic scattered across code
- Safe modification: Consolidate config source; add validation schema; document all defaults explicitly; add config migration tests
- Test coverage: No tests for config loading; no tests for missing files, malformed JSON, or incomplete configs

**Feishu SDK Integration:**
- Files: `internal/feishu/client.go`
- Why fragile: Tightly coupled to specific larksuite SDK version (v3.5.3); SDK updates could break struct marshaling or API behavior
- Safe modification: Add wrapper layer around SDK types; pin SDK version explicitly; add integration tests against Feishu test environment
- Test coverage: No tests for message sending, updating, or deletion; API errors not tested

## Scaling Limits

**Single Gateway Connection Per Process:**
- Current capacity: One bridge instance serves all chats through single ClawdBot gateway
- Limit: ClawdBot gateway throughput; if gateway can handle N messages/sec, adding more chats just increases queue length
- Scaling path: Support multiple bridge instances per gateway; add load balancing; implement message queue with worker pool

**Memory Unbounded for Large Message Caches:**
- Current capacity: Message cache stores IDs indefinitely until 10-minute TTL expires (no size limit)
- Limit: With many unique chat rooms, cache could grow to megabytes even though IDs are small
- Scaling path: Add max cache size limit; implement LRU eviction; monitor cache size in metrics

**No Connection Multiplexing:**
- Current capacity: Each message opens/closes WebSocket; authentication for every message
- Limit: Gateway becomes bottleneck; connection pool exhaustion under high load; TCP connection limits
- Scaling path: Implement persistent connection with request multiplexing; connection pooling; load shedding

## Dependencies at Risk

**larksuite/oapi-sdk-go/v3 (v3.5.3) - Indirect dependency on protobuf:**
- Risk: SDK transitively depends on `github.com/gogo/protobuf v1.3.2` which is deprecated; no active maintenance
- Impact: Security vulnerabilities in protobuf won't be patched; potential incompatibility with newer Go versions
- Migration plan: Monitor for protobuf fork that's maintained; consider using official protobuf library; evaluate alternative Feishu SDK

**gorilla/websocket (v1.5.1) - Potential replacement:**
- Risk: While maintained, gorilla/websocket is being phased out in favor of x/net/websocket in Go standard library (still experimental)
- Impact: Long-term maintenance uncertainty; could be unmaintained in future Go versions
- Migration plan: Monitor Go releases; consider x/net/websocket migration path when stable; abstract websocket behind interface

## Missing Critical Features

**No Graceful Shutdown:**
- Problem: SIGTERM stops goroutines immediately; pending messages are dropped; WebSocket connections forcefully closed
- Blocks: Cannot safely restart daemon without losing in-flight messages
- Fix approach: Implement graceful shutdown with timeout; wait for in-flight messages before closing connections; flush message cache on exit

**No Metrics or Observability:**
- Problem: Only simple text logging; no metrics collection, no prometheus endpoint, no structured logging
- Blocks: Cannot diagnose performance issues; cannot track uptime; no alerting capability
- Fix approach: Add structured logging (JSON); expose metrics (prometheus format); track message latency, error rates, gateway health

**No Retry Logic for Failed Messages:**
- Problem: If sending message to Feishu fails, silently logs error; no retry mechanism or dead letter queue
- Blocks: User never knows message was lost; no way to recover
- Fix approach: Add configurable retry with exponential backoff; implement dead letter queue for failed messages

**No Health Check Endpoint:**
- Problem: No way to verify bridge is healthy without running test message
- Blocks: Load balancer cannot health check; monitoring tools cannot verify status
- Fix approach: Add simple HTTP health check endpoint; check ClawdBot gateway connectivity; expose in metrics

**No Configuration Hot-Reload:**
- Problem: Changing config requires daemon restart
- Blocks: Cannot update agent ID or thinking threshold without service interruption
- Fix approach: Add config file watcher; validate new config before applying; implement hot-reload with zero downtime

## Test Coverage Gaps

**No Unit Tests:**
- What's not tested: All core business logic (message handling, protocol handshakes, config loading)
- Files: `internal/bridge/bridge.go`, `internal/clawdbot/client.go`, `internal/feishu/client.go`, `internal/config/config.go`
- Risk: Regressions go undetected; refactoring is risky; protocol changes break silently
- Priority: High - Core business logic has zero test coverage

**No Integration Tests:**
- What's not tested: End-to-end flow from Feishu message to ClawdBot response; error scenarios (gateway down, bad credentials, timeout)
- Files: All internal modules
- Risk: Real integration points are untested; Feishu/ClawdBot API changes break in production
- Priority: High - Critical workflows unvalidated

**No Daemon Lifecycle Tests:**
- What's not tested: start/stop/restart commands, PID file management, log file creation, config argument parsing
- Files: `cmd/bridge/main.go`
- Risk: Daemon can deadlock or leave stale processes; PID file bugs undetected
- Priority: Medium - Operational issues could strand processes

**No Edge Case Tests:**
- What's not tested: Empty messages, very long messages, special characters (emoji, RTL), mention parsing, concurrent message handling
- Files: `internal/bridge/bridge.go`
- Risk: Crashes or silent drops in production with unexpected input
- Priority: Medium - User-facing stability concerns

**No Mocking Framework:**
- What's not tested: Gateway/Feishu failures can be simulated for testing error handling
- Files: All
- Risk: Error paths never exercised; timeout/retry logic untested; unreliable network scenarios untested
- Priority: Medium - Infrastructure resilience unknown

---

*Concerns audit: 2026-01-29*
