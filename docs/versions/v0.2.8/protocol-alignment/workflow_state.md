# 工作流状态 — 协议对齐与 Models 接口增强

> 版本：v0.2.8 | 需求：协议对齐与 Models 接口增强 | 分支：feature/v0.2.8
> 创建日期：2026-07-22 | 最后更新：2026-07-22

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 产物路径 |
|------|------|:----:|:--------:|---------|
| **Phase 1** 方案设计 | Step 1: 方案设计与确认 | ✅ | 2026-07-22 | `docs/versions/v0.2.8/protocol-alignment/技术方案.md` |
| **Phase 2** 任务规划 | Step 2: 任务规划 | ✅ | 2026-07-22 | `docs/versions/v0.2.8/protocol-alignment/任务计划.md` |
| **Phase 3** 编码实现 | Step 3: SDD 编码实现 | ✅ | 2026-07-22 | 对应 `core/` / `plugins/` 目录 |
| | Step 4: 单元测试补全 | ✅ | 2026-07-22 | 对应 `*_test.go` |
| **Phase 4** 质量交付 | Step 5: CR 审查 | ✅ | 2026-07-22 | `docs/versions/v0.2.8/protocol-alignment/` |
| **Phase 5** 发版 | Step 6: 发版 | ✅ | 2026-07-22 | GitHub Release `v0.2.8` |

> 状态标记：⬜ = 未开始 | 🔄 = 进行中 | ✅ = 已完成

## 门禁状态

| 门禁 | 位置 | 状态 | 通过日期 | 备注 |
|------|------|:----:|:--------:|------|
| Gate 1 | Phase 1 → 2（技术方案落盘 + 内审通过） | ✅ | 2026-07-22 | Critical=0；High 均有应对；8 项 gap 已回写技术方案，L1-L4/S1/S2 已闭合 |
| Gate 2 | Phase 2 → 3（任务计划落盘 + 可执行验收标准） | ✅ | 2026-07-22 | 13 项任务全映射风险；核对修正 4 处计划缺陷（P1 可改文件缺 2 调用点 / P2 plugin 缺 ToolDefinition / P3 ChatCompletionRequest 缺 Tools / P9 方案 B 定调）|
| Gate 3 | Phase 3 → 4（测试通过 + 覆盖率达标） | ✅ | 2026-07-22 | 全量测试通过，lint 无警告 |
| Gate 4 | Phase 4 → 5（CR 准出 + **人工批准可发版**） | ✅ | 2026-07-22 | 人工确认发版许可 |
| Gate 5 | 发版准出（Release 资产 + 冒烟） | ✅ | 2026-07-22 | 资产齐全，安装验收：用户手动 |

## 产物清单

- [x] **技术方案** → `docs/versions/v0.2.8/protocol-alignment/技术方案.md`
- [x] **开发风险评估** → `docs/versions/v0.2.8/protocol-alignment/开发风险评估.md`
- [x] **验证测试方案** → `docs/versions/v0.2.8/protocol-alignment/验证测试方案.md`（Step1 验证充分性补全：含设计复核 8 项 gap + 用例矩阵 + Gate3 命令）
- [x] **任务计划** → `docs/versions/v0.2.8/protocol-alignment/任务计划.md`
- [ ] **自测记录** → `docs/versions/v0.2.8/protocol-alignment/自测记录.md`
- [ ] **CR 报告** → `docs/versions/v0.2.8/protocol-alignment/CR_报告.md`（结论含是否批准发版）
- [ ] **代码 + 测试** → 对应 `core/` / `plugins/` / `web/`
- [ ] **GitHub Release**（Step 6）→ `v0.2.8`

## 决策日志

| 日期 | 决策 | 原因 | 提出人 |
|------|------|------|--------|
| 2026-07-22 | 协议对齐范围限定为 OpenAI Chat Completions / Anthropic Messages，不包含 Responses API 深度改造 | Responses API 有独立协议插件且为 build tag 可选，本轮聚焦最常用路径 | AI |
| 2026-07-22 | Models 接口同时输出后端模型 + Pipeline ID，保持 `pipeline.` 前缀作为模型名 | 兼容已有第三方 agent 使用习惯，同时暴露后端能力 | AI |
| 2026-07-22 | `response_format` 和 `tool_choice` 标记为 P0，因大量第三方 agent 依赖 | 多个 agent 测试验证报错的核心原因 | AI |
| 2026-07-22 | 对照源码发现 8 项设计 gap（G1 Anthropic RawBody 非原始 map / G2 Anthropic error 字段为字符串非对象 / G3 tool_use_id 读取错位 / G4 缓存 key 未纳 P0 字段 / G5 OpenAI usage total==completion 错算 / G6 流式 usage 条件偏窄 / G7 P2 字段提升与"暂不实装"自相矛盾 / G8 generateID 恒定）| 影响"协议可靠性/兼容性"验证，须在 Gate1 回写技术方案或在任务计划落地 | AI |

---

## 使用说明

### 何时更新

- 每个 Step 完成时：更新对应步骤状态为 ✅，记录完成日期
- 进入新 Step 时：更新当前步骤状态为 🔄
- 通过门禁时：更新门禁状态为 ✅，记录通过日期
- **Gate 4**：仅在 Step 5 AskQuestion 得到「批准 — 可发版」后标记 ✅
- 有重要决策时：追加到决策日志
