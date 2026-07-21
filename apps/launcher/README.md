# Centag Launcher（L1 桌面补充）

轻量桌面启动器：拉起 sidecar + 系统浏览器。默认 **lite**（无 CGO，可交叉编译）；可选 **tray**（菜单栏/托盘，需 CGO）。

不依赖 Wails / WebView，**不 import Centag core**——与 `minimal` / `personal` 发行版解耦，仅作补充入口。

菜单图标资源使用 `apps/launcher/assets/` 下的 `trayicon.png` / `trayicon.ico`（仅 tray 构建嵌入）。

## 两种形态

| 形态 | 二进制 | CGO | 能力 |
|------|--------|-----|------|
| **lite（默认）** | `centag-launcher` | 否 | 启 sidecar + 打开浏览器，Ctrl+C 退出 |
| **tray** | `centag-launcher-tray` | 是 | 菜单栏/托盘 + 打开 UI + 退出停 sidecar |

## 用法（`--launcher` / `--launcher-tray`）

```bash
# lite（默认，无 CGO）
./start.sh build personal --launcher
./start.sh run personal --launcher

# tray（CGO / systray）
./start.sh build personal --launcher-tray
./start.sh run personal --launcher-tray

./start.sh build minimal --launcher
```

| 命令 | 含义 |
|------|------|
| `build personal --launcher` | 发行包 + 前端 + **lite** 启动器 |
| `build personal --launcher-tray` | 发行包 + 前端 + **tray** 启动器 |
| `build team --launcher` | **不支持** |

真源构建：

```bash
./scripts/build-launcher.sh          # lite → ~/.centag/var/cross/launcher/... + bin/
./scripts/build-launcher.sh --tray   # tray → ~/.centag/var/cross/launcher/... + bin/
```

## 平台支持

| 系统 | lite | tray | 说明 |
|------|------|------|------|
| macOS (darwin) | ✅ 可交叉编译 | ✅ 本机 CGO | tray：菜单栏图标 |
| Windows | ✅ 可交叉编译 | ✅ 本机 CGO | tray：系统托盘 |
| Linux | ✅ 可交叉编译 | ✅ 本机 CGO + GTK | tray：需桌面环境 |

tray 因 `energye/systray` 依赖 **CGO**，请在目标系统上本地构建；不保证交叉编译。  
可用 `CENTAG_LAUNCHER_GOOS` / `CENTAG_LAUNCHER_GOARCH` 覆盖（lite 可交叉；tray 跨平台常失败）。

产物：

```
~/.centag/var/cross/launcher/<goos>-<goarch>/centag-launcher[.exe]
~/.centag/var/cross/launcher/<goos>-<goarch>/centag-launcher-tray[.exe]
~/.centag/bin/centag-launcher[.exe]        # 当前主机 lite 便捷副本
~/.centag/bin/centag-launcher-tray[.exe]   # 当前主机 tray 便捷副本
```

## 能力

| 项 | lite | tray |
|----|------|------|
| 启动 sidecar | ✅ | ✅ |
| 打开 UI（系统浏览器） | ✅ | ✅ |
| 菜单/托盘 | ❌ | ✅ |
| 数据目录 | 同左 | macOS: `~/Library/Application Support/Centag[Minimal]` |

`./start.sh run … --launcher` 会先 `load_env`，把 `config/secrets/.env` 中的 `LLM_PROXY_ADMIN_PASSWORD` 传给 sidecar。

CI / 无桌面环境可用 `-headless`（tray 构建内跳过 systray；lite 本身即无托盘）。

**非目标**：原生 App 窗口、首次向导、系统代理注入、AI 助手、team 桌面版。

## 解耦约定

1. 独立 `go.mod`，**不加入** 根 `go.work`。
2. 不修改 `core/`、`dist/` 业务逻辑。
3. 入口通过 `build|run <edition> --launcher|--launcher-tray`，不占用独立顶层命令。
4. Build tag：`tray` 启用 systray；默认无 tag = lite。
