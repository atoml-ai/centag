# 版本归档

按版本存放需求产物（技术方案、任务计划、自测记录、CR 报告）。

```
docs/versions/
├── README.md          # 本文件
└── vX.Y/
    └── <需求名>/
        ├── workflow_state.md
        ├── 技术方案.md
        ├── 任务计划.md
        └── …
```

历史 Centag 版本文档已在 Centag 首提前清理；新需求从本目录重新归档即可。

## 版本索引

| 版本 | 需求 | 分支 | 状态 |
|------|------|------|------|
| v0.3.1 | [Prompt 策略](v0.3.1/prompt-strategy/) | `feature/v0.3.1` | Step 1–5 ✅；Gate 4 ✅；可 `step6-release` |
| v0.3.0 | [国际化与 GitHub 推广](v0.3.0/i18n-github-promo/) | `feature/v0.3.0` | Step 1 ✅；Gate 1 待人工确认 |
| v0.2.8 | [协议对齐与 Models 接口增强](v0.2.8/protocol-alignment/) | `feature/v0.2.8` | Step 1–6 ✅；GitHub `v0.2.8` |
| v0.2.9 | [Provider/Agent 本地接入与账户池](v0.2.9/provider-agent-local/)（主） | `feature/v0.2.9` | Step 1 ✅；Gate 1 待人工确认 |
| v0.2.9 | [npm 打包与安装文档](v0.2.9/npm-packaging/)（搭便车） | `feature/v0.2.9` | npm `@atomlai/centag@0.2.9` 已发 |
| v0.2.7 | [商业化分层 Open Core](v0.2.7/commercialization-layered/) | `feature/v0.2.7` | 插件态方案；**E0/E1 ✅**（删 dist/team，team 构建进 pro）；E2/E3/E4 待做 |
| v0.2.6 | [普通用户 WebUI 能力矩阵](v0.2.6/普通用户WebUI能力矩阵/)（含 [UI浏览器验收](v0.2.6/普通用户WebUI能力矩阵/UI测试流程.md)） | `feature/v0.2.6` | Step 1–5 ✅；Gate 3 ✅；**Gate 4 待人工确认** |
| v0.2.5 | [本机系统代理出口](v0.2.5/本机系统代理出口/)（本机+远端/Team） | `feature/v0.2.5` | Step 5 CR 有条件批准；Gate 4 待人工确认手工自测 |
| v0.2.4 | [计费功能](v0.2.4/billing/) | `feature/v0.2.4` | Step 1–3 完成；可 `step4-test` / `step5-review` |
| v0.2.3 | [能力槽模型配置](v0.2.3/能力槽模型配置/) | `feature/v0.2.3` | Gate 1 通过；可 step2-plan |
| v0.2.3 | 补充：可选桌面形态 `apps/launcher`（现：`build/run <edition> --desktop`） | `feature/v0.2.3` | 见 [apps/launcher/README.md](../../apps/launcher/README.md) |
| v0.2.2 | [钩子增值能力](v0.2.2/钩子增值能力/) | `feature/v0.2.2` | Step 1–5 完成；Gate 4 待确认 |
