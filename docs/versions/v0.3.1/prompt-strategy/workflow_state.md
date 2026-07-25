# 工作流状态 — prompt-strategy

> 版本：v0.3.1 | 需求：prompt-strategy | 分支：`feature/v0.3.1`
> 创建日期：2026-07-25 | 最后更新：2026-07-25

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 产物路径 |
|------|------|:----:|:--------:|---------|
| **Phase 1** 方案设计 | Step 1: 方案设计与确认 | ✅ | 2026-07-25 | `docs/versions/v0.3.1/prompt-strategy/技术方案.md` |
| **Phase 2** 任务规划 | Step 2: 任务规划 | ✅ | 2026-07-25 | `docs/versions/v0.3.1/prompt-strategy/任务计划.md` |
| **Phase 3** 编码实现 | Step 3: SDD 编码实现 | ✅ | 2026-07-25 | `core/pkg/pipeline/promptstrategy/` 等 |
| | Step 4: 单元测试补全 | ✅ | 2026-07-25 | `*_test.go`（generator 策略 / user&output ops / 模板契约） |
| **Phase 4** 质量交付 | Step 5: CR 审查 | ✅ | 2026-07-25 | `CR_报告.md`；人工批准可发版 |
| **Phase 5** 发版 | Step 6: 发版 | ⬜ | | GitHub Release `v0.3.1` |

> 状态标记：⬜ = 未开始 | 🔄 = 进行中 | ✅ = 已完成

## 门禁状态

| 门禁 | 位置 | 状态 | 通过日期 | 备注 |
|------|------|:----:|:--------:|------|
| Gate 1 | Phase 1 → 2（技术方案落盘 + 内审通过） | ✅ | 2026-07-25 | 用户要求建分支并进入 step2-plan，视为方案与风险确认 |
| Gate 2 | Phase 2 → 3（任务计划落盘 + 可执行验收标准） | ✅ | 2026-07-25 | 任务计划已确认，Critical=0，High 风险已映射 |
| Gate 3 | Phase 3 → 4（测试通过 + 覆盖率达标） | ✅ | 2026-07-25 | `make test` + promptstrategy 89.9%；web lint ✅；harness-check ✅；本机无 golangci-lint |
| Gate 4 | Phase 4 → 5（CR 准出 + **人工批准可发版**） | ✅ | 2026-07-25 | 人工确认发版许可（选项：批准 — 可发版） |
| Gate 5 | 发版准出（Release 资产 + 冒烟） | ⬜ | | |

## 产物清单

- [x] **技术方案** → `docs/versions/v0.3.1/prompt-strategy/技术方案.md`
- [x] **开发风险评估** → `docs/versions/v0.3.1/prompt-strategy/开发风险评估.md`（Critical=0）
- [x] **前端配置操作说明** → `docs/versions/v0.3.1/prompt-strategy/前端配置操作说明.md`
- [x] **任务计划** → `docs/versions/v0.3.1/prompt-strategy/任务计划.md`
- [x] **自测记录** → `docs/versions/v0.3.1/prompt-strategy/自测记录.md`
- [x] **CR 报告** → `docs/versions/v0.3.1/prompt-strategy/CR_报告.md`
- [x] **代码 + 测试** → `core/pkg/pipeline/promptstrategy/`（任务1已完成）
- [ ] **GitHub Release**（Step 6）→ `v0.3.1`

## 任务进度

| ID | 任务 | 状态 | 完成日期 |
|----|------|:----:|:--------:|
| T1 | System 策略框架与双路径接线（A0） | ✅ | 2026-07-25 |
| T2 | user_prompt_ops 入站节点（A1） | ✅ | 2026-07-25 |
| T3 | output_post_ops 出站后处理节点（A2） | ✅ | 2026-07-25 |
| T4 | 文档与模板标注（A3） | ✅ | 2026-07-25 |
| T5 | Web 配置必做（A4） | ✅ | 2026-07-25 |
| T6 | LLM 预处理/后处理占位（B1） | ✅ | 2026-07-25 |

## T1 完成摘要

**任务1：System 策略框架与双路径接线（A0）**

已完成的工作：
1. 创建 `core/pkg/pipeline/promptstrategy/` 包，实现 `ApplySystemStrategy` 算子
2. 支持 `passthrough` / `append` / `replace` 三种策略模式
3. 实现 `ResolveSystemMode` 兼容映射（`inject_system_prompt` → `system_prompt_strategy`）
4. 改造 `TransparentForwardNode` 使用新算子，支持 `system_prompt_strategy` 和 `append_position` 配置
5. 改造 `GeneratorNode` 使用新算子，流式/非流式路径对齐
6. 扩展测试用例覆盖策略矩阵和兼容性

