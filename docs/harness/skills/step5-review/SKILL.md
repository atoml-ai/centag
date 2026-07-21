---
name: step5-review
description: "工作流 Step 5：CR 审查 — 自测验证 + 代码审查 + 人工确认 Gate 4（发版许可）。触发场景：CR、step5-review、代码审查"
---

# Step 5: CR 审查

> 执行指南：`docs/harness/workflow/phase-4-deliver.md`

## 触发词

`step5-review` / CR / 代码审查 / 自测 / step5

## 前置门禁

**Gate 3**：全量测试通过 + 覆盖率达标 + lint 无新增

> ⚠️ 执行前必须先检查 Gate 3，未通过则拒绝继续。

## 执行流程

### 第一步：自测验证

1. 对照任务计划的验收标准，逐项验证
2. 对照 `开发风险评估.md`：Critical/High 应为「已关闭（实现验证）」或明确可接受残余
3. 运行全量测试：`go test ./...`
4. 运行 lint：`make lint`
5. 运行卫生检查：`make harness-check`
6. 验证 API 行为（若有接口变更）：`curl http://localhost:20060/api/...`
7. 按 `docs/harness/templates/自测记录模板.md` 记录自测结果

### 第二步：代码审查

按以下维度审查：

| 维度 | 检查内容 |
|------|---------|
| 架构合规 | 分层正确，无跨层调用 |
| 反模式检测 | 无 `panic` 用于业务错误、无忽略 `err`、无循环依赖 |
| 命名规范 | 符合 `docs/harness/CONVENTIONS.md` |
| 错误处理 | 所有 `err` 已处理，使用 `%w` 包装 |
| 测试覆盖 | 新增代码有对应测试，覆盖边界场景 |
| 风险闭环 | High/Critical 缓解措施已落地且有验证证据 |

### 第三步：产物检查

按 `docs/harness/templates/CR_报告模板.md` 输出 CR 报告：

- [ ] 技术方案已落盘
- [ ] 开发风险评估已落盘且无开放 Critical
- [ ] 任务计划已落盘
- [ ] 自测记录已落盘
- [ ] CR 报告已落盘（写入 `docs/versions/<版本>/<需求>/CR_报告.md`）
- [ ] 代码 + 测试已就绪

### 第四步：人工复核 → **Gate 4（发版许可）**

> ⚠️ **禁止**在未获人工确认时将 Gate 4 标为 ✅。写完 CR ≠ 通过 Gate 4。

1. 用户复核代码变更与产物完整性。
2. **必须**用 AskQuestion 征求结论（不可用时极短文字提问）：

| id | prompt | options |
|----|--------|---------|
| `cr_gate4` | CR 结论 / Gate 4 发版许可 | 批准 — 可发版 / 需修改后重审 / 拒绝 |

3. 按选项更新状态：

| 选项 | 动作 |
|------|------|
| **批准 — 可发版** | CR 结论勾选批准；`workflow_state`：Step 5 → ✅，Gate 3 → ✅，**Gate 4 → ✅**（备注：人工确认发版许可）；提示可执行 `step6-release` |
| **需修改后重审** | Gate 4 保持 ⬜；Step 5 可为 🔄；列出待修项 |
| **拒绝** | Gate 4 保持 ⬜；CR 结论勾选拒绝；不得进入 Step 6 |

4. 仅「批准」时提示下一步：`step6-release`（发版）。合并 `main` 可与发版并行，但**上传 Release 必须以 Gate 4 为准**。

## 产出

| 产物 | 路径 |
|------|------|
| 自测记录 | `docs/versions/<版本>/<需求>/自测记录.md` |
| CR 报告 | `docs/versions/<版本>/<需求>/CR_报告.md` |
| 风险评估终态 | `docs/versions/<版本>/<需求>/开发风险评估.md`（状态已更新） |
| Gate 4 状态 | `workflow_state.md`（仅人工批准后 ✅） |

## 完成后

- Step 5 文档与审查完成 → Step 5 可标 ✅
- **Gate 4 仅在人工选择「批准 — 可发版」后**标 ✅
- 批准后提示用户执行 `step6-release`；未批准则禁止发版
