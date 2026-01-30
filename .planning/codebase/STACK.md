# Technology Stack

**Analysis Date:** 2026-01-29

## Languages

**Primary:**
- Go 1.21+ - Bridge application, all source code in `cmd/` and `internal/` directories

## Runtime

**Environment:**
- Go 1.21 (specified in `go.mod`)

**Package Manager:**
- Go Modules (`go.mod`, `go.sum`)
- Lockfile: Present (`go.sum`)

## Frameworks

**Core:**
- Gorilla WebSocket 1.5.1 - WebSocket client for ClawdBot Gateway communication (`internal/clawdbot/client.go`)
- Lark (Feishu) OpenAPI SDK v3 (larksuite/oapi-sdk-go/v3 v3.5.3) - Feishu/Lark integration (`internal/feishu/client.go`)

**Build/Dev:**
- Make - Task automation (`Makefile`)
- Go built-in tools (fmt, vet, test)
- GitHub Actions - CI/CD pipeline (`.github/workflows/release.yml`)

## Key Dependencies

**Critical:**
- `github.com/larksuite/oapi-sdk-go/v3` v3.5.3 - Feishu enterprise messaging platform SDK for WebSocket and REST API
  - Provides event dispatcher, WebSocket client, and IM message management
  - Location: `internal/feishu/client.go` - Used for connecting to Feishu platform, receiving messages, and sending responses

- `github.com/gorilla/websocket` v1.5.1 - WebSocket protocol implementation
  - Location: `internal/clawdbot/client.go` - Used for bidirectional communication with ClawdBot Gateway

- `github.com/google/uuid` v1.6.0 - UUID generation
  - Location: `internal/clawdbot/client.go` - Used for generating idempotency keys in agent requests

**Infrastructure:**
- `golang.org/x/net` v0.17.0 - Networking primitives (indirect dependency)
- `github.com/gogo/protobuf` v1.3.2 - Protocol Buffer support (indirect dependency)

## Configuration

**Environment:**
- Configuration via JSON files in `~/.clawdbot/` directory
- `clawdbot.json` - Managed by ClawdBot, contains gateway port and auth token
- `bridge.json` - Bridge-specific configuration with Feishu credentials and optional parameters

**Build:**
- `Makefile` - Primary build configuration
- `scripts/build.sh` - Multi-platform cross-compilation script
- `.github/workflows/release.yml` - GitHub Actions release workflow

## Platform Requirements

**Development:**
- Go 1.21 or newer
- Make (for development commands)
- Unix-like shell for build scripts

**Production:**
- ClawdBot Gateway service running on localhost:18789 (configurable)
- Feishu enterprise application with valid App ID and App Secret
- Multi-platform support:
  - Linux (amd64, arm64)
  - macOS (amd64, arm64 / Apple Silicon)
  - Windows (amd64, arm64)

**Deployment:**
- GitHub Releases - Pre-compiled binaries distributed via GitHub Releases (`. github/workflows/release.yml`)
- Daemon mode support (Unix: systemd; Windows: separate daemon implementation)

---

*Stack analysis: 2026-01-29*
