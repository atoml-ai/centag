---
name: step5-review
description: "工作流 Step 5：CR 审查 — 自测验证 + 代码审查 + 人工复核 + 产物检查。触发场景：CR、step5-review、代码审查"
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
2. 运行全量测试：`go test ./...`
3. 运行 lint：`make lint`
4. 运行卫生检查：`make harness-check`
5. 验证 API 行为（若有接口变更）：`curl http://localhost:20060/api/...`
6. 按 `docs/harness/templates/自测记录模板.md` 记录自测结果

### 第二步：代码审查

按以下维度审查：

| 维度 | 检查内容 |
|------|---------|
| 架构合规 | 分层正确，无跨层调用 |
| 反模式检测 | 无 `panic` 用于业务错误、无忽略 `err`、无循环依赖 |
| 命名规范 | 符合 `docs/harness/CONVENTIONS.md` |
| 错误处理 | 所有 `err` 已处理，使用 `%w` 包装 |
| 测试覆盖 | 新增代码有对应测试，覆盖边界场景 |

### 第三步：产物检查
_报告模板.md` 输出 CR 报告：

- [ ] 技术方案已落盘
- [ ] 任务计划已落盘
- [ ] 自测记录已落盘
- [ ] CR 报告已落盘（写入 `docs/versions/<版本>/<需求>/CR_报告.md`）
- [ ] CR 报告已落盘
- [ ] 代码 + 测试已就绪

### 第四步：人工复核

- 用户复核代码变更
- 用户确认产物完整性
- 用户确认可交付

## 产出

| 产物 | 路径 |
|------|------|
| 自测记录 | `docs/versions/<版本>/<需求>/自测记录.md` |
| CR 报告 | `docs/versions/<版本>/<需求>/CR_报告.md` |

## 完成后

- 更新 `workflow_state.md`：Step 5 状态 → ✅ 已完成，Gate 3 → ✅，Gate 4 → ✅
- 确认 Gate 4 准出条件全部满足，提示用户可进行合并或部署。
