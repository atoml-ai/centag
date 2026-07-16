# Profile: team — 团队版（多用户 / 多租户）

**目标**：在**单独部署的中间件**（PostgreSQL、向量等）上运行团队共享入口；支持多租户与负载均衡水平扩展。

> 与 **gateway** 的二进制插件集合**对齐**（同为全功能）。差别在部署：team 默认依赖外部 PG / 向量等；gateway 默认内置 SQLite。详见 [`docs/guide/dist-profiles.md`](../../../docs/guide/dist-profiles.md)。

## 特点

- **CENTAG_EDITION=team**：启用用户管理、多租户、系统更新等团队功能
- **PostgreSQL 等中间件单独部署**：共享元数据（stack `centag-postgresql` 等）
- **就绪探针**：`/health/ready` 验证数据库连接，适合 K8s / Compose healthcheck
- **可选 HA**：`docker-compose.ha.yaml` + Nginx 对多副本 `centag` 做负载均衡

## 与 gateway / cached 的区别

| 维度 | gateway | cached | **team** |
|------|---------|--------|----------|
| 二进制插件 | 全功能 | （视所用 Dist） | **与 gateway 相同** |
| Edition | 个人侧默认配置 | team | **team（显式）** |
| 数据库默认 | **内置 SQLite** | PostgreSQL | **外部 PostgreSQL** |
| 中间件 | 默认不强制；可配置外接 | stack PG 等 | **默认单独部署** |
| 主要场景 | 个人全功能 | 缓存加速 | **团队运维 / 多用户** |
| 水平扩展 | 否 | 否 | **可选（HA overlay）** |

## 单节点启动

```bash
cd /path/to/centag

cp config/profiles/team/.env.example config/profiles/team/.env
vim config/profiles/team/.env   # 修改管理员密码与 API Key

./start.sh profile team up
```

## 验证

```bash
# 存活探针（进程级）
curl -s http://localhost:20060/health

# 就绪探针（含数据库）
curl -s http://localhost:20060/health/ready

# 版本信息
curl -s http://localhost:20060/api/v1/status | jq '.edition'
# 预期: "team"
```

## 水平扩展（可选）

在 `config/profiles/team` 目录使用 HA overlay，将入口切换到 Nginx，并扩展 `centag` 副本数：

```bash
cd config/profiles/team

docker compose \
  -f docker-compose.yaml \
  -f docker-compose.stack-network.yaml \
  -f docker-compose.ha.yaml \
  up -d --scale centag=2
```

- 对外端口仍为 `LLM_PROXY_SERVER_PORT`（默认 20060），由 `centag-lb` 监听
- 各 `centag` 副本不直接暴露宿主机端口
- Docker 内置 DNS 将 `centag` 解析为全部副本 IP，Nginx `upstream` 自动轮询

### Kubernetes 探针示例

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 20060
  initialDelaySeconds: 15
readinessProbe:
  httpGet:
    path: /health/ready
    port: 20060
  initialDelaySeconds: 20
  periodSeconds: 10
```

## 配置说明

见 `.env.example`。关键变量：

| 变量 | 说明 |
|------|------|
| `CENTAG_EDITION` | 固定为 `team` |
| `PG_HOST` | stack 中 PostgreSQL 服务名 |
| `LLM_PROXY_ADMIN_*` | 首次空库时创建的管理员 |

## 故障排查

```bash
./start.sh profile team logs
./start.sh profile team status

# 数据库未就绪时 /health/ready 返回 503
curl -v http://localhost:20060/health/ready
```