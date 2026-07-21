# 工作流状态 — 发版仅限版本分支

> 版本：v0.2.7 | 需求：release-version-branch | 分支：feature/v0.2.7  
> 最后更新：2026-07-21

## 状态概览

| 阶段 | 步骤 | 状态 | 完成日期 | 备注 |
|------|------|:----:|:--------:|------|
| Phase 1 | Step 1 方案设计 | ✅ | 2026-07-21 | 版本分支门禁 |
| Phase 2 | Step 2 任务规划 | ✅ | 2026-07-21 | T1–T3 |
| Phase 3 | Step 3 编码 | ✅ | 2026-07-21 | bc6840c 已实现 |
| | Step 4 单测补全 | ✅ | 2026-07-21 | require-release-branch_test.sh（15 case） |
| Phase 4 | Step 5 CR | ✅ | 2026-07-21 | 自测记录 + CR；人工已批准发版 |
| Phase 5 | Step 6 发版 | ✅ | 2026-07-21 | wrap 重命名重发；tag→5d7000c；见 `发版验收.md` |

## 门禁

| 门禁 | 状态 | 备注 |
|------|:----:|------|
| Gate 1 | ✅ | |
| Gate 2 | ✅ | |
| Gate 3 | ✅ | shell 测试 + `make test` + lint；harness 既有路径告警不阻塞 |
| Gate 4 | ✅ | 2026-07-21：人工确认「批准 — 可发版」 |
| Gate 5 | ✅ | 公开 Release 13 资产（personal+wrap）；安装验收用户手动 |

## 产物

- [x] 技术方案 / 风险评估 / 任务计划
- [x] `require-release-branch.sh` + CI/publish 接线
- [x] shell 测试 `require-release-branch_test.sh`
- [x] 自测记录 / CR（已批准可发版）
- [x] Step 6 Release + `发版验收.md`

## 决策日志

| 日期 | 决策 | 提出人 |
|------|------|--------|
| 2026-07-21 | 发版门禁从 main-only 改为版本分支 | 用户 |
| 2026-07-21 | 对本分支执行 step4-test + step5-cr（范围：本需求相对 main 的未合入变更） | 用户 |
| 2026-07-21 | 发版并入标准流程 Step 6；Gate 4 须人工确认后方可 `step6-release` | 用户 |
| 2026-07-21 | Gate 4：批准 — 可发版 | 用户 |
| 2026-07-21 | Gate 5 准出：以 GitHub 资产校验为准；私有仓 `install.sh` 匿名 404 记残余 | 用户/验收 |
| 2026-07-21 | 仓改 public 后重做 Step 6：重建产物 + Publish v0.2.7 | 用户 |
| 2026-07-21 | 修复 publish-binaries 解析 OUT_DIR（tail -n1） | Agent |
| 2026-07-21 | Step 6 默认跳过安装冒烟；用户手动部署 | 用户 |
| 2026-07-21 | 重发 v0.2.7：wrap 资产 + 删 proxyctl + tag→5d7000c | 用户 |
