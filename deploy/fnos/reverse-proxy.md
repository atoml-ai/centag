# fnOS 部署：HTTPS / 反向代理指南

> 适用：Centag v0.3.3（Native 模式，`/vol1/@appcenter/centag`）| 创建：2026-08-13

## 1. 背景与结论

Centag 自身**只监听明文 HTTP**（默认 `0.0.0.0:20060`），**无内置 TLS 终止**，也不自带证书签发。

因此「HTTPS / 反向代理」是**部署运维层问题**，不是产品缺陷，需要由外部 Web 服务器（反向代理）完成：

- **TLS 终止**（证书卸载）；
- **证书签发与续期**（内部 CA 或 Let's Encrypt）；
- **暴露面收口**（Centag 只绑回环地址，仅代理端口对外）。

当前部署形态（巡检事实）：

| 项 | 值 |
|----|----|
| 监听 | `0.0.0.0:20060` 明文 HTTP |
| fnOS 应用入口 | `ui/config`：`protocol: http`、端口 `20060`，桌面直达 `http://<NAS-IP>:20060/` |
| 主机环境 | 飞牛 NAS（fnOS），已运行 Docker（如 `pansou-web:20080`）、PG `5432` |

风险点：admin 密码、API Key、代理流量均以明文在网内传输；若端口映射到公网，等于明文暴露全部凭据。

## 2. 场景判断

| 场景 | 建议 |
|------|------|
| 纯内网 + 网络可信（家中/办公室） | 可维持 HTTP；至少建议：不对外映射 `20060`，仅局域网访问 |
| 内网但网段内有不可信设备/访客 Wi-Fi | 推荐加 TLS（反向代理 + 内部 CA） |
| 需要公网访问（远程办公、暴露到公网） | **必须**反向代理 + TLS（公网域名用 Let's Encrypt，无域名用内部 CA + 隧道） |

> 公网暴露场景下，除 TLS 外还应配合：强密码 / 更换默认 API Key / 限 IP / 限速。

## 3. 方案一：Caddy（Docker，推荐）

Caddy 自动申请/续期证书、自动 HTTP/2，配置最短。NAS 已启用 Docker，可直接运行：

```bash
mkdir -p /vol1/docker/caddy && cat > /vol1/docker/caddy/Caddyfile <<'EOF'
centag.example.com {            # 局域网可用: 内网主机名（见 §5 hosts 方案）
    reverse_proxy 127.0.0.1:20060
}
EOF

docker run -d --name caddy \
  --restart unless-stopped \
  -p 443:443 -p 80:80 \
  -v /vol1/docker/caddy/Caddyfile:/etc/caddy/Caddyfile:ro \
  -v /vol1/docker/caddy/data:/data \
  -v /vol1/docker/caddy/config:/config \
  caddy:latest
```

- 有公网域名且 80/443 可达 → Caddy 自动申请 Let's Encrypt 证书并续期，无需任何证书配置；
- 仅内网使用（无公网验证）→ 用内部 CA，Caddy 配置改为：

```caddyfile
centag.local {
    tls /vol1/docker/caddy/certs/centag.local.pem /vol1/docker/caddy/certs/centag.local-key.pem
    reverse_proxy 127.0.0.1:20060
}
```

## 4. 方案二：nginx（Docker 或系统 nginx）

若更习惯 nginx，可复用主机上的 nginx（`nginx: master process`），或单独起容器。关键配置：

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 443 ssl http2;
    server_name centag.local;

    ssl_certificate     /etc/ssl/certs/centag.local.pem;
    ssl_certificate_key /etc/ssl/private/centag.local-key.pem;

    location / {
        proxy_pass         http://127.0.0.1:20060;
        proxy_http_version 1.1;

        # 透传真实客户端信息
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket（对话/流式会话）
        proxy_set_header Upgrade    $http_upgrade;
        proxy_set_header Connection $connection_upgrade;

        # SSE / 流式透传：关闭缓冲，长读超时
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

> `map` 指令必须放在 `http{}` 层（如 `/etc/nginx/nginx.conf`），`server` 块再引用 `$connection_upgrade`。

## 5. 内部 CA：mkcert（局域网无公网域名）

无公网域名且不愿暴露 80/443 时，用 `mkcert` 建内部私有 CA，客户端装根证书即可信任：

```bash
# 在 NAS 上（或任一台管理机）安装 mkcert
brew install mkcert nss        # macOS；Linux 用 apt/dnf 或二进制
mkcert -install                # 生成并信任本地 CA（根证书在 ~/.local/share/mkcert）

# 为「内网主机名 + 内网 IP」签发证书
mkcert -cert-file centag.local.pem -key-file centag.local-key.pem \
    centag.local 192.168.1.4 localhost

# 把根证书（mkcert -CAROOT/rootCA.pem）分发给各客户端设备：
#   - macOS/iOS: 安装描述文件 → 设置 → 证书信任设置 → 开启完全信任
#   - Windows: 导入到「受信任的根证书颁发机构」
#   - Android: 安装到用户/系统信任库
```

客户端解析 `centag.local`：

- 简单方式：在客户端 `/etc/hosts`（或 NAS 的 DHCP/DNS）加 `192.168.1.4 centag.local`；
- 多设备统一方式：在 NAS 上跑 AdGuard Home / dnsmasq，局域网所有设备自动解析。

## 6. Centag 侧配置（收口暴露面）

代理就绪后，让 Centag **只监听回环地址**，杜绝绕过代理直连明文口：

1. 修改 `/vol1/@appcenter/centag/config/runtime.env`：

   ```bash
   SERVER_HOST='127.0.0.1'
   LLM_PROXY_SERVER_HOST='127.0.0.1'
   ```

2. 重启应用（fnOS 应用中心 → Centag → 停止/启动，或 `cmd/main` 重启）；
3. 验证 `20060` 已不再对局域网开放：从另一台设备 `curl -v http://192.168.1.4:20060/` 应连接失败/超时；
4. **不要**再对 `20060` 做公网端口映射；只暴露代理端口 `443`。

> 反向代理方案同样适用于 WebSocket/SSE（§3/§4 已带配置）；若 Centag 部署在 Docker 而非 Native，同理改容器环境变量 `SERVER_HOST` 并重建容器。

## 7. 验证清单

```bash
# 1) TLS 握手与证书信任（客户端应 0 告警）
curl -v https://centag.local/ 2>&1 | grep -iE 'SSL|subject|issuer'

# 2) WebUI 可登录（admin 密码）
open https://centag.local/

# 3) 流式/SSE 正常（proxy_buffering off 生效，输出不攒批）
curl -N https://centag.local/v1/chat/completions \
  -H 'Authorization: Bearer <API_KEY>' -H 'Content-Type: application/json' \
  -d '{"model":"deepseek-v4-flash","stream":true,"messages":[{"role":"user","content":"hi"}]}'

# 4) X-Forwarded-For 生效（管理端日志出现真实客户端 IP）

# 5) 明文口已收口：非本机访问 http://192.168.1.4:20060/ 失败
```

## 8. 回滚

- 还原 `runtime.env` 中 `SERVER_HOST` 为 `0.0.0.0` 并重启应用；
- 停止/删除反向代理容器（`docker rm -f caddy`），移除 iptables/Docker 端口映射；
- 如需彻底移除内部 CA 信任：客户端执行 `mkcert -uninstall`（本机）或删除已安装的根证书。
