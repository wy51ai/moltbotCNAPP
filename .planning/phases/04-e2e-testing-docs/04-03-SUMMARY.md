---
phase: 04
plan: 03
subsystem: documentation
completed: 2026-01-29
duration: 3m
tags: [docs, webhook, readme, faq, monitoring]
requires:
  - "04-01"  # Webhook test coverage
  - "04-02"  # Webhook observability
provides:
  - "Webhook 模式完整文档"
  - "飞书后台配置指南"
  - "ngrok 本地验收 runbook"
  - "常见问题 FAQ"
  - "监控指标说明"
affects:
  - "用户能够自助配置 Webhook 模式"
  - "运维能够通过文档排查问题"
tech-stack:
  added: []
  patterns:
    - "User-facing documentation"
    - "Configuration reference"
    - "Troubleshooting guide"
key-files:
  created: []
  modified:
    - "README.md"
decisions:
  - id: "event_id-deduplication"
    decision: "Use event_id instead of message_id for deduplication"
    rationale: "Feishu retry mechanism generates new event_id for same message_id, preventing message loss"
    alternatives: "message_id (would cause dropped messages on retry)"
    codex-requested: true
---

# Phase 04 Plan 03: Webhook Mode Documentation Summary

Webhook 模式完整文档：配置字段、飞书后台配置、ngrok 验收、FAQ 和监控指标

## What Was Done

### Task 1: Webhook Configuration Section
**Commit:** a5ad362

Added comprehensive Webhook mode configuration documentation:
- **适用场景说明**：公网访问生产环境，对比 WebSocket 优势
- **配置字段表格**：7 个字段（mode, port, path, verification_token, encrypt_key, workers, queue_size）
- **完整 JSON 配置示例**：完整的 `bridge.json` 示例
- **启动命令**：2 种方式（CLI 参数 + 手动编辑配置）

### Task 2: Feishu Backend Configuration Guide
**Commit:** 21d1c1b

Added Feishu open platform configuration guide (5 steps):
1. **获取应用凭据**：App ID 和 App Secret
2. **配置事件订阅**：Webhook URL、Verification Token、Encrypt Key
3. **添加事件订阅**：im.message.receive_v1
4. **配置权限**：4 个必需权限（im:message, group_at_msg 等）
5. **发布应用**：版本管理、审核、发布流程

### Task 3: FAQ and ngrok Verification Guide
**Commit:** 3ef0504

Added comprehensive troubleshooting and verification documentation:

**ngrok 验收指南：**
- 安装说明（macOS, Linux, Windows）
- 使用步骤（启动 bridge + ngrok）
- 5 步验收流程（配置 URL → 验证 → 测试消息 → 检查日志 → 验证响应）

**FAQ（4 个问答）：**
- Q1: 启动报错缺少 verification_token（配置缺失）
- Q2: 飞书验证 Webhook URL 失败（4 种可能原因 + 排查步骤）
- Q3: 收到消息但没有响应（4 项排查清单）
- **Q4: 为什么使用 event_id 而不是 message_id 去重**（Codex 评审要求，详细场景示例）

**监控指标表格：**
- 6 个 Prometheus 指标（requests_total, duration, queue_depth, signature_failures 等）
- 使用示例（curl 命令查询指标）

## Technical Implementation

### Documentation Structure

```
README.md
├── Webhook 模式
│   ├── 适用场景
│   ├── 配置字段（表格）
│   ├── 完整配置示例（JSON）
│   ├── 启动 Webhook 模式
│   ├── 飞书后台配置指南（5 步）
│   ├── ngrok 本地验收指南
│   │   ├── 安装 ngrok
│   │   ├── 使用 ngrok
│   │   └── 验收步骤（5 步）
│   ├── 常见问题 FAQ（4 个 Q&A）
│   └── 监控指标（Prometheus 表格）
```

### Key Content Highlights

