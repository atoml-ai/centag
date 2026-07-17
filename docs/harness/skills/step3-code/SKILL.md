---
name: step3-code
description: "工作流 Step 3：SDD 编码实现 — 按 Spec 编码，落实开发风险缓解，测试验证合规。触发场景：编码、step3-code、开始写代码"
---

# Step 3: SDD 编码实现

> 执行指南：`docs/harness/workflow/phase-3-implement.md`

## 触发词

`step3-code` / 编码 / 开始写代码 / step3

## 前置门禁

**Gate 2**：任务计划落盘 + 可执行验收标准 + **开发风险已映射到任务** + Critical 开放 = 0

> ⚠️ 执行前必须先检查 Gate 2，未通过则拒绝继续。

## 执行流程

### 第一步：读取上下文

1. 确认当前要执行的任务
2. 读取任务计划（`docs/versions/<版本>/<需求>/任务计划.md`）
3. **读取开发风险评估**，确认本任务关联的风险 ID 与验证方式
4. 读取相关架构约束（`docs/harness/ARCHITECTURE.md`）
5. 读取编码规范（`docs/harness/CONVENTIONS.md`）
6. 检查反模式（`docs/harness/ANTI-PATTERNS.md`）

### 第二步：编码实现

- 严格依据技术方案实现，不偏离 Spec
- **优先落实本任务关联的 High 风险缓解措施**（如防双记、接口注入避免循环依赖、异步降级等）
- 遵循架构分层：
  - `cmd/` / `dist/` — 仅入口与组装
  - `core/internal/`、`core/pkg/` — 依赖方向向内，不跨层
  - `plugins/` — 与契约保持一致
- 小步修改，避免无关格式化
- 新增接口或配置变更时同步 `docs/api/` 或 `docs/guide/`
- 禁止引用 `archive/deprecated/` 的内容

### 第三步：测试验证与风险关闭

- 实现后运行对应模块测试：`go test ./...`（范围以任务验收为准）
- 按关联风险的「验证方式」执行检查（测试断言 / `rg` / httptest）
- 风险验证通过后，更新 `开发风险评估.md` 对应行状态为 **已关闭（实现验证）**
- 确保不破坏已有测试

### 第四步：循环执行

- 按任务计划逐个任务执行
- 每个任务完成后标记完成
- 发现新的开发风险：追加到 `开发风险评估.md`，Critical 立即停手并升级沟通

## 产出

| 产物 | 路径 |
|------|------|
| 代码 | 对应 `core/` / `plugins/` / `dist/` / `web/` |
| 测试 | 与被测文件同目录 `*_test.go` |
| 风险评估状态更新 | `开发风险评估.md` |

## 完成后

- 更新 `workflow_state.md`：Step 3 状态 → ✅ 已完成
- 提示用户可执行 `step4-test` 补测试或 `step5-review` 进入质量交付。
