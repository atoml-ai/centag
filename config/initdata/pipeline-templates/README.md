# 流水线模板清单

真源目录：`config/initdata/pipeline-templates/*.yaml`（随 bootstrap 加载）。

> 仅保留 **builtin** 核心模板。依赖 `business.*` 的模板已移出本仓，请通过外部业务插件仓库按需引入。

## 内置模板

| 文件 | pipeline_id | 快捷码 | 说明 |
|------|-------------|--------|------|
| `00-aggregator-mode.yaml` | aggregator-mode | `#ag` | 多路生成 → 聚合 |
| `02-transparent-fast.yaml` | transparent-fast | `#tf` | 无缓存透明转发 |
| `03-direct-backend.yaml` | direct-backend | `#d` | 单 generator 直连 |
| `04-fallback-mode.yaml` | fallback-mode | `#f` | 降级链 + 熔断 |
| `10-pipeline-mode.yaml` | pipeline-mode | — | 通用流水线示例 |
| `12-smart-scheduling.yaml` | smart-scheduling | `#s` | builtin.scheduler |
| `14-transparent-proxy.yaml` | transparent-proxy | `#t` | 透明代理 + 缓存 |
| `14-transparent-proxy-redis-example.yaml` | — | — | Redis 缓存示例 |
| `15-raw-forward.yaml` | raw-forward | — | Raw 转发 |
| `15-cache-hit.yaml` | cache-hit | `#ch` | 精确缓存优先 |
| `16-cache-mode.yaml` | cache-mode | — | 缓存模式 |
| `25-coding-agent.yaml` | coding-agent | — | Coding agent（builtin.generator） |

## 本地联调

```bash
./scripts/profile-ch-cache-demo.sh
./scripts/profile-transparent-demo.sh
```
