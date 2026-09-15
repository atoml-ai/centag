# 本机 / 团队代理出口（进程代理 + PAC + MITM）

> 版本：v0.2.7  
> 目标：第三方 Agent **尽量不改自身配置**，把 **大模型 API 流量** 导入 Centag；**不**为某个 Agent 做专用适配。

## 推荐接入（先看这里）

| 优先级 | 方式 | 适用 |
|--------|------|------|
| **首选** | `centag wrap run -- …` | OpenCode 等多数 CLI（不读系统 PAC）；主二进制子命令，**不起网关** |
| 可选 | `centag wrap enable --system-proxy` | 认「自动代理」的桌面客户端 |
| 后续 | Clash TUN 等 | 都不认的硬编码客户端 |

员工侧 **一条命令**（自动下 CA、设 `HTTPS_PROXY` + `NODE_EXTRA_CA_CERTS`、启动 Agent）：

```bash
# 已安装 personal（GitHub Release / install.sh）；局域网建议带 --token
centag wrap run --server http://<advertise_host>:20060 --token llmproxy_xxxx -- opencode

# 本机 Centag（无 --server；仅本机可不带 token）
centag wrap run -- opencode
```

调试只看环境变量：

```bash
centag wrap env --server http://<advertise_host>:20060
# 或: eval "$(centag wrap env --server …)"
```

### 桌面壳与应用目录（本机已装应用）

桌面壳（macOS/Windows）托盘「代理启动应用」会列出**本机已安装、可经 Centag 代理**的 AI/Agent 应用，点击即代理启动；「代理诊断」检查 CA/MITM/出口 Key 就绪状态。列表真源为 Agent 注册表（`core/internal/agent`），本机安装检测在客户端完成。

```bash
centag wrap apps             # 列出可代理应用 + 本机是否已安装
centag wrap apps --installed # 仅已安装
centag wrap apps --json      # 机器可读（含 installed/path）
```

- 模型名默认由**透明模式兜底**（未命中模型回落系统默认）；也可在桌面壳勾选「启动前写入模型配置」或 Web「本机代理启动」页写入 `centag/<pipeline>`。
- 本地控制接口（回环免鉴权，非回环需鉴权）：`GET /api/v1/wrap/apps`、`POST /api/v1/wrap/apps/:id/prepare`、`GET /api/v1/wrap/doctor`。

### Wrap Doctor 端点

`GET /api/v1/wrap/doctor` 可选查询参数：

| 参数 | 说明 |
|------|------|
| `app_id` | 检查指定应用是否存在 |
| `domain` | 检查域名是否有证书固定嫌疑 |

示例响应：
```json
{
  "ok": true,
  "checks": [
    {"id": "sidecar", "ok": true, "message": "sidecar 运行中"},
    {"id": "ca", "ok": true, "message": "CA 证书存在: /path/to/ca.crt"},
    {"id": "mitm", "ok": true, "message": "MITM 代理已启用"},
    {"id": "egress_key", "ok": true, "message": "出口 API Key 已配置"},
    {"id": "cert_pinning", "ok": false, "message": "疑似证书固定: api.openai.com", "action": "该应用可能使用了证书固定，无法通过 MITM 代理。建议使用「写配置」方式配置模型"}
  ]
}
```

**不要**把 `HTTPS_PROXY` 写进 `~/.zshrc`。Agent **不需要**知道 Centag API Key（由服务端 MITM 注入）。

鉴权：

```bash
# WebUI → API Keys 创建个人 llmproxy_* Key（员工各自一把）
# 推荐命令行传入（优先级高于环境变量）：
centag wrap doctor --server http://<advertise>:20060 --token llmproxy_xxxxxxxx
centag wrap run --server http://<advertise>:20060 --token llmproxy_xxxxxxxx -- opencode

# 等价环境变量：
export CENTAG_WRAP_TOKEN='llmproxy_xxxxxxxx'
export CENTAG_API_BASE='http://127.0.0.1:20060'  # 可选
centag wrap doctor
```

- **仅本机 MITM（127.0.0.1）**：不强制代理鉴权；`--token` / `CENTAG_WRAP_TOKEN` 主要用于 setup/status。
- **开启「允许局域网」后**：MITM 对非本机客户端强制 `Proxy-Authorization`。`centag wrap run/env` 会把 Token 写入 `HTTPS_PROXY` userinfo，第三方 Agent **不必**配置 Centag Key。
- 未带 Token 的裸连 / 手写无凭证 `HTTPS_PROXY` 会收到 **407**。

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
| 主二进制（含 `centag wrap`） | `./start.sh build personal` / `make build` |
| 独立 wrap（仅本地/npm，**不进 GitHub Release**） | `./start.sh build wrap` |

