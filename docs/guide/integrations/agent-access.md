# Agent 接入能力矩阵

Centag「Agent 配置」页按**能力矩阵**展示接入方式，而不是仅按 CLI / 桌面分组猜测。

## 接入方式

| 方式 | 含义 |
|------|------|
| `write_config` | 一键改写本地配置，使请求走 Centag 代理 |
| `ui_guide` | 客户端不支持有效写配置；向导给出可复制参数（请求地址、随流水线变化的模型 ID 等）供在客户端填写 |
| `wrap_cli` | `centag wrap run -- <companion-cli>`，依赖系统代理（MITM + 出口 Key） |
| `builtin` | 内置 TUI / Web，进程内绑定流水线 |

一张产品卡可同时声明多种方式（例如 CodeBuddy：写配置 + wrap 配套 CLI）。

## 桌面与 wrap

- **不要**用 wrap 启动桌面 `.app` / IDE launcher（如 `trae` 打开 Trae.app）。
- 若产品另有配套 CLI（如 `codebuddy`），wrap 只启动该 CLI，并在卡片上展示安装提示。
- 纯桌面走 UI 参数向导（如 TRAE、WorkBuddy）：选择流水线后复制请求地址与模型 ID，在客户端「设置 → 模型」填写。
  - **默认请求地址均为 `…/v1`**（不带 `/chat/completions`）；个别客户端若也接受完整路径，向导会额外提示。
  - **TRAE**：关闭「完整 URL」。
  - 独立开源 Trae Agent（`trae-cli`）与 IDE 不是同一产品，首期未挂到同一卡。

## 双形态产品

同时提供桌面与 CLI、且配置可写时，策略与终端 Agent 相同：**写配置 + wrap CLI**；桌面侧重启/选用模型作为补充说明。

## 验证标记

`verified_write` / `verified_ui` / `verified_wrap` 分别表示维护者是否亲自验证过该路径。未验证 ≠ 不可用。
