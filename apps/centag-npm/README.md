# @atomlai/centag（npm）

Centag LLM 统一网关的 **npm 分发渠道**。底层是 Go 原生二进制；本包负责安装、更新与启动。

> **普通用户请优先用 [install.sh](https://github.com/atoml-ai/centag#安装)**（`curl | bash`，无需 Node.js，装到 `~/.centag/`）。  
> 本文档面向 **已有 Node.js / npm 工具链** 的开发者。

## 两个包

| 包 | 行为 | 适用 |
|----|------|------|
| `@atomlai/centag` | 安装时按平台从 GitHub Release 下载二进制；首次运行也可惰性下载 | 有网环境，包小（~1 MB） |
| `@atomlai/centag-offline` | 二进制随包发布，安装即离线可用 | 内网 / 无法访问 GitHub |

两个包暴露同名 bin：`centag`。

## 安装

### 方式 A：`npx`（无需全局、无需改 npm 配置）

`npx` **不会在系统里永久安装**命令，而是临时下载到缓存、运行一次即结束（所以装完后 `which centag` 可能仍为空，这是正常的）。

```bash
# 跳过确认提示，直接运行
npx --yes @atomlai/centag version
npx --yes @atomlai/centag

# 若出现 Ok to proceed? (y) 须输入 y 回车，否则不会下载
npx @atomlai/centag version
```

适合快速试用，或 macOS 上 `npm -g` 报权限错时。

### 方式 B：全局安装

```bash
npm install -g @atomlai/centag
```

内网 / 离线：

```bash
npm install -g @atomlai/centag-offline
```

### 方式 C：项目本地依赖

```bash
npm install @atomlai/centag
npx centag version
```

## 故障排除

### `EACCES: permission denied, mkdir '/usr/local/lib/node_modules/...'`

npm **不会**像 Homebrew 那样自动询问密码；遇到权限不足时只会报错退出。需要**手动加 `sudo`**，终端会提示输入本机登录密码：

```bash
sudo npm install -g @atomlai/centag
```

离线版同理：

```bash
sudo npm install -g @atomlai/centag-offline
```

> **说明**：用 `sudo` 装的全局包归 root 所有，日后 `npm update -g` 可能仍需 `sudo`。若经常装全局 CLI，建议改 prefix 到用户目录（见下方方式 3）。

**其它处理方式：**

1. **用 `npx`**，不要 `-g`（见上文方式 A，无权限问题）
2. **改用 install.sh**（无需 Node，装到用户目录）  
   ```bash
   curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/v0.2.9/scripts/install.sh | bash
   ```
3. **把 npm 全局 prefix 改到用户目录**（一劳永逸，适合经常用 `npm -g` 的开发者）  
   ```bash
   mkdir -p ~/.npm-global
   npm config set prefix ~/.npm-global
   echo 'export PATH="$HOME/.npm-global/bin:$PATH"' >> ~/.zshrc
   source ~/.zshrc
   npm install -g @atomlai/centag
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
  （首次启动前设置 `LLM_PROXY_ADMIN_PASSWORD` 可覆盖）

## 环境变量

| 变量 | 说明 |
|------|------|
| `CENTAG_MIRROR` | 覆盖下载 base URL（默认 GitHub Releases），用于私有/CDN 镜像 |
| `CENTAG_TOKEN` | 私有镜像的 Bearer token |
| `CENTAG_SKIP_DOWNLOAD` | `1` 时跳过 postinstall 下载 |
| `CENTAG_REQUIRE_DOWNLOAD` | `1` 时下载失败则安装报错 |

## 更新与卸载

```bash
npm update -g @atomlai/centag
npm uninstall -g @atomlai/centag
```

## 与 install.sh 的对比

| | install.sh（推荐） | npm |
|---|-------------------|-----|
| 需要 Node.js | 否 | 是 |
| 安装位置 | `~/.centag/` | npm 全局或项目 `node_modules` |
| 更新 | 重新运行 install.sh | `npm update -g` |
| 权限问题 | 无（用户目录） | macOS 上 `-g` 可能 EACCES |
| 适用 | **所有用户** | Node.js 开发者 |

两种方式互不冲突，可同时使用。

## 发布（维护者）

```bash
source ~/.zshrc   # 若 CENTAG_NPM_TOKEN 写在 profile 里
./scripts/publish-centag-npm.sh

# 仅构建打包（不发布）
DRY_RUN=1 ./scripts/publish-centag-npm.sh
```

## 相关

- 项目: https://github.com/atoml-ai/centag
- npm: https://www.npmjs.com/package/@atomlai/centag
- 问题: https://github.com/atoml-ai/centag/issues