**Configuration Reference:**
- 所有字段含默认值和必填标识
- 完整 JSON 配置示例可直接复制使用
- CLI 参数和配置文件两种方式说明

**Troubleshooting Guide:**
- 每个 FAQ 包含原因分析 + 解决方案
- Q2 提供 4 步排查脚本（status → logs → curl test → ngrok check）
- Q3 提供结构化排查清单（队列 → worker → gateway → 权限）

**Deduplication Rationale (Codex Requirement):**
- 清晰说明 event_id vs message_id 语义差异
- 3 步场景示例演示重试机制
- 结论：event_id 确保"每次投递都处理一次"

## Verification Results

✅ Success Criteria Met:
- [x] Webhook 模式章节存在，含配置表格
- [x] 完整 JSON 配置示例正确（包含所有 Webhook 字段）
- [x] 飞书后台配置指南完整（5 步骤）
- [x] FAQ 至少 4 个问答（包含 4 个）
- [x] event_id 去重原因有说明（Q4 详细阐述，Codex 评审要求）
- [x] ngrok 验收步骤清晰（5 步操作流程）
- [x] 监控指标表格完整（6 个 Prometheus 指标）

## Deviations from Plan

None - plan executed exactly as written.

## Decisions Made

### Event ID Deduplication Documentation
**Decision:** Documented why event_id is used instead of message_id for deduplication
**Context:** Codex 评审 explicitly requested clarification on deduplication mechanism
**Rationale:**
- event_id = Feishu event delivery unique ID (identifies "this delivery attempt")
- message_id = Message content ID (same across retries)
- Using message_id would drop retried events, causing message loss
**Impact:** Users understand the reliability guarantee

## Next Phase Readiness

### Blockers
None.

### Concerns
None - documentation complete and comprehensive.

### Outstanding Items
None - all Webhook mode documentation requirements met.

## Metrics

- **Tasks completed:** 3/3
- **Commits:** 3 (1 per task, atomic)
- **Files modified:** 1 (README.md)
- **Lines added:** ~229
- **Documentation sections:** 4 major sections (Config, Feishu Guide, ngrok, FAQ)
- **FAQ entries:** 4 (Q1-Q4)
- **Configuration fields documented:** 7
- **Prometheus metrics documented:** 6

## Files Modified

### README.md
**Purpose:** User-facing documentation for Webhook mode

**Changes:**
- Added "## Webhook 模式" section with 4 subsections
- Configuration fields table (7 fields)
- Complete JSON configuration example
- Feishu backend configuration guide (5 steps)
- ngrok local verification guide
- FAQ (4 Q&A)
- Monitoring metrics table (6 Prometheus metrics)

**Key Features:**
- Self-service configuration guide
- Troubleshooting runbook
- Production-ready monitoring documentation
- event_id deduplication rationale (Codex requirement)

## Testing Evidence

### Manual Verification
- ✅ README.md structure validated (4 major sections)
- ✅ Configuration table accuracy verified (7 fields, all correct defaults)
- ✅ JSON example syntax validated (valid JSON, all required fields present)
- ✅ Feishu configuration steps verified (5 steps, correct sequence)
- ✅ FAQ coverage validated (4 common issues addressed)
- ✅ Metrics table completeness verified (6 metrics, types correct)

## Lessons Learned

### What Went Well
- **Structured approach:** Breaking docs into 3 atomic tasks kept focus clear
- **Completeness:** All must_haves and success criteria addressed
- **User-centric:** FAQ covers actual user pain points (startup errors, validation failures)

### Improvements for Future Plans
- None - documentation workflow worked smoothly

## Related Plans

- **Depends on:** 04-01 (Webhook test coverage), 04-02 (Webhook observability)
- **Enables:** User self-service Webhook configuration, production deployment readiness

---

**Plan Status:** ✅ COMPLETE
**Duration:** ~3 minutes
**Task Commits:** a5ad362, 21d1c1b, 3ef0504
