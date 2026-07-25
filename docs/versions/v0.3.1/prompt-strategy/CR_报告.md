# CR 报告 — prompt-strategy

> 日期：2026-07-25 | 审查人：AI（待人工 Gate 4）  
> 审查范围：`feature/v0.3.1` 相对已推送基线的 prompt-strategy 改动（含 CR 中修复的 body 保真）

## 门禁检查报告 — Gate 3

| 项 | 结果 | 说明 |
|----|:----:|------|
| G3.1 全量单元测试 | ✅ | `make test`（`scripts/ci-go-packages.sh`）通过。裸 `go test ./...` 会因 plugin build tags 在 `cmd/centag` setup failed，**以 `make test` 为准** |
| G3.2 覆盖率 ≥ 80% | ✅ | `go test ./core/pkg/pipeline/promptstrategy/ -cover` → **89.9%**；pipeline 相关专项用例绿 |
| G3.3 lint 无新增 | ⚠️ | `web` `npm run lint:ci` ✅（0 warnings）；本机未安装 `golangci-lint`，未能跑 `make lint` |
| G3.4 harness-check | ✅ | 已将既有 `apps/centag-npm` 加入白名单后 `make harness-check` OK |

**Gate 3 结论：通过**（lint 附条件：本机缺 golangci-lint；CI 侧应仍跑 lint）。

## 审查维度

| 维度 | 结果 | 说明 |
|------|:----:|------|
| 架构合规 | ✅ | 策略仅在 `core/pkg/pipeline`；未写入 hooks / proxy handler |
| 反模式检测 | ✅ | 无业务 panic；旧 `injectSystemPromptIntoChatBody` 已薄封装转调 `ApplySystemStrategy` |
| 命名规范 | ✅ | 符合 CONVENTIONS |
| 错误处理 | ✅ | body 同步失败 fail-open；block 错误不含原文 |
| 测试覆盖 | ✅ | 算子 / 双路径 / user&output ops / 模板契约 / tool_calls 保真 |
| 风险闭环 | ✅ | High 均有实现验证证据；Critical=0（见下） |

## Critical 问题

| # | 问题描述 | 位置 | 状态 |
|---|---------|------|------|
| 1 | `#d`/replace\|append 经 Parse→Sync 重建 messages，丢失 `tool_calls` / 多模态 content | `promptstrategy/system.go` Sync 路径 | **已修复**：raw body 在原 map 上改 system；单测 `TestApplySystemStrategy_RawBodyPreservesToolCallsAndMultimodal`、`TestTransparentForwardNode_LegacyInjectSystemPrompt` |

## CR 中顺带修复

1. raw body system 策略保真（Critical）  
2. 删除 `transparent_forward` 不可达的平行 inject 分支；旧函数薄封装  
3. `scripts/check-harness-hygiene.sh` 允许 `apps/centag-npm`（发版 npm 壳，既有漏网）

## 产物完整性

- [x] 技术方案已落盘
- [x] 开发风险评估已落盘且无开放 Critical
- [x] 任务计划已落盘
- [x] 自测记录已落盘
- [x] CR 报告已落盘（本文件）
- [x] 代码 + 测试已就绪

## 残余 / 手工项

| 项 | 说明 |
|----|------|
| 前端 §10 五步 | 需人工浏览器联调勾选（R13） |
| `make lint` | 本机无 golangci-lint；请以 CI 为准 |
| `stream_mode=buffer` 未设 max | 默认 skip；显式 buffer 时建议配置 `max_buffer_bytes` |

## 结论

- [x] **批准 — 可发版** — Gate 4 通过；允许执行 `step6-release`
- [ ] **需修改后重审** — Gate 4 保持未通过
- [ ] **拒绝** — 禁止发版

> 2026-07-25 人工选择「批准 — 可发版」；`workflow_state` Gate 4 = ✅。
