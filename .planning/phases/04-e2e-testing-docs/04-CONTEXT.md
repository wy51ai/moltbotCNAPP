# Phase 4: 端到端测试和文档 - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

完成测试覆盖和用户文档，确保 Webhook 功能可用且用户能自助配置。包括单元测试、集成测试、可观测性日志、ngrok 真实环境验证、README 更新。

</domain>

<decisions>
## Implementation Decisions

### 测试策略
- 分层测试：单元测试 mock + 集成测试用真实 SDK
- 覆盖范围：核心路径 + 边界场景
  - 核心：Challenge、签名验证、消息解密、队列满/503
  - 边界：重复事件、无效 JSON、超时场景
- 测试文件放独立 test/ 目录
- 集成测试使用 build tag 隔离，需要飞书测试应用凭证

### 文档结构
- Webhook 配置说明放在 README 新增章节（不是独立文档）
- 配置示例用表格说明字段 + 完整 JSON 示例
- 飞书后台配置步骤需要截图

### 日志规范
- 继续使用标准库 log（保持一致）
- 输出格式：文本格式（可读性优先）

### Claude's Discretion
- 常见问题排查放 README 末尾还是独立文件（根据内容量判断）
- Webhook 请求日志字段（根据调试需要判断）
- 签名验证失败日志级别（根据安全策略判断）
- ngrok 测试流程是否写入文档
- 真实环境验收标准（关键路径判断）
- 验证是否需要自动化脚本
- 飞书测试凭证配置方式（环境变量 vs 配置文件）

</decisions>

<specifics>
## Specific Ideas

- 用户选择分层测试，说明重视测试质量
- 测试文件独立目录（test/）而非 Go 标准的同目录 _test.go
- 文档要有截图，说明目标用户可能不熟悉飞书后台

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-e2e-testing-docs*
*Context gathered: 2026-01-29*
