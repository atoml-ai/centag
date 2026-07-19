# 本机 / 团队代理出口（进程代理 + PAC + MITM）

> 版本：v0.2.5  
> 目标：第三方 Agent **尽量不改自身配置**，把 **大模型 API 流量** 导入 Centag，享受流水线与可定制增值；**不**为某个 Agent 做专用适配。

## 推荐接入方式（先看这里）

实践中，**大多数 CLI / Agent 并不走系统 PAC**（例如 OpenCode），因此：

| 优先级 | 方式 | 适用 | 对上网的影响 |
|--------|------|------|----------------|
| **首选** | **仅给 Agent 进程设置** `HTTPS_PROXY` / `HTTP_PROXY` | 多数 CLI、SDK、忽略 PAC 的工具 | **不影响**浏览器与其它 App；见下文约束 |
| 可选 | `centag-proxyctl` + 系统 PAC | 认系统「自动代理」的桌面客户端 | 按 PAC 白名单分流；勿开「全局代理」 |
| 后续 | Clash TUN 等 | 两者都不认的硬编码客户端 | 另波次 |

### 进程级代理（推荐模板）

只包住 **启动 Agent 的那一行**，不要写进全局 `~/.zshrc` / 系统环境，以免影响日常上网：

```bash
# 本机 MITM（默认 8081）。NO_PROXY 避免本机环回走代理。
export NO_PROXY=localhost,127.0.0.1,::1
export no_proxy="$NO_PROXY"
export HTTPS_PROXY=http://127.0.0.1:8081
export HTTP_PROXY=http://127.0.0.1:8081
export https_proxy="$HTTPS_PROXY"
export http_proxy="$HTTP_PROXY"

# 示例：仅本次进程生效
HTTPS_PROXY=http://127.0.0.1:8081 \
HTTP_PROXY=http://127.0.0.1:8081 \
NO_PROXY=localhost,127.0.0.1 \
opencode
```

Team 员工指向服务器 MITM 时，把 `127.0.0.1` 换成 `advertise_host`（须管理员已开局域网出口）。

### 为何「设了代理」却不该拖垮其它上网

1. **用户日常上网**：代理变量只加在 Agent 启动命令 / 专用 launcher 上，**不要**开系统全局代理，也**不要**把 `HTTPS_PROXY` 写进登录 shell 的全局环境。  
2. **Agent 的非大模型访问**（WebFetch、下代码、访问 npm/GitHub 等）：MITM 对 **未在域名白名单中的主机** 只做 **CONNECT TCP 隧道（不解密）**，流量直达目标，不进 Centag。  
3. **只有白名单 LLM 域名** 才会 MITM 解密；其中像 LLM API 的路径再转发到 Centag `:20060`，其余路径仍回原站。

因此：进程级 `HTTPS_PROXY` 指向 Centag MITM = 「该进程的出口先到 MITM」，**不是**「整台电脑翻墙」，也**不是**「Agent 所有 HTTPS 都被改写」。

> 白名单域名上的 **非 API 页面**（如厂商营销页）仍会经 MITM 解密后回原站，需信任 Centag CA。真正无关的主机（GitHub 等）走隧道，**不要求**对该主机信任 Centag CA。

---

## 两种部署模式（服务端）

| 模式 | 适用 | MITM 监听 | 进程代理应指向 |
|------|------|-----------|----------------|
| **本机** | personal / 单机 | `127.0.0.1:8081` | `http://127.0.0.1:8081` |
| **团队局域网** | team | `0.0.0.0:8081`（显式开启） | `http://<advertise_host>:8081` |

PAC 正文中的 `PROXY` 与上表一致（认 PAC 的客户端用）；**多数 Agent 请直接用上表「进程代理」列**。

## 构建 centag-proxyctl

| 方式 | 命令 |
|------|------|
| 仓库入口 | `./start.sh build proxyctl`（或 `./start.sh build personal --proxyctl`） |
| 运行（开发机） | `./start.sh run proxyctl enable` |
| **真源**（客户端直接用） | `cd apps/proxyctl && GOWORK=off go build -o centag-proxyctl .` |

