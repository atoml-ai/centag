# 本机 / 团队代理出口（进程代理 + PAC + MITM）

> 版本：v0.2.5  
> 目标：第三方 Agent **尽量不改自身配置**，把 **大模型 API 流量** 导入 Centag；**不**为某个 Agent 做专用适配。

## 推荐接入（先看这里）

| 优先级 | 方式 | 适用 |
|--------|------|------|
| **首选** | `centag-proxyctl run -- …` | OpenCode 等多数 CLI（不读系统 PAC） |
| 可选 | `centag-proxyctl enable` + 系统 PAC | 认「自动代理」的桌面客户端 |
| 后续 | Clash TUN 等 | 两者都不认的硬编码客户端 |

员工侧 **一条命令**（自动下 CA、设 `HTTPS_PROXY` + `NODE_EXTRA_CA_CERTS`、启动 Agent）：

```bash
# 开发机
./start.sh build proxyctl   # 首次
./start.sh run proxyctl run --server http://<advertise_host>:20060 -- opencode

# 或真源二进制
./bin/proxyctl/centag-proxyctl run --server http://<advertise_host>:20060 -- opencode
```

本机 Centag（无 `--server`）：

```bash
./start.sh run proxyctl run -- opencode
```

调试只看环境变量：

```bash
./start.sh run proxyctl env --server http://<advertise_host>:20060
# 或: eval "$(centag-proxyctl env --server …)"
```

**不要**把 `HTTPS_PROXY` 写进 `~/.zshrc`。Agent **不需要**知道 Centag API Key（由服务端 MITM 注入）。

---

## 管理员（一次配置，无需为 Key 停服）

1. Web →「本机代理出口」→ **团队服务器**  
2. 开启 MITM；开启「允许局域网客户端」；填写 `advertise_host`；`listen_addr` 建议 `0.0.0.0`  
3. 点击 **「一键绑定/创建出口 Key」**（或从已有 Key 下拉绑定）— **热生效，无需重启**  
4. 确认 PAC 正文为 `PROXY <advertise>:8081`（不是 `127.0.0.1`）  
5. 防火墙放行可信网段的 `20060` 与 `8081`

开启 MITM / 保存系统代理配置时，后端也会自动尝试创建/绑定名为 `system-proxy-egress` 的 Key。

出口 Key 解析顺序（服务端）：

1. `system_proxy.egress_api_key`（Web 绑定，推荐）  
2. `LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY`（可选 bootstrap）  
3. `LLM_PROXY_DEFAULT_ADMIN_API_KEY` / `LLM_PROXY_ADMIN_API_KEY`（可选 bootstrap）

日常运维 **不必**改环境变量重启服务。

---

## 两种部署模式

| 模式 | MITM 监听 | 进程代理指向 |
|------|-----------|--------------|
| 本机 | `127.0.0.1:8081` | `http://127.0.0.1:8081` |
| 团队局域网 | `0.0.0.0:8081` | `http://<advertise_host>:8081` |

## 构建

| 方式 | 命令 |
|------|------|
| 仓库 | `./start.sh build proxyctl` |
| 真源 | `cd apps/proxyctl && GOWORK=off go build -o centag-proxyctl .` |

## 系统 PAC（可选）

```bash
centag-proxyctl enable [--server http://<advertise>:20060]
centag-proxyctl doctor [--server …]
centag-proxyctl disable   # 远端模式不关服务器 MITM
```

若 `setup/status` 需登录：`CENTAG_PROXYCTL_TOKEN=<Bearer>`。

## 手写环境变量（等价于 run，一般不必）

```bash
curl -fsSL -o ~/.centag/proxyctl/ca.crt http://<advertise>:20060/api/v1/proxy/ca.crt
HTTPS_PROXY=http://<advertise>:8081 \
HTTP_PROXY=http://<advertise>:8081 \
NO_PROXY=localhost,127.0.0.1,::1 \
NODE_EXTRA_CA_CERTS=$HOME/.centag/proxyctl/ca.crt \
opencode
```

## 为何不影响其它上网

1. 代理变量只包住 Agent 进程。  
2. 非白名单域名：MITM 只做 CONNECT 隧道，不解密。  
3. 白名单 LLM API 才进 Centag；出口 Key 仅在服务端注入。

## Agent 适用矩阵

| 类型 | 例子 | 推荐 |
|------|------|------|
| 忽略 PAC | OpenCode 等 | `proxyctl run` |
| 认系统 PAC | 部分桌面客户端 | `proxyctl enable` |
| 都不认 | 部分 Electron | Clash TUN（后续） |

## 鉴权与模型

- Agent 可填任意上游 Token；MITM 换成 Centag 出口 Key。  
- 上游真实密钥配在 Centag **后端 Provider**。  
- 透明模式改写为系统默认模型；改 Web「默认模型 / 默认后端」即可。  
- SSE 原样透传。

## 安全要点

- 默认 MITM 仅 loopback；Team 须显式开 LAN。  
- 勿开全局系统代理；勿污染登录 shell 环境。  
- 信任 CA 后白名单域名可被解密——仅可信内网。  

## 相关

- 工具：`apps/proxyctl`  
- Web：配置页 →「本机代理出口」  
- 技术方案：`docs/versions/v0.2.5/本机系统代理出口/技术方案.md`
