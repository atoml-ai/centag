# centag-proxyctl (npm)

Centag 的**本机 / Team 员工侧**代理辅助工具，npm 分发外壳。底层是 Go 实现的原生二进制（`apps/proxyctl`），本包只负责**安装、更新与调用**，让你能像其他 Agent 一样用 npm 分发。

> 工具本体逻辑见 `apps/proxyctl` 与 `docs/guide/system-proxy-egress.md`。本包不重写 Go 代码。

## 两个包

| 包 | 行为 | 适用 |
|----|------|------|
| `centag-proxyctl` | 安装时按平台从 GitHub Release 下载二进制；首次运行也可惰性下载 | 默认推荐，包小、随 npm 更新 |
| `centag-proxyctl-offline` | 二进制随包发布，安装即离线可用 | 内网 / 无外网环境 |

两个包暴露同名 bin：`centag-proxyctl`。

## 安装

```bash
npm i -g centag-proxyctl          # 或 npx centag-proxyctl <cmd>
```

### 与第三方 Agent 一起发布

在 Agent 的 `package.json` 中加入依赖即可随 `npm i` 自动装好：

```json
{ "dependencies": { "centag-proxyctl": "^0.2.7" } }
```

Agent 启动脚本里直接调用：

```js
import { execFileSync } from 'node:child_process';
execFileSync('centag-proxyctl', ['run', '--', 'opencode'], { stdio: 'inherit' });
```

## 用法

```bash
# 进程代理启动 Agent（推荐）
centag-proxyctl run --server http://<advertise>:20060 -- opencode
centag-proxyctl run -- opencode                      # 本机 Centag

# 只打印 export（可 eval）
centag-proxyctl env --server http://<advertise>:20060

# 系统 PAC + CA（可选）
centag-proxyctl enable [--server URL]
centag-proxyctl disable
centag-proxyctl status
centag-proxyctl doctor [--server URL]
```

## 环境变量

| 变量 | 说明 |
|------|------|
| `CENTAG_PROXYCTL_MIRROR` | 覆盖下载 base URL（默认 `https://github.com/atoml-ai/centag/releases/download/v<version>`），用于私有/CDN 镜像 |
| `CENTAG_PROXYCTL_TOKEN` | 私有镜像的 Bearer token |
| `CENTAG_PROXYCTL_SKIP_DOWNLOAD` | `1` 时跳过下载（仅离线包或已预置二进制时有用） |
| `CENTAG_PROXYCTL_REQUIRE_DOWNLOAD` | `1` 时下载失败则安装报错（默认失败仅警告，运行时再惰性下载） |
| `CENTAG_PROXYCTL_TOKEN`（运行时） | 连接 Team 服务器 setup/status 的 Bearer token |
| `CENTAG_API_BASE` | 默认 API base（缺省 `http://127.0.0.1:20060`） |

## 更新

随 npm：`npm update -g centag-proxyctl`。主包会重新 postinstall 下载新版本二进制；或直接重装以获取新二进制。

## 发布

见 `scripts/publish-proxyctl-npm.sh`（交叉编译 6 平台二进制 + 生成 checksums + 打两个 npm 包 + 可选 GitHub Release）。
