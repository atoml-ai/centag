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
| v0.2.7 | [商业化分层 Open Core](v0.2.7/commercialization-layered/) | `v0.2.7` | 插件态方案；**E0/E1 ✅**（删 dist/team，team 构建进 pro）；E2/E3/E4 待做 |
| v0.2.6 | [普通用户 WebUI 能力矩阵](v0.2.6/普通用户WebUI能力矩阵/)（含 [UI浏览器验收](v0.2.6/普通用户WebUI能力矩阵/UI测试流程.md)） | `feature/v0.2.6` | Step 1–5 ✅；Gate 3 ✅；**Gate 4 待人工确认** |
| v0.2.5 | [本机系统代理出口](v0.2.5/本机系统代理出口/)（本机+远端/Team） | `feature/v0.2.5` | Step 5 CR 有条件批准；Gate 4 待人工确认手工自测 |
| v0.2.4 | [计费功能](v0.2.4/billing/) | `feature/v0.2.4` | Step 1–3 完成；可 `step4-test` / `step5-review` |
| v0.2.3 | [能力槽模型配置](v0.2.3/能力槽模型配置/) | `feature/v0.2.3` | Gate 1 通过；可 step2-plan |
| v0.2.3 | 补充：可选桌面启动器 `apps/launcher`（`build/run <edition> --launcher`） | `feature/v0.2.3` | 见 [apps/launcher/README.md](../../apps/launcher/README.md) |
| v0.2.2 | [钩子增值能力](v0.2.2/钩子增值能力/) | `feature/v0.2.2` | Step 1–5 完成；Gate 4 待确认 |
