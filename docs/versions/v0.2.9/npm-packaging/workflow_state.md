# 工作流状态 — npm 打包与安装文档

> 版本：v0.2.9 | 需求：npm 打包修复与安装文档 | 分支：feature/v0.2.9
> 创建日期：2026-07-23 | 最后更新：2026-07-23

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 产物路径 |
|------|------|:----:|:--------:|---------|
| **Phase 5** 发版 | Step 6: 发版 | ✅ | 2026-07-23 | GitHub Release `v0.2.9` + npm `@atomlai/centag@0.2.9` |

> 状态标记：⬜ = 未开始 | 🔄 = 进行中 | ✅ = 已完成

## 门禁状态

| 门禁 | 位置 | 状态 | 通过日期 | 备注 |
|------|------|:----:|:--------:|------|
| Gate 4 | Phase 4 → 5 | ✅ | 2026-07-23 | 继承 v0.2.8 CR 批准；本版本为 npm/文档补丁 |
| Gate 5 | 发版准出 | ✅ | 2026-07-23 | GitHub v0.2.9 + npm 0.2.9；二进制与 v0.2.8 相同 |

## 产物清单

- [x] **npm 包** → `@atomlai/centag@0.2.9`、`@atomlai/centag-offline@0.2.9`
- [x] **发布脚本修复** → `scripts/publish-centag-npm.sh`（`install.js`、token、ls glob）
- [x] **安装文档** → `README.md`、`apps/centag-npm/README.md`
- [x] **GitHub Release** → `v0.2.9`

## 决策日志

| 日期 | 决策 | 原因 | 提出人 |
|------|------|------|--------|
| 2026-07-23 | npm 包名改为 `@atomlai/centag` / `@atomlai/centag-offline` | npm 拒绝无 scope 名 `centag`（与 `etag` 过于相似） | AI |
| 2026-07-23 | 补丁版本 0.2.9 | 0.2.8 npm 包缺 `install.js` 且不可覆盖 republish | AI |
| 2026-07-23 | GitHub v0.2.9 复用 v0.2.8 二进制 | 无业务代码变更，仅打包/文档修复 | AI |
