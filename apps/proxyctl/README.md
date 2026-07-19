# centag-proxyctl

本机 / Team 员工侧工具：写入系统 PAC、安装 CA，将 Agent 流量导向 Centag（不改 Agent 配置）。

独立 `go.mod`，**不**加入根 `go.work`，不依赖 Centag core。

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

员工机或单独拷贝源码时，直接用模块内命令（与 `scripts/build-proxyctl.sh` 等价）：

```bash
cd apps/proxyctl
GOWORK=off go build -o centag-proxyctl .
```

## 用法

### 经 start.sh（开发机）

```bash
./start.sh run proxyctl enable                         # 本机 Centag
./start.sh run proxyctl enable --server http://host:20060   # Team 远端
./start.sh run proxyctl doctor [--server URL]
./start.sh run proxyctl disable                        # 恢复快照；远端模式不关服务器 MITM
./start.sh run proxyctl status
```

### 真源 CLI（分发给员工的二进制）

```bash
./centag-proxyctl enable                         # 本机 Centag
./centag-proxyctl enable --server http://host:20060   # Team 远端
./centag-proxyctl doctor [--server URL]
./centag-proxyctl disable                        # 恢复快照；远端模式不关服务器 MITM
./centag-proxyctl status
```

`./start.sh run proxyctl …` 与直接执行 `centag-proxyctl …` 参数一致。

可选环境变量：`CENTAG_API_BASE`、`CENTAG_PROXYCTL_TOKEN`。

文档：`docs/guide/system-proxy-egress.md`。
