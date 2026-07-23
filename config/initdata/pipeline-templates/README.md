# 流水线模板清单

真源目录：`config/initdata/pipeline-templates/{common,team}/`（按 edition 加载，见 `editionDirMap`）。

| 子目录 | 发行版 |
|--------|--------|
| `common/` | **minimal / personal / team** 均加载 |
| `team/` | **仅 team** 发行版（原 `personal/` 目录已改名） |

> personal / minimal 一键安装包与 Docker Profile **只打包 `common/`**。  
> team 目录下的模板仅在构建 team 版本时带上。

## 内置模板

### common（minimal / personal / team）

| 文件 | pipeline_id | 快捷码 | 说明 |
|------|-------------|--------|------|
| `direct-backend.yaml` | direct-backend | `#d` | 直连（共用 `transparent_forward`：fixed + 注入 system） |
| `transparent-proxy.yaml` | transparent-proxy | `#t` | 透明（共用节点：按 model 选路） |
| `fixed-egress.yaml` | fixed-egress | `#j` | 跳板（共用节点：固定出站、不注入） |
| `smart-scheduling.yaml` | smart-scheduling | `#s` | builtin.scheduler |
| `router-mode.yaml` | router-mode | `#r` | builtin.router |
| `coding-agent.yaml` | coding-agent | — | Coding agent |
| `education-agent.yaml` | education-agent | — | Education agent |

### team（仅 team）

| 文件 | pipeline_id | 快捷码 | 说明 |
|------|-------------|--------|------|
| `aggregator-mode.yaml` | aggregator-mode | `#ag` | 多路生成 → 聚合 |
| `fallback-mode.yaml` | fallback-mode | `#f` | 降级链 + 熔断 |
| `pipeline-mode.yaml` | pipeline-mode | — | 通用流水线示例 |
| `cache-hit.yaml` | cache-hit | `#ch` | 精确缓存优先 |
| `cache-mode.yaml` | cache-mode | — | 缓存模式 |
| `transparent-proxy-redis-example.yaml` | — | — | Redis 缓存示例 |

## 本地联调

```bash
./scripts/profile-ch-cache-demo.sh
./scripts/profile-transparent-demo.sh
```