## 系统 PAC（可选）

```bash
# 默认仅生成 CA 证书并信任，不修改系统代理设置
centag wrap enable [--server http://<advertise>:20060]

# 需要修改系统代理设置时，显式传入 --system-proxy
centag wrap enable --system-proxy [--server http://<advertise>:20060]

centag wrap doctor [--server …]
centag wrap disable   # 远端模式不关服务器 MITM
```

若 `setup/status` 需登录：`CENTAG_WRAP_TOKEN=<Bearer>`。

### NO_PROXY 默认值

`centag wrap run` 默认包含以下 NO_PROXY 值（可通过 `--no-proxy` 扩展）：

- `localhost,127.0.0.1,::1`
- `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16`（RFC1918 私有地址）
- `.localhost,.local,.lan,.example,.invalid`

```bash
# 示例：追加自定义 NO_PROXY
centag wrap run --no-proxy "*.internal.com,10.0.0.0/8" -- opencode
```

## 手写环境变量（等价于 run，一般不必）

```bash
curl -fsSL -o ~/.centag/wrap/ca.crt http://<advertise>:20060/api/v1/proxy/ca.crt
# LAN 时必须在代理 URL 中带 Token（与 wrap 一致）；本机可省略 userinfo
HTTPS_PROXY=http://:${CENTAG_WRAP_TOKEN}@<advertise>:8081 \
HTTP_PROXY=http://:${CENTAG_WRAP_TOKEN}@<advertise>:8081 \
NO_PROXY=localhost,127.0.0.1,::1 \
NODE_EXTRA_CA_CERTS=$HOME/.centag/wrap/ca.crt \
opencode
```

## 为何不影响其它上网

1. 代理变量只包住 Agent 进程。  
2. 非白名单域名：MITM 只做 CONNECT 隧道，不解密。  
3. 白名单 LLM API 才进 Centag；出口 Key 仅在服务端注入。

## 证书固定检测

当 MITM 代理检测到 TLS 握手失败或连接重置时，会记录失败信号。如果同一域名在 5 分钟内出现 5 次以上 TLS 握手失败，系统会提示「疑似证书固定」。

诊断命令：
```bash
# 检查域名是否被疑似证书固定
curl -s "http://localhost:20060/api/v1/wrap/doctor?domain=api.openai.com"
```

如果检测到证书固定，建议使用「写配置」方式配置模型，而非依赖透明代理模式。

## Agent 适用矩阵

| 类型 | 例子 | 推荐 |
|------|------|------|
| 进程级代理 | OpenCode 等 | `centag wrap run`（默认，CA-only） |
| 认系统 PAC | 部分桌面客户端 | `centag wrap run` 或 `centag wrap enable --system-proxy` |
| Electron/Chromium | Claude Desktop, CodeBuddy 等 | `centag wrap run`（自动设置 WinChromium） |
| 都不认 | 部分 Electron | Clash TUN（后续） |

### CA 证书管理

- `centag wrap enable` 默认仅生成 CA 证书并信任，**不修改系统代理设置**
- 需要系统 PAC 时显式传入 `--system-proxy`
- 菜单「移除 CA 信任」可从 Root/CA 存储中移除证书
- 桌面壳托盘提供「信任 CA 证书」和「移除 CA 信任」入口

## 鉴权与模型

- Agent 可填任意上游 Token；MITM 换成 Centag 出口 Key。  
- 上游真实密钥配在 Centag **后端 Provider**。  
- 透明模式改写为系统默认模型；改 Web「默认模型 / 默认后端」即可。  
- SSE 原样透传。

## 安全要点

- 默认 MITM 仅 loopback；Team 须显式开 LAN。  
- 开 LAN 后强制代理鉴权（个人 `CENTAG_WRAP_TOKEN`）；出口 Key 仍为服务端内部注入，不发给员工。  
- 勿开全局系统代理；勿污染登录 shell 环境。  
- 信任 CA 后白名单域名可被解密——仅可信内网。  

## 相关

- 主命令：`centag wrap …`（逻辑真源 `apps/wrap`）  
- Web：配置页 →「本机代理出口」
