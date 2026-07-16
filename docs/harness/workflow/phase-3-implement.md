# Phase 3: 编码实现（Step 3 + Step 4）

> 快捷别名：`step3-code` / 编码 / 开始写代码 / step3
> 快捷别名：`step4-test` / 补测试 / step4

## 目标

按 Spec 编码实现，并补充完整测试。

## 前置门禁

Gate 2（任务计划落盘 + 可执行验收标准）

> ⚠️ `step3-code` 执行前必须先检查 Gate 2，未通过则拒绝继续。

---

## Step 3: SDD 编码实现

### 执行清单

#### 1. 读取上下文

- [ ] 读取任务计划（`docs/versions/<版本>/<需求>/任务计划.md`）
- [ ] 确认当前要执行的任务
- [ ] 阅读架构约束（`docs/harness/ARCHITECTURE.md`）
- [ ] 阅读编码规范（`docs/harness/CONVENTIONS.md`）
- [ ] 检查反模式（`docs/harness/ANTI-PATTERNS.md`）

#### 2. 编码实现

- [ ] 严格依据技术方案实现，不偏离 Spec
- [ ] 遵循架构分层（`internal/` 依赖方向向内，不跨层）
- [ ] 小步修改，避免无关格式化
- [ ] 新增接口或配置变更时同步 `docs/api/` 或 `docs/guide/`

#### 3. 测试验证

- [ ] 实现后运行对应模块测试：`go test ./internal/<模块>/...`
- [ ] 验证新增行为是否按预期工作
- [ ] 确保不破坏已有测试
- [ ] 更新 `workflow_state.md`：Step 3 → ✅

---

## Step 4: 单元测试补全

### 执行清单

- [ ] 为新增的 Service / Handler / 核心函数编写单元测试
- [ ] 覆盖场景：正常路径、参数校验失败、依赖异常/超时、边界值
- [ ] 使用表驱动测试（Table-Driven Tests）
- [ ] Mock 外部依赖，不依赖外部环境
- [ ] 运行全量测试确保无回归：`go test ./...`
- [ ] 更新 `workflow_state.md`：Step 4 → ✅

### 测试规范

| 规范 | 说明 |
|------|------|
| 文件位置 | 与被测文件同目录，`*_test.go` |
| 函数命名 | `Test<函数名>_<场景描述>` |
| Mock 框架 | 基于 interface 的 Mock 或 gomonkey 打桩 |
| 覆盖率 | 新增代码覆盖率 ≥ 80% |

---

## 产出

| 产物 | 路径 |
|------|------|
| 代码 | 对应 `internal/` 或 `plugins/` 目录 |
| 测试 | 与被测文件同目录 `*_test.go` |

## 完成后

- 更新 `workflow_state.md`：Step 3 → ✅，Step 4 → ✅
- 提示用户可执行 `step5-review` 进入质量交付阶段（将自动检查 Gate 3）
