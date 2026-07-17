# Centag Launcher（L1 桌面补充）

轻量桌面启动器：**拉起 sidecar + 系统菜单/托盘 + 系统浏览器**。  
不依赖 Wails / WebView，**不 import Centag core**——与 `minimal` / `personal` 发行版解耦，仅作补充入口。

菜单图标资源来自 ProxyClaw `apps/desktop` 的 `trayicon.png` / `trayicon.ico`。

## 用法（`--launcher` 辅助开关）

```bash
# 普通服务
./start.sh build personal
./start.sh run personal

# 启动器方式（同 edition + 当前系统 launcher）
./start.sh build personal --launcher
./start.sh run personal --launcher

./start.sh build minimal --launcher
./start.sh run minimal --launcher
```

| 命令 | 含义 |
|------|------|
| `build personal` | 仅 gateway 发行包（个人版服务） |
| `build personal --launcher` | 发行包 + 前端 + **当前系统**启动器 |
| `build minimal --launcher` | minimal 发行包 + 前端 + 启动器 |
| `build team --launcher` | **不支持** |

`gateway` 是 `personal` 的兼容别名。

## 平台支持

| 系统 | 启动器 | 说明 |
|------|--------|------|
| macOS (darwin) | ✅ | 菜单栏图标（systray + template PNG） |
| Windows | ✅ | 系统托盘（systray + ICO） |
| Linux | ✅ | 状态栏/托盘（需桌面环境；通常要 CGO 工具链） |

构建时**自动识别当前主机**的 `GOOS`/`GOARCH`（`go env`），产物：

```
bin/launcher/<goos>-<goarch>/centag-launcher[.exe]
bin/launcher/centag-launcher[.exe]   # 当前主机便捷副本
```

因 `energye/systray` 依赖 **CGO**，请在目标系统上本地构建；不保证交叉编译。  
可用 `CENTAG_LAUNCHER_GOOS` / `CENTAG_LAUNCHER_GOARCH` 覆盖，但跨平台 CGO 往往会失败。

## 能力

| 项 | 说明 |
|----|------|
| 启动 | exec `centag-minimal` 或 `centag-gateway` |
| 打开 UI | 系统默认浏览器 |
| 菜单 | 打开管理界面 / 退出（停止 sidecar） |
| 数据目录 | macOS: `~/Library/Application Support/Centag[Minimal]`（与 `bin/server` 开发库分离） |

`./start.sh run … --launcher` 会先 `load_env`，把 `config/secrets/.env` 中的 `LLM_PROXY_ADMIN_PASSWORD` 传给 sidecar，首轮 seed 才会用该口令。若首次未加载 `.env` 已写入默认口令，需清空用户数据目录后重启再 seed。

**非目标**：原生 App 窗口、首次向导、系统代理注入、AI 助手、team 桌面版。

## 解耦约定

1. 独立 `go.mod`，**不加入** 根 `go.work`。
2. 不修改 `core/`、`dist/` 业务逻辑。
3. 入口只通过 `build|run <edition> --launcher`，不占用独立顶层命令。
