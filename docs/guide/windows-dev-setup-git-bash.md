# Windows 开发环境搭建（Git Bash）

本文档介绍如何在 Windows 的 Git Bash 环境下搭建 Centag 开发环境。

## 前置条件

- Git for Windows (包含 Git Bash)
- Go 1.21+
- Node.js 20.19+ 或 22.12+
- npm（随 Node.js 安装）

## 安装 make

`start.sh` 在构建后端时会调用 `make build`，因此需要在 Git Bash 中安装 GNU Make。

> **注意**：Git Bash 的 `/etc/profile` 会重置 PATH，`.bashrc` / `.bash_profile` 中追加的路径可能不生效。推荐直接复制到 Git Bash 内置的 `/usr/bin/` 目录。

### 推荐方法：复制到 /usr/bin

1. 下载 make 二进制文件：
```bash
curl -L -o "$TEMP/make.exe" "https://www.equation.com/ftpdir/make/x86_64/make.exe"
```

2. 复制到 Git Bash 的 `/usr/bin/`（始终在 PATH 中）：
```bash
cp "$TEMP/make.exe" /usr/bin/make.exe
```

3. 重启 Git Bash 终端，验证：
```bash
make --version
# GNU Make 4.4.1
```

### 备选方法：Chocolatey（需要管理员权限）

以**管理员身份**打开 PowerShell：
```powershell
choco install make -y
```
重启 Git Bash 验证。

## 配置 Go 临时目录（GOTMPDIR）

Go 在 Windows 上默认使用 `C:\Windows` 作为临时编译目录，普通用户无写入权限，编译时会报错：

```
go: creating work dir: mkdir C:\Windows\go-build...: Access is denied.
```

### 解决方法

1. 创建临时目录：
```bash
mkdir -p "$HOME/tmp"
```

2. 写入 Go 环境配置文件（持久生效）：
```bash
mkdir -p ~/go
echo 'GOTMPDIR=C:/Users/你的用户名/tmp' > ~/go/env
```

3. 重启 Git Bash，验证：
```bash
go env GOTMPDIR
# 应输出: C:/Users/<用户名>/tmp
```

## 构建和运行

```bash
cd /d/atomlai/centag
./start.sh run personal
```

或使用 Windows 原生脚本（无需 make）：
```cmd
start.bat run personal
```

## 常见问题

### Q: `make: command not found`

直接复制到 `/usr/bin/`（见上方推荐方法）。Git Bash 的 `.bashrc` / `.bash_profile` 中追加的路径**可能不生效**，因为 `/etc/profile` 会重置 PATH。

### Q: `go: creating work dir: mkdir C:\Windows\go-build...: Access is denied`

设置 `GOTMPDIR`（见上方"配置 Go 临时目录"）。

### Q: start.sh vs start.bat

| 脚本 | 环境 | 说明 |
|------|------|------|
| `start.sh` | Git Bash | Linux/macOS 风格，需要 make + GOTMPDIR |
| `start.bat` | CMD/PowerShell | Windows 原生，无需 make |

**推荐**：Windows 用户优先使用 `start.bat`，避免 Unix 工具链依赖问题。

## 路径说明

Git Bash 中的路径映射：
- `C:\Users\<用户名>` → `/c/Users/<用户名>` 或 `~`
- `D:\atomlai\centag` → `/d/atomlai/centag`

## 相关文档

- [主 README](../../README.md)
- [中文 README](../../README.zh-CN.md)
- [deploy/stack 快速开始](../../deploy/stack/docs/getting-started.md)
