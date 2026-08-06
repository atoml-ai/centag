# Centag desktop shell（`apps/launcher`）

桌面形态外壳：拉起 sidecar + 菜单栏/托盘。产品形态只有两种：

| 形态 | 入口 | 说明 |
|------|------|------|
| **cli** | `./start.sh run personal` | daemon 监督 sidecar（非 debug）；无托盘 |
| **desktop** | `./start.sh run personal --desktop` | 菜单栏/托盘 + 打开 UI + 退出停 sidecar |

内部 Go build tag `tray` 仍用于启用 systray（实现细节，不是产品名）。

菜单图标资源：`apps/launcher/assets/` 下的 `trayicon.png` / `trayicon.ico`。

## 用法

```bash
# 构建 CLI sidecar + desktop 外壳
./start.sh build personal --desktop

# 以 desktop 启动（生产日志）
./start.sh run personal --desktop

# 开发：先构建再以 debug 启动 desktop（前端 vite watch + debug 日志）
./start.sh debug personal --desktop

# 仅 CLI
./start.sh run personal
./start.sh debug personal
```

也可直接调用构建脚本：

```bash
./scripts/build-launcher.sh --desktop   # → centag-desktop
```

## 产物路径

```
~/.centag/var/cross/launcher/<goos>-<goarch>/centag-desktop[.exe]
~/.centag/bin/centag-desktop[.exe]
```

发行包命名（GitHub）：`centag-desktop-<edition>-macos-<arch>.{dmg,zip}` / `…-windows-<arch>.zip`（见 `scripts/release/package-desktop.sh`）。

## 平台

| 系统 | desktop | 说明 |
|------|---------|------|
| macOS (darwin) | ✅ 本机 CGO | 菜单栏图标 |
| Windows | ✅ 本机 CGO | 系统托盘 |
| Linux | ✅ 本机 CGO + GTK | 需桌面环境 |

desktop 因 systray 依赖 **CGO**，请在目标系统上本地构建；不保证交叉编译。

## 行为摘要

1. 解析 sidecar（`centag-personal` 等）与用户数据目录。
2. 启动 sidecar，健康检查通过后显示托盘。
3. **非 debug**：托盘监督 sidecar，异常退出后自动拉起（退避重试）。可用 `CENTAG_LAUNCHER_SUPERVISE=0` 关闭；`LLM_PROXY_SERVER_MODE=debug`（`./start.sh debug … --desktop`）默认不自动拉起。
4. 菜单：**打开管理界面**、**运行**（`/agent-run`）、**退出**（停止监督并结束）。
5. `./start.sh run … --desktop` 会先 `load_env`，把 `config/secrets/.env` 中的管理员口令传给 sidecar。
6. **本迭代不做桌面应用内 OTA**（升级请重装 dmg/zip）。

CI / 无桌面环境可用 `-headless`（跳过 systray 循环）。
