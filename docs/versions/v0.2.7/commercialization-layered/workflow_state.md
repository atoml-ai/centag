# 工作流状态 — 商业化分层（插件式 Open Core）

> 版本：v0.2.7 | 需求：commercialization-layered | 分支：feature/v0.2.7  
> 最后更新：2026-07-21

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 备注 |
|------|------|:----:|:--------:|------|
| Phase 1 | Step 1 方案设计 | ✅ | 2026-07-20 | 插件态方案（删 dist/team） |
| Phase 2 | Step 2 任务规划 | ✅ | 2026-07-21 | D1–D6；E2R 纠正落点 |
| Phase 3 | Step 3 编码 | ✅ | 2026-07-21 | E0–E4 + E2R（D6）完成 |
| | Step 4 单测补全 | ⬜ | | |
| Phase 4 | Step 5 CR | ⬜ | | |

## 门禁

| 门禁 | 状态 | 备注 |
|------|:----:|------|
| Gate 1 | ✅ | |
| Gate 2 | ✅ | D1–D5（2026-07-21）；D6 用户纠正实现进 pro |
| Gate 3 | ⬜ | E2+ 与测试后 |
| Gate 4 | ⬜ | |

## 产物

- [x] 技术方案 / 风险评估 / 任务计划
- [x] E0：`core/pkg/extension`
- [x] E1：pro `cmd/centag-team`；开源无 `dist/team`；OSS 拒绝 `build/debug team`
- [x] E2.0：Host 白名单扩展 + `RuntimeHost` + Server 接线
- [x] E2.1–E2.4：Admin users/keys/tenants/usage + BillingHook（**实现在 pro**）
- [x] E2R：Host 原语 facade；`centag-pro/internal/teamadmin`；开源删除 `pkg/teamadmin`；R14 关闭
- [x] E2.5：`/system/update*` + `/admin/ab-eval*` 仅 pro；PersistABEval 留开源
- [x] E2.6：verify.sh + license 产品路由冒烟
- [x] E2.7：R09 源码/空 Host 测试 + guide
- [x] E2.8：文档与 `rg` 无私有边验收
- [x] E3：`centag-pro/web/packs/team` + D1 `/cost`；`build-web-team.sh`
- [x] E4：OSS/pro CI workflow；`ci-check-no-private-deps.sh`；`plugins/example`
- [ ] 自测记录 / CR

## 决策日志

| 日期 | 决策 | 提出人 |
|------|------|--------|
| 2026-07-20 | 闭源仓名 `centag-pro` | 用户 |
| 2026-07-20 | 仅 team 构建依赖 pro；personal/minimal 自立 | 用户 |
| 2026-07-20 | **插件式**扩展；Team main 在 pro；**开源删除 dist/team** | 用户 |
| 2026-07-20 | 前后端 team 均走 pro（前端 E3） | 用户 |
| 2026-07-20 | **centag-pro 与 centag 分支同名同步**（当前均为 `feature/v0.2.7`） | 用户 |
| 2026-07-21 | E2 **先拆任务再编码**；计划默认 D1–D5 | 用户 |
| 2026-07-21 | **确认计划 + D1–D5，开始 E2.0** | 用户 |
| 2026-07-21 | **取消开源 `start.sh build team` 转调**；Team 仅在 centag-pro 构建 | 用户 |
| 2026-07-21 | **D6**：Team 产品实现必须在闭源 pro；禁止开源 `teamadmin` 落盘（纠正 E2.1–E2.4） | 用户 |
