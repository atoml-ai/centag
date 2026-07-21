# wrap（`centag wrap`）

本机 / Team 员工侧工具，逻辑真源在本目录；**用户入口优先使用主二进制子命令**：

```bash
centag wrap run -- opencode
centag wrap doctor
```

（不起网关；GitHub Release 只发 personal，已内含该子命令。）

能力：

1. **进程代理包装（推荐）**：下载 CA、设置 `HTTPS_PROXY` / `NODE_EXTRA_CA_CERTS`，再启动第三方 Agent（OpenCode 等多数 CLI 不读系统 PAC）。
2. **系统 PAC + CA**：写入系统自动代理（仅认 PAC 的桌面客户端需要）。

独立 `go.mod`，**不**依赖 Centag core。Centag API Key **不会**注入到 Agent 环境；由服务端 MITM 注入出口 Key。

## 构建

### 随 personal 一并构建（推荐）

```bash
./start.sh build personal
# 或
make build
```

之后使用：`centag wrap …`

### 独立二进制（仅本地 / npm，不进默认 GitHub Release）

```bash
./start.sh build wrap
# 或
cd apps/wrap && GOWORK=off go build -o centag-wrap .
```

产物：

```
~/.centag/var/cross/wrap/<goos>-<goarch>/centag-wrap[.exe]
~/.centag/bin/centag-wrap[.exe]
```

## 用法

### 进程代理启动 Agent（推荐）

```bash
centag wrap run --server http://192.168.1.4:20060 -- opencode
centag wrap run -- opencode                      # 本机 Centag
eval "$(centag wrap env --server http://192.168.1.4:20060)"
```

开发机也可：`./start.sh run wrap run -- …`

### 系统 PAC（可选）

```bash
centag wrap enable [--server URL]
centag wrap doctor [--server URL]
centag wrap disable
centag wrap status
```

可选环境变量：`CENTAG_API_BASE`、`CENTAG_WRAP_TOKEN`。

文档：`docs/guide/system-proxy-egress.md`。
