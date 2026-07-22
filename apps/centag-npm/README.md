# centag (npm)

Centag LLM 统一网关的 npm 分发包。底层是 Go 实现的原生二进制，本包负责**安装、更新与启动**。

## 两个包

| 包 | 行为 | 适用 |
|----|------|------|
| `centag` | 安装时按平台从 GitHub Release 下载二进制；首次运行也可惰性下载 | 默认推荐，包小、随 npm 更新 |
| `centag-offline` | 二进制随包发布，安装即离线可用 | 内网 / 无外网环境 |

两个包暴露同名 bin：`centag`。

## 安装

```bash
npm install -g centag
```

### 离线版（内网环境）

```bash
npm install -g centag-offline
```

## 使用

```bash
# 启动网关 (http://127.0.0.1:20060)
centag

# 指定端口
centag --port 8080

# 查看版本
centag version

# 进程代理（不启动网关）
centag wrap doctor
centag wrap run -- opencode

# 帮助
centag --help
```

## 默认登录

- 用户名: `admin`
- 密码: `centag123`
  (首次启动前设置 `LLM_PROXY_ADMIN_PASSWORD` 可覆盖)

## 环境变量

| 变量 | 说明 |
|------|------|
| `CENTAG_MIRROR` | 覆盖下载 base URL（默认 GitHub Releases），用于私有/CDN 镜像 |
| `CENTAG_TOKEN` | 私有镜像的 Bearer token |
| `CENTAG_SKIP_DOWNLOAD` | `1` 时跳过 postinstall 下载 |
| `CENTAG_REQUIRE_DOWNLOAD` | `1` 时下载失败则安装报错 |

## 更新

```bash
npm update -g centag
```

## 卸载

```bash
npm uninstall -g centag
```

## 与 install.sh 的区别

| | npm | install.sh |
|---|-----|-----------|
| 安装位置 | npm 全局目录 | `~/.centag/` |
| 更新方式 | `npm update -g` | 重新运行 install.sh |
| PATH | npm 自动管理 | 需手动 source env |
| 适用场景 | Node.js 开发者 | 通用 |

两种安装方式互不冲突，可同时使用。

## 发布

```bash
# 构建 + 发布
CENTAG_NPM_TOKEN=xxx ./scripts/publish-centag-npm.sh

# 仅构建打包（不发布）
DRY_RUN=1 ./scripts/publish-centag-npm.sh
```

## 相关

- 项目: https://github.com/atoml-ai/centag
- 问题: https://github.com/atoml-ai/centag/issues