关键文件：
- `core/pkg/pipeline/promptstrategy/system.go` — 算子实现
- `core/pkg/pipeline/promptstrategy/system_test.go` — 算子测试
- `core/pkg/pipeline/transparent_forward_node.go` — 透明转发节点改造
- `core/pkg/pipeline/builtin_nodes.go` — Generator 节点改造
- `core/pkg/pipeline/transparent_forward_node_test.go` — 新增策略测试

## T2 完成摘要

**任务2：user_prompt_ops 入站节点（A1）**

已完成的工作：
1. 创建 `core/pkg/pipeline/user_prompt_ops_node.go` 实现 `UserPromptOpsNode`
2. 实现规则检查：正则 deny_patterns、密钥形态启发式检测
3. 实现优化：空白折叠、最大长度截断
4. 支持三种处置动作：`log`（记录）、`redact`（脱敏）、`block`（阻断）
5. 支持 raw_request_body 同步（透传路径场景）
6. 注册节点类型 `NodeTypeUserPromptOps`、Schema、Kind 映射
7. 扩展测试用例覆盖策略矩阵和兼容性

关键文件：
- `core/pkg/pipeline/user_prompt_ops_node.go` — 节点实现
- `core/pkg/pipeline/user_prompt_ops_node_test.go` — 节点测试
- `core/pkg/pipeline/node.go` — NodeType 常量
- `core/pkg/pipeline/builtin_nodes.go` — 注册
- `core/pkg/pipeline/builtin_schemas.go` — Schema
- `core/pkg/pipeline/plugin_contract.go` — Kind 映射

## T3 完成摘要

**任务3：output_post_ops 出站后处理节点（A2）**

已完成的工作：
1. 创建 `core/pkg/pipeline/output_post_ops_node.go` 实现 `OutputPostOpsNode`
2. 实现字符串操作链：`trim_space`、`strip_markdown_fence`、`extract_json`、`json_compact`
3. 支持 `on_invalid_json` 配置：`pass`（透传）或 `wrap_error_object`（包装错误）
4. 注册节点类型 `NodeTypeOutputPostOps`、Schema、Kind 映射
5. 扩展测试用例覆盖各操作和组合场景

关键文件：
- `core/pkg/pipeline/output_post_ops_node.go` — 节点实现
- `core/pkg/pipeline/output_post_ops_node_test.go` — 节点测试
- `core/pkg/pipeline/node.go` — NodeType 常量
- `core/pkg/pipeline/builtin_nodes.go` — 注册
- `core/pkg/pipeline/builtin_schemas.go` — Schema
- `core/pkg/pipeline/plugin_contract.go` — Kind 映射

## T4 完成摘要

**任务4：文档与模板标注（A3）**

已完成的工作：
1. 创建 `docs/guide/prompt-strategy.md` 指南文档
2. 更新 `docs/versions/v0.3.1/prompt-strategy/workflow_state.md`

## T5 完成摘要

**任务5：Web 配置必做（A4）**

已完成的工作：
1. 修改 `web/src/components/NodeConfigDrawer.vue`
   - 替换布尔 `inject_system_prompt` 为三选一 `system_prompt_strategy`（passthrough/append/replace）
   - 新增 `append_position` 选择器
   - 新增 `user_prompt_ops` 节点配置表单
   - 新增 `output_post_ops` 节点配置表单
2. 更新 `web/src/locales/zh-CN/nodeConfig.json` 中文文案
3. 更新 `web/src/locales/en/nodeConfig.json` 英文文案

## T6 完成摘要

**任务6：LLM 预处理/后处理占位（B1）**

已完成的工作：
1. 创建 `core/pkg/pipeline/promptstrategy/llm_placeholder.go`
   - 定义 `PromptLLMProcessor` 接口
   - 实现 `NullPromptLLMProcessor` 空处理器
   - 定义 `LLMConfig` 配置结构

## 决策日志

| 日期 | 决策 | 原因 | 提出人 |
|------|------|------|--------|
| 2026-07-25 | 版本 v0.3.1 / 需求名 prompt-strategy | 用户确认 | 用户 |
| 2026-07-25 | 覆盖 system / user / 输出后处理；分阶段（框架+基础能力优先，预留专用 LLM） | 用户确认 | 用户 |
| 2026-07-25 | 补前端配置操作说明（画布 + 抽屉示意） | 方案前端缺口 | 用户/AI |
| 2026-07-25 | Web 配置定为 Phase A **必做**（验收入口）；路线增 A4 | 用户确认：无 Web 无法测试 | 用户 |
| 2026-07-25 | 不区分流水线；凡涉及 system/user 设置与替换均可配置策略 | 需求核心 | 用户 |
