# 工作流状态 — 商业化分层（插件式 Open Core）

> 版本：v0.2.7 | 需求：commercialization-layered | 分支：v0.2.7  
> 最后更新：2026-07-20

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 备注 |
|------|------|:----:|:--------:|------|
| Phase 1 | Step 1 方案设计 | ✅ | 2026-07-20 | 插件态方案（删 dist/team） |
| Phase 2 | Step 2 任务规划 | ✅ | 2026-07-20 | E0–E4 |
| Phase 3 | Step 3 编码 | 🔄 | 2026-07-20 | **E0/E1 完成**；E2/E3/E4 待做 |
| | Step 4 单测补全 | ⬜ | | |
| Phase 4 | Step 5 CR | ⬜ | | |

## 门禁

| 门禁 | 状态 | 备注 |
|------|:----:|------|
| Gate 1 | ✅ | |
| Gate 2 | ✅ | 用户确认插件态 + 删 dist/team |
| Gate 3 | ⬜ | E2+ 与测试后 |
| Gate 4 | ⬜ | |

## 产物

- [x] 技术方案 / 风险评估 / 任务计划
- [x] E0：`core/pkg/extension`
- [x] E1：pro `cmd/centag-team`；开源无 `dist/team`；`start.sh build team` 转调
- [ ] E2 后端迁出 / E3 前端 pack / E4 CI
- [ ] 自测记录 / CR

## 决策日志

| 日期 | 决策 | 提出人 |
|------|------|--------|
| 2026-07-20 | 闭源仓名 `centag-pro` | 用户 |
| 2026-07-20 | 仅 team 构建依赖 pro；personal/minimal 自立 | 用户 |
| 2026-07-20 | **插件式**扩展；Team main 在 pro；**开源删除 dist/team** | 用户 |
| 2026-07-20 | 前后端 team 均走 pro（前端 E3） | 用户 |
| 2026-07-20 | **centag-pro 与 centag 分支同名同步**（当前均为 `v0.2.7`） | 用户 |
