# 流水线模板清单

真源目录：`config/initdata/pipeline-templates/{common,team,extras}/`（按 edition 加载，见 `editionDirMap`）。

| 子目录 | 发行版 |
|--------|--------|
| `common/` | **minimal / personal / team** 均加载 |
| `team/` | **仅 team** 发行版（原 `personal/` 目录已改名） |
| `extras/` | 扩展模板 |

> personal / minimal 一键安装包与 Docker Profile **只打包 `common/`**。  
> team 目录下的模板仅在构建 team 版本时带上。

## 内置模板

### common（minimal / personal / team）

| 文件 | pipeline_id | 快捷码 | 说明 |
|------|-------------|--------|------|
| `transparent.yaml` | transparent | `#t` | 统一透传流水线（合并 transparent-proxy / direct-backend / fixed-egress） |
| `router-pipeline.yaml` | router-pipeline | `#r` | 通用路由流水线（合并 router-mode，支持 keyword/llm_classify/hybrid 策略） |
| `cache-pipeline.yaml` | cache-pipeline | `#cache` | 统一缓存流水线（合并 cache-hit / cache-mode / 18-rag-mode） |
| `agent-skill-router.yaml` | centag-ops-router | `#ops` | Centag 运维技能路由（LLM 意图分类） |
| `agent-skill-status-check.yaml` | — | — | skill manifest：状态检查 |
| `agent-skill-config-analysis.yaml` | — | — | skill manifest：配置分析 |
| `agent-skill-error-diagnosis.yaml` | — | — | skill manifest：错误诊断 |
| `agent-skill-log-analysis.yaml` | — | — | skill manifest：日志分析 |
| `agent-skill-strategy-recommend.yaml` | — | — | skill manifest：策略建议 |
| `smart-scheduling.yaml` | smart-scheduling | `#s` | builtin.scheduler |

### extras

| 文件 | pipeline_id | 快捷码 | 说明 |
|------|-------------|--------|------|
| `coding-agent.yaml` | coding-agent | — | Coding agent |
| `education-agent.yaml` | education-agent | — | Education agent |

### team（仅 team）

| 文件 | pipeline_id | 快捷码 | 说明 |
|------|-------------|--------|------|
| `aggregator-mode.yaml` | aggregator-mode | `#ag` | 多路生成 → 聚合 |
| `pipeline-mode.yaml` | pipeline-mode | — | 通用流水线示例 |
| `transparent-proxy-redis-example.yaml` | — | — | Redis 缓存示例 |

> **注意**：旧模板（transparent-proxy、direct-backend、fixed-egress、router-mode、cache-hit、cache-mode、18-rag-mode）已合并删除。旧流水线 ID 通过代码级兼容层 `PipelineAliases` 自动映射到新流水线，已部署实例不受影响。

## 本地联调

```bash
./scripts/profile-ch-cache-demo.sh
./scripts/profile-transparent-demo.sh
```
