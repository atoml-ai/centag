# 部署包统一入口

三维概念：**形态 × 系统 × 架构**。

| 维度 | 取值 | 说明 |
|------|------|------|
| **form（形态）** | `cli` / `desktop` | 同一维度：命令行包 vs 桌面包（桌面含托盘壳，不再单独叫 tray） |
| **os（系统）** | `macos` / `linux` / `windows` / `fnos` / `docker` | 目标运行环境；**fnos / docker 也是系统** |
| **arch（架构）** | `amd64` / `arm64` / `host` / `all` | 可省略（有默认） |

与开发用 `make build`、热更新 `./start.sh pack` 分开。

## 入口

```bash
./start.sh package <form> <os> [arch] [选项...]
./start.sh package list
```

## 矩阵

|  | macos | linux | windows | fnos | docker |
|--|:-----:|:-----:|:-------:|:----:|:------:|
| **cli** | ✓ | ✓ | ✓ | ✓ (.fpk) | ✓ (离线包) |
| **desktop** | ✓ (dmg+zip) | · | ✓ (zip) | · | · |

## 示例

```bash
# 桌面（本机 macOS → dmg）
./start.sh package desktop macos
./start.sh package desktop macos --skip-frontend

# 桌面 Windows（须在 Windows 本机）
./start.sh package desktop windows

# Linux CLI
./start.sh package cli linux
./start.sh package cli linux amd64

# 飞牛（系统 = fnos）
./start.sh package cli fnos amd64 --edition personal

# Docker 离线
./start.sh package cli docker
```

## 产物命名（互不覆盖）

同版本目录 `~/.centag/var/release/<version>/` 可并存：

| 形态 | 文件名 |
|------|--------|
| cli | `centag-cli-<edition>-<goos>-<goarch>.tar.gz` |
| desktop | `centag-desktop-<edition>-macos-<arch>.{dmg,zip}` / `…-windows-<arch>.zip` |

构建时只替换**本形态本平台**文件，不会清空整个版本目录。

## install.sh / GitHub 发布集

不是单独「github 形态」，而是组合：

- `desktop` + `macos`
- `desktop` + `windows`
- `cli` + `linux`

由 CI 分 runner 构建后汇总；本机可只打当前系统的 desktop + 交叉 `cli linux`。

## 默认参数（fnos）

根目录 [`packaging.env`](../../packaging.env)：`PACKAGE_ARCH` / `PACKAGE_MODE` / `PACKAGE_EDITION` / `PACKAGE_OUTPUT` 等。

## 新增目标

1. 实现构建脚本。
2. 在 `package.sh` 的 `run_cli` / `run_desktop` 中按 os 分支注册。
3. 更新本 README 矩阵表。