`proxyctl enable` 配置的是 **系统 PAC + CA**，适合认系统代理的 App。  
若你只用 CLI + `HTTPS_PROXY`，仍需：**启动 Centag、开启 MITM、信任 CA（解密白名单 LLM 域名时）**；不一定要 `proxyctl enable`。

## 本机一键（系统 PAC，可选）

1. 启动 Centag，在 Web「本机代理出口」开启 MITM。  
2. 需要系统级 PAC 时再执行：

```bash
centag-proxyctl enable
centag-proxyctl doctor
centag-proxyctl disable
```

`disable` 恢复启用前的系统代理，并按指纹尝试卸载 Centag CA。

## Team：管理员 + 员工

### 管理员（服务器）

1. Web →「团队服务器」→ 开启「允许局域网客户端」（二次确认）。  
2. 填写 `advertise_host`，`listen_addr` 建议 `0.0.0.0`。  
3. 复制员工可用的 PAC/CA，以及 **进程代理** 示例：  
   `HTTPS_PROXY=http://<advertise>:8081`  
4. 防火墙仅对可信网段开放 `20060` 与 `8081`。

### 员工（客户端）

**多数情况（CLI Agent）**：信任团队 CA 后，仅在启动 Agent 时设置 `HTTPS_PROXY` / `HTTP_PROXY` / `NO_PROXY`（见上文模板）。

**认系统 PAC 时**：

```bash
centag-proxyctl enable --server http://<advertise_host>:20060
centag-proxyctl doctor --server http://<advertise_host>:20060
centag-proxyctl disable   # 只恢复自己电脑，不关服务器
```

若 `setup/status` 需登录，可设 `CENTAG_PROXYCTL_TOKEN`。

## Agent 适用矩阵

| 类型 | 例子 | 推荐做法 |
|------|------|----------|
| 只认环境变量 / 忽略 PAC | OpenCode 等多数 CLI | **进程级** `HTTPS_PROXY`→MITM（首选） |
| 认系统自动代理 | 部分桌面客户端 | `proxyctl enable` + PAC 白名单 |
| 两者都忽略 | 部分硬编码 Electron | Clash TUN（后续）或改 `base_url` |

## 鉴权（Agent 不用改 Key）

第三方 Agent 里仍填**上游厂商 Token**。  
MITM 转发到 Centag `:20060` 时会注入出口 Key；Agent 无需填写 Centag Key。

出口 Key 解析顺序：

1. `system_proxy.egress_api_key`  
2. `LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY`  
3. `LLM_PROXY_DEFAULT_ADMIN_API_KEY` / `LLM_PROXY_ADMIN_API_KEY`

请在 Web「API Keys」创建 `llmproxy_*`，写入上述之一后重启或保存配置。  
上游真实密钥配置在 Centag **后端**，由流水线使用。

### 模型名

透明模式会把请求体里的厂商模型名改写为 Centag **系统默认模型**（`proxy.default_model`），并保留 messages/tools。  
要用 mimo 等，请改 Web「默认模型 / 默认后端」，不要改 Agent。  
流式响应按上游 SSE **原样透传**（避免客户端把 `data:` 当正文显示）。

## 安全要点

- 默认 MITM 仅 loopback；Team 须显式 `allow_lan_clients`。  
- PAC / 域名表保持白名单；勿开全局系统代理。  
- 进程级 `HTTPS_PROXY` 只给 Agent；勿污染用户全局环境。  
- 信任 CA 后，**白名单域名**上的 HTTPS 可被解密——仅在可信环境使用。  
- 不对公网自动暴露 MITM。

## 相关

- 技术方案：`docs/versions/v0.2.5/本机系统代理出口/技术方案.md`  
- 工具：`apps/proxyctl`  
- Web：配置页 →「本机代理出口」
