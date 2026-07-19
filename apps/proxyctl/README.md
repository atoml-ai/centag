# centag-proxyctl

本机 / Team 员工侧工具：

1. **进程代理包装（推荐）**：下载 CA、设置 `HTTPS_PROXY` / `NODE_EXTRA_CA_CERTS`，再启动第三方 Agent（OpenCode 等多数 CLI 不读系统 PAC）。
2. **系统 PAC + CA**：写入系统自动代理（仅认 PAC 的桌面客户端需要）。

独立 `go.mod`，**不**加入根 `go.work`，不依赖 Centag core。

Centag API Key **不会**注入到 Agent 环境；由服务端 MITM 注入出口 Key。

## 构建

### 仓库入口（推荐开发机）

```bash
./start.sh build proxyctl
# 或与发行版一并构建
./start.sh build personal --proxyctl
./start.sh build team --proxyctl
```

产物：

```
bin/proxyctl/<goos>-<goarch>/centag-proxyctl[.exe]
bin/proxyctl/centag-proxyctl[.exe]   # 当前主机便捷副本
```

### 真源命令（客户端 / CI / 不经 start.sh）

```bash
cd apps/proxyctl
GOWORK=off go build -o centag-proxyctl .
```

## 用法

### 进程代理启动 Agent（推荐）

```bash
# 开发机
./start.sh run proxyctl run --server http://192.168.1.4:20060 -- opencode

# 真源二进制
./bin/proxyctl/centag-proxyctl run --server http://192.168.1.4:20060 -- opencode

# 只打印 export（可 eval）
./bin/proxyctl/centag-proxyctl env --server http://192.168.1.4:20060
```

本机 Centag（无 `--server`）默认指向 `http://127.0.0.1:20060`，MITM `http://127.0.0.1:8081`。

### 系统 PAC（可选）

```bash
./start.sh run proxyctl enable [--server URL]
./start.sh run proxyctl doctor [--server URL]
./start.sh run proxyctl disable
./start.sh run proxyctl status
```

可选环境变量：`CENTAG_API_BASE`、`CENTAG_PROXYCTL_TOKEN`。

文档：`docs/guide/system-proxy-egress.md`。
