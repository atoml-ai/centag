# CR 报告 — 本机系统代理出口

> 日期：2026-07-19 | 审查人：AI（待人工复核）  
> 审查范围：`feature/v0.2.5` vs `main`（未提交工作区：`apps/proxyctl`、system_proxy、SystemProxy.vue 等）  
> 关联：`技术方案.md` / `任务计划.md` / `开发风险评估.md` / `自测记录.md`

## 门禁检查报告 — Gate 3

| 检查项 | 状态 | 证据 |
|--------|------|------|
| G3.1 全量单元测试通过 | ⚠️ 附条件通过 | **本需求相关包全部通过**（见下）。`cd core && go test ./...` 存在**既有**失败 `TestModelPriceTable`（`pkg/scheduler`，价格表期望值过期）；在 stash 清空本需求改动后复现，**非本需求引入** |
| G3.2 覆盖率达标 | ✅ | 新增 `system_proxy.go` 函数覆盖约 88%–100%；`proxyctl/engine` **81.1%**；`GetSetupStatus` 100% |
| G3.3 lint 无新增 | ✅ / ⚠️ | `web` `npm run lint:ci` ✅；`make harness-check` ✅（已允许 `apps/proxyctl`）；本机未安装 `golangci-lint`，未能跑 `make lint` |
| G3.4 人工检查点 | ⚠️ | 自测记录中本机三平台 / 双机 Team **手工项未勾**；需用户确认 |

**结论**：Gate 3 **附条件通过**（本需求自动化证据齐全；仓库既有 scheduler 单测债 + 手工 OS 验收待用户确认）。允许进入 Step 5 产物审查，**不建议在未确认手工项前合并生产**。

---

## 审查维度

| 维度 | 结果 | 说明 |
|------|------|------|
| 架构合规 | ✅ | `proxyctl` 独立 `go.mod`、不入 `go.work`、不 import core；MITM/PAC 变更在 `config`/`server`/`handler`；Web 仅管理面 |
| 反模式检测 | ✅ | CLI 子命令白名单；无业务 `panic`；LAN 需显式开关；默认 loopback |
| 命名规范 | ✅ | 与现有 `SystemProxy` / API 风格一致 |
| 错误处理 | ✅ | enable 失败事务回滚；远端 disable 不关服务器；API Validate 返回 400 |
| 测试覆盖 | ✅ | mock OS + httptest 覆盖 local/remote/回滚；PAC advertise 断言 |
| 风险闭环 | ⚠️ | Critical=0；R03/R05 已关闭；R01/R04/R09 依赖真人 OS 勾选；R06/R07 Medium 文档化可接受 |

## Critical 问题

| # | 问题描述 | 位置 | 状态 |
|---|---------|------|------|
| — | 无 | — | — |

## High / 应关注（非阻断）

| # | 问题 | 位置 | 建议 |
|---|------|------|------|
| H1 | CA 卸载主要按 CN「Centag CA」，非严格按指纹删 | `apps/proxyctl/internal/osproxy/{darwin,windows,linux}.go` | 后续按指纹/序列号精删；现有可接受残余已记 R04 |
| H2 | 仓库既有 `TestModelPriceTable` 失败 | `core/pkg/scheduler/scorer_test.go` | **另开任务**修价格期望，勿混入本 PR |
| H3 | 本机/Team 真人双机未勾选 | `自测记录.md` | 合并前人工跑一遍 |

## CR 中已修复项（审查期）

| 项 | 处理 |
|----|------|
| `harness-check` 拒绝 `apps/proxyctl` | 更新 `scripts/check-harness-hygiene.sh` 白名单；同步 AGENTS/ARCHITECTURE |

## 产物完整性

- [x] 技术方案已落盘
- [x] 开发风险评估已落盘（Critical=0）
- [x] 任务计划已落盘
- [x] 自测记录已落盘
- [x] CR 报告已落盘（本文件）
- [x] 代码 + 测试已就绪
- [x] 用户指南 `docs/guide/system-proxy-egress.md`

## 与方案一致性（抽样）

| 方案要点 | 实现 |
|----------|------|
| 默认 loopback MITM | `NormalizeSystemProxyConfig` 强制 `127.0.0.1` |
| LAN 需 `allow_lan_clients` + advertise | `ValidateSystemProxyConfig` + Web 二次确认 |
| PAC 不得写死 127.0.0.1（LAN） | `PACProxyHost` + handler/server 测试 |
| `setup/status` | `GET /api/v1/proxy/setup/status` |
| proxyctl `--server` / disable 不关远端 | `engine.enableRemote` / `Disable` |
| 前端双模式 | `SystemProxy.vue` 本机/团队 |

## 结论

- [x] **有条件批准** — 本需求代码可合并到发布分支前，请人工：  
  1）勾选自测记录本机/Team 手工项；  
  2）知悉并另单跟踪 `TestModelPriceTable` 既有失败；  
  3）确认接受 CA 按 CN 卸载的残余风险（R04）。  
- [ ] **需修改** — （无本需求 Critical）  
- [ ] **拒绝**

## 人工复核清单（请回复确认）

1. [ ] 本机 macOS/Windows（或你可用平台）`enable/disable` 往返正常  
2. [ ] Team：LAN 开 → 员工 `--server` → 员工 disable 后服务器仍可用  
3. [ ] 同意在 H2/H3 条件下合并本功能  
