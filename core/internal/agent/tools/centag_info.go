package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
)

// CentagInfoTool 返回 centag 运行时信息（数据目录结构、日志/数据库路径、配置说明）
type CentagInfoTool struct {
	dataDir       string
	dbPath        string
	allowedTables []string
}

// NewCentagInfoTool 创建 centag 信息工具
func NewCentagInfoTool(dataDir, dbPath string, allowedTables []string) agentcore.Tool {
	return &CentagInfoTool{dataDir: dataDir, dbPath: dbPath, allowedTables: allowedTables}
}

// Name 返回工具名称
func (t *CentagInfoTool) Name() string {
	return "centag_info"
}

// Description 返回工具描述
func (t *CentagInfoTool) Description() string {
	return "获取 centag 系统信息：数据目录结构、日志文件路径、数据库路径与可用表、配置文件说明。分析日志/配置/数据库前先调用此工具了解路径"
}

// Parameters 返回参数定义
func (t *CentagInfoTool) Parameters() any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

// IsReadOnly 返回是否为只读工具
func (t *CentagInfoTool) IsReadOnly() bool {
	return true
}

// ParamSchema 返回参数模式
func (t *CentagInfoTool) ParamSchema() map[string]any {
	return t.Parameters().(map[string]any)
}

// Execute 执行工具
func (t *CentagInfoTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	var b strings.Builder

	b.WriteString("== centag 系统信息 ==\n\n")
	b.WriteString(fmt.Sprintf("数据目录 (data_dir): %s\n", t.dataDir))

	// 探测日志路径
	b.WriteString("\n## 日志文件\n")
	logPaths := t.findLogPaths()
	if len(logPaths) == 0 {
		b.WriteString("- 未找到日志文件\n")
	} else {
		for _, p := range logPaths {
			b.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	// 数据库
	b.WriteString("\n## 数据库\n")
	if t.dbPath != "" {
		b.WriteString(fmt.Sprintf("- 路径: %s\n", t.dbPath))
		if fi, err := os.Stat(t.dbPath); err == nil {
			b.WriteString(fmt.Sprintf("- 大小: %d 字节\n", fi.Size()))
		}
	}
	b.WriteString("\n### 当前白名单内可直接查询的表\n")
	b.WriteString("(read_database 空 table 调用同样返回此清单；不在清单内的表需管理员扩展 agent.database.allowed_tables 配置)\n")
	if len(t.allowedTables) == 0 {
		b.WriteString("- (未配置，read_database 将拒绝所有查询)\n")
	} else {
		for _, tb := range t.allowedTables {
			b.WriteString(fmt.Sprintf("- %s%s\n", tb, tableCatalog[tb]))
		}
	}
	b.WriteString("\n### 全库表目录（按域分组）\n")
	for _, g := range tableCatalogGroups {
		b.WriteString("- " + g + "\n")
	}

	// 配置说明
	b.WriteString("\n## 配置说明\n")
	b.WriteString("- 配置存储在数据库 system_config 表（key-value JSON），无独立 yaml/json 文件\n")
	b.WriteString("- 后端配置在 backends 表；默认后端/模型在 system_config\n")
	b.WriteString("- 流水线与模式：pipelines/preset_modes/mode_mappings；调度决策留痕在 scheduler_decisions\n")
	b.WriteString("- 计费策略：pricing_rules（规则）/plan_templates（套餐模板）/*_plans/*_quotas/*_pricing_overrides（套餐、配额与价格覆盖）\n")
	b.WriteString("- 运行观测：token_usage/token_usage_daily（用量）、pipeline_executions（执行记录）、cache_savings（缓存节省）、user_request_logs（请求日志）\n")
	b.WriteString("- 配置文件读取可用 read_config 工具，或 read_database 查询 system_config/backends 表\n")

	// 关键目录
	b.WriteString("\n## 目录结构\n")
	b.WriteString(fmt.Sprintf("- 配置/数据: %s/lib/<edition>/ 或 %s/var/\n", t.dataDir, t.dataDir))
	b.WriteString("- 日志: 通常位于 <data_dir>/lib/<edition>/logs/ 或 <data_dir>/var/logs/\n")

	return &agentcore.ToolResult{Content: b.String()}, nil
}

// tableCatalog 表名 → 一句话用途说明（用于白名单清单行内标注）。
var tableCatalog = map[string]string{
	"backends":                 "(后端配置)",
	"system_config":            "(系统配置KV)",
	"pipelines":                "(流水线)",
	"preset_modes":             "(预设模式)",
	"mode_mappings":            "(模式映射)",
	"clash_rules":              "(代理规则)",
	"user_config":              "(用户配置)",
	"token_usage":              "(用量明细)",
	"token_usage_daily":        "(日聚合用量)",
	"token_usage_daily_new":    "(日聚合用量-新表)",
	"pipeline_executions":      "(流水线执行记录)",
	"scheduler_decisions":      "(调度决策留痕)",
	"cache_savings":            "(缓存节省统计)",
	"user_request_logs":        "(请求日志)",
	"pricing_rules":            "(计费规则)",
	"plan_templates":           "(套餐模板)",
	"user_plans":               "(用户套餐实例)",
	"user_plan_assignments":    "(用户套餐绑定)",
	"group_plans":              "(分组套餐实例)",
	"group_plan_assignments":   "(分组套餐绑定)",
	"user_pricing_overrides":   "(用户价格覆盖)",
	"group_pricing_overrides":  "(分组价格覆盖)",
	"token_quotas":             "(令牌配额)",
	"team_quota":               "(团队配额)",
	"tenant_quotas":            "(租户配额)",
	"tenants":                  "(租户)",
	"tenant_usage":             "(租户用量)",
	"tenant_api_keys":          "(租户密钥)",
	"agent_sessions":           "(Agent会话)",
	"agent_messages":           "(Agent消息)",
	"conversation_sessions":    "(会话)",
	"conversation_messages":    "(会话消息)",
	"agent_memory_docs":        "(Agent记忆文档)",
	"agent_memory_chunks":      "(Agent记忆分块)",
	"users":                    "(用户)",
	"api_keys":                 "(API密钥)",
	"groups":                   "(分组)",
	"audit_logs":               "(审计日志)",
	"refresh_tokens":           "(刷新令牌)",
	"plugin_registry":          "(插件注册)",
	"plugin_market_registry":   "(插件市场注册)",
	"plugin_market_ratings":    "(插件市场评分)",
	"ab_eval_results":          "(AB评估结果)",
	"schema_migrations":        "(迁移版本)",
}

// tableCatalogGroups 全库表目录（按域分组），供自我分析定位数据源。
var tableCatalogGroups = []string{
	"核心配置: backends(后端配置), system_config(系统配置KV), pipelines(流水线), preset_modes(预设模式), mode_mappings(模式映射), clash_rules(代理规则), user_config(用户配置)",
	"运行观测: token_usage(用量明细), token_usage_daily/token_usage_daily_new(日聚合用量), pipeline_executions(流水线执行记录), scheduler_decisions(调度决策留痕), cache_savings(缓存节省统计), user_request_logs(请求日志), ab_eval_results(AB评估结果)",
	"计费配额: pricing_rules(计费规则), plan_templates(套餐模板), user_plans/user_plan_assignments(用户套餐及绑定), group_plans/group_plan_assignments(分组套餐及绑定), user_pricing_overrides/group_pricing_overrides(价格覆盖), token_quotas/team_quota/tenant_quotas(配额), tenants(租户), tenant_usage(租户用量), tenant_api_keys(租户密钥)",
	"会话记忆: agent_sessions/agent_messages(Agent会话与消息), conversation_sessions/conversation_messages(会话与消息), agent_memory_docs/agent_memory_chunks(Agent记忆文档与分块)",
	"平台安全: users(用户), api_keys(API密钥), groups(分组), audit_logs(审计日志), refresh_tokens(刷新令牌), plugin_registry/plugin_market_registry/plugin_market_ratings(插件注册与市场)",
}

// findLogPaths 探测常见日志路径
func (t *CentagInfoTool) findLogPaths() []string {
	var out []string
	seen := map[string]bool{}

	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			seen[p] = true
			out = append(out, p)
		}
	}

	// 常见路径
	add(filepath.Join(t.dataDir, "logs", "centag.log"))
	add(filepath.Join(t.dataDir, "log", "centag.log"))
	add(filepath.Join(t.dataDir, "var", "logs", "centag.log"))
	add(filepath.Join(t.dataDir, "lib", "personal", "logs", "centag.log"))
	add(filepath.Join(t.dataDir, "lib", "minimal", "logs", "centag.log"))
	add(filepath.Join(t.dataDir, "lib", "team", "logs", "centag.log"))

	// 递归探测 lib 下所有 *.log
	libDir := filepath.Join(t.dataDir, "lib")
	_ = filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".log") {
			add(path)
		}
		return nil
	})

	return out
}
