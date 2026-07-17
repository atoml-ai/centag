# 流水线模板清单

真源目录：`config/initdata/pipeline-templates/{common,gateway}/`（按 edition 加载，见 `editionDirMap`）。

> `common/` 供 minimal / gateway / team；`gateway/` 仅 gateway / team。依赖外置中间件或重型业务插件的模板放 `gateway/`。

## 内置模板

### common（全版本）

| 文件 | pipeline_id | 快捷码 | 说明 |
|------|-------------|--------|------|
| `direct-backend.yaml` | direct-backend | `#d` | 单 generator 直连 |
| `transparent-proxy.yaml` | transparent-proxy | `#t` | 透明代理（不注入 system） |
| `smart-scheduling.yaml` | smart-scheduling | `#s` | builtin.scheduler |
| `router-mode.yaml` | router-mode | `#r` | builtin.router 关键词/意图分支 |

### gateway（gateway / team）

| 文件 | pipeline_id | 快捷码 | 说明 |
|------|-------------|--------|------|
| `aggregator-mode.yaml` | aggregator-mode | `#ag` | 多路生成 → 聚合 |
| `fallback-mode.yaml` | fallback-mode | `#f` | 降级链 + 熔断 |
| `pipeline-mode.yaml` | pipeline-mode | — | 通用流水线示例 |
| `raw-forward.yaml` | raw-forward | — | Raw 转发 |
| `cache-hit.yaml` | cache-hit | `#ch` | 精确缓存优先 |
| `cache-mode.yaml` | cache-mode | — | 缓存模式 |
| `coding-agent.yaml` | coding-agent | — | Coding agent |
| `transparent-proxy-redis-example.yaml` | — | — | Redis 缓存示例 |

## 本地联调

```bash
./scripts/profile-ch-cache-demo.sh
./scripts/profile-transparent-demo.sh
```
