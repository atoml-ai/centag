# 计费规则 API

> 版本：v0.2.4 | 路径前缀：`/api/v1/admin/billing/rules`  
> 鉴权：JWT + Admin（team/personal 的 `/api/v1/admin`）；minimal 挂在受保护 `/api/v1/admin/billing/rules`

## 规则管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/admin/billing/rules` | 列出规则 |
| POST | `/api/v1/admin/billing/rules` | 创建规则 |
| PUT | `/api/v1/admin/billing/rules/:id` | 更新规则 |
| DELETE | `/api/v1/admin/billing/rules/:id` | 删除规则 |
| POST | `/api/v1/admin/billing/rules/import` | YAML 导入（替换全部） |
| GET | `/api/v1/admin/billing/rules/export` | YAML 导出 |

### 规则 JSON 字段

```json
{
  "name": "PPIO DeepSeek V3.2",
  "backend_id": "ppinfra",
  "model": "deepseek-v3.2",
  "input_price_per_m": 0.1389,
  "output_price_per_m": 0.1389,
  "currency": "USD",
  "priority": 100,
  "enabled": true
}
```

`model` / `backend_id` 支持通配符 `*`。匹配时 `priority` 高者优先；同分更具体匹配优先。  
**单价与落库金额均为 USD**。

### YAML

```yaml
version: "1.0"
currency: "USD"
usd_to_cny: 7.2
rules: [...]
```

- 导入/导出默认 `currency: USD`
- 若导入文件为 `CNY`，服务端按 `usd_to_cny` 换算为 USD 后存储
- `usd_to_cny` 写入运行时，供前端显示换算

## 成本汇总（既有接口，增强）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/admin/cost/summary` | 按维度聚合成本（team） |

查询参数：`from`、`to`（`YYYY-MM-DD`）、`group_by`（`model|backend|date|tenant|dept|agent_type`）、`tenant_id`。

响应：

- `currency`：始终为 `USD`（金额正本）
- `usd_to_cny`：显示用汇率
- `total_cost_usd` / `cost_usd`：USD 金额（列名历史兼容）

**不会**提供平行路径 `/api/v1/admin/billing/costs*`。前端若选人民币显示，自行 `amount * usd_to_cny`。

## 预置配置

默认种子：`config/pricing/default.yaml`（可用环境变量 `CENTAG_PRICING_FILE` 覆盖）。  
personal/team：表空时自动导入规则，并始终从该文件加载汇率；minimal：启动加载到内存 RuleStore。
