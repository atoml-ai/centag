---
name: step3-code
description: "工作流 Step 3：SDD 编码实现 — 按 Spec 编码，测试验证合规。触发场景：编码、step3-code、开始写代码"
---

# Step 3: SDD 编码实现

> 执行指南：`docs/harness/workflow/phase-3-implement.md`

## 触发词

`step3-code` / 编码 / 开始写代码 / step3

## 前置门禁

**Gate 2**：任务计划落盘 + 可执行验收标准

> ⚠️ 执行前必须先检查 Gate 2，未通过则拒绝继续。

## 执行流程

### 第一步：读取上下文

1. 确认当前要执行的任务
2. 读取任务计划（`docs/versions/<版本>/<需求>/任务计划.md`）
3. 读取相关架构约束（`docs/harness/ARCHITECTURE.md`）
4. 读取编码规范（`docs/harness/CONVENTIONS.md`）
5. 检查反模式（`docs/harness/ANTI-PATTERNS.md`）

### 第二步：编码实现

- 严格依据技术方案实现，不偏离 Spec
- 遵循架构分层：
  - `cmd/` — 仅入口与组装
  - `internal/` — 依赖方向向内，不跨层
  - `plugins/` — 与 `internal/plugin` 契约保持一致
- 小步修改，避免无关格式化
- 新增接口或配置变更时同步 `docs/api/` 或 `docs/guide/`
- 禁止引用 `archive/deprecated/` 的内容

### 第三步：测试验证

- 实现后运行对应模块测试：`go test ./internal/<模块>/...`
- 验证新增行为是否按预期工作
- 确保不破坏已有测试

### 第四步：循环执行

- 按任务计划逐个任务执行
- 每个任务完成后标记完成

## 产出

| 产物 | 路径 |
|------|------|
| 代码 | 对应 `internal/` / `plugins/` 目录 |
| 测试 | 与被测文件同目录 `*_test.go` |

## 完成后

- 更新 `workflow_state.md`：Step 3 状态 → ✅ 已完成
- 提示用户可执行 `step4-test` 补测试或 `step5-review` 进入质量交付。
