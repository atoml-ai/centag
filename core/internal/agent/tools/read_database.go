package tools

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
)

// ReadDatabaseTool 读取数据库工具
type ReadDatabaseTool struct {
	db       *sql.DB
	allowedTables []string
}

// NewReadDatabaseTool 创建读取数据库工具
func NewReadDatabaseTool(db *sql.DB, allowedTables []string) agentcore.Tool {
	return &ReadDatabaseTool{
		db:       db,
		allowedTables: allowedTables,
	}
}

// Name 返回工具名称
func (t *ReadDatabaseTool) Name() string {
	return "read_database"
}

// Description 返回工具描述
func (t *ReadDatabaseTool) Description() string {
	return "读取centag数据库"
}

// Parameters 返回参数定义
func (t *ReadDatabaseTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"table": map[string]any{
				"type":        "string",
				"description": "表名",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "SQL查询语句（可选，仅支持单条只读 SELECT/WITH，且引用表必须在白名单内）",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "返回的行数限制（默认100行）",
			},
		},
		"required": []string{"table"},
	}
}

// IsReadOnly 返回是否为只读工具
func (t *ReadDatabaseTool) IsReadOnly() bool {
	return true
}

// ParamSchema 返回参数模式
func (t *ReadDatabaseTool) ParamSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"table": map[string]any{
				"type":        "string",
				"description": "表名",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "SQL查询语句（可选，仅支持单条只读 SELECT/WITH，且引用表必须在白名单内）",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "返回的行数限制（默认100行）",
			},
		},
		"required": []string{"table"},
	}
}

// Execute 执行工具
func (t *ReadDatabaseTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	table, _ := params["table"].(string)

	// table 为空时列出允许访问的表
	if table == "" {
		return &agentcore.ToolResult{Content: "可访问的表：\n" + strings.Join(t.allowedTables, "\n")}, nil
	}
	
	// 检查表是否允许访问
	allowed := false
	for _, allowedTable := range t.allowedTables {
		if strings.EqualFold(allowedTable, table) {
			allowed = true
			break
		}
	}
	
	if !allowed {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("表 %s 不允许访问", table)}, nil
	}
	
	// 构建查询
	query := fmt.Sprintf("SELECT * FROM %s", table)
	if queryParam, ok := params["query"].(string); ok && queryParam != "" {
		// 安全（R-P1）：自由 query 必须校验只读且引用表均在白名单内，
		// 否则可绕过 table 白名单读取任意表（如 users/api_keys）。
		if err := validateReadOnlyQuery(queryParam, t.allowedTables); err != nil {
			return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("query 校验失败: %v", err)}, nil
		}
		query = queryParam
	}
	
	// 添加限制
	limit := 100
	if limitParam, ok := params["limit"].(float64); ok {
		limit = int(limitParam)
	}
	
	// 执行查询
	rows, err := t.db.QueryContext(ctx, query)
	if err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("查询数据库失败: %v", err)}, nil
	}
	defer rows.Close()
	
	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("获取列名失败: %v", err)}, nil
	}
	
	// 读取数据
	var result []map[string]interface{}
	rowCount := 0
	
	for rows.Next() {
		if rowCount >= limit {
			break
		}
		
		// 创建值数组
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		// 扫描行
		if err := rows.Scan(valuePtrs...); err != nil {
			return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("扫描行失败: %v", err)}, nil
		}
		
		// 创建行数据
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			row[col] = val
		}
		
		result = append(result, row)
		rowCount++
	}
	
	if err := rows.Err(); err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("读取行失败: %v", err)}, nil
	}
	
	if len(result) == 0 {
		return &agentcore.ToolResult{Content: "没有找到数据"}, nil
	}
	
	// 转换为JSON字符串
	jsonStr := fmt.Sprintf("%v", result)
	return &agentcore.ToolResult{Content: jsonStr}, nil
}

// sqlWriteKeywords 自由 SQL 中一律拒绝的数据修改关键字（词边界匹配）。
// 覆盖写操作、DDL/DCL 与驱动侧扩展语句（如 PRAGMA/ATTACH）。
var sqlWriteKeywords = []string{
	"insert", "update", "delete", "merge", "replace", "upsert",
	"drop", "alter", "create", "truncate", "rename",
	"grant", "revoke", "attach", "detach", "pragma", "vacuum", "reindex",
}

// sqlWriteKeywordRes 预编译的写/DDL 关键字词边界正则。
var sqlWriteKeywordRes = func() []*regexp.Regexp {
	res := make([]*regexp.Regexp, 0, len(sqlWriteKeywords))
	for _, kw := range sqlWriteKeywords {
		res = append(res, regexp.MustCompile(`(?i)\b`+kw+`\b`))
	}
	return res
}()

// sqlTableRefRe 匹配 FROM/JOIN 后的表引用（含引号包裹、schema 限定与逗号列表）。
var sqlTableRefRe = regexp.MustCompile(`(?i)\b(?:from|join)\s+((?:["'\[\x60]?[A-Za-z_][\w$]*(?:\.[A-Za-z_][\w$]*)*["'\]\x60]?)(?:\s*,\s*["'\[\x60]?[A-Za-z_][\w$]*(?:\.[A-Za-z_][\w$]*)*["'\]\x60]?)*)`)

// sqlCTERe 匹配 CTE 定义名：WITH x AS (...) 或 ), y AS (...) / , z AS (...)。
var sqlCTERe = regexp.MustCompile(`(?i)(?:\bwith\b|[,(])\s*([A-Za-z_][\w$]*)\s+as\s*\(`)

// sqlIdentRe 合法表名标识符（去引号、去 schema 前缀后）。
var sqlIdentRe = regexp.MustCompile(`^[A-Za-z_][\w$]*$`)

// validateReadOnlyQuery 校验自定义 SQL 为单条只读查询，且所有引用表均在白名单内。
func validateReadOnlyQuery(query string, allowedTables []string) error {
	cleaned := strings.TrimSpace(stripSQLComments(query))
	if cleaned == "" {
		return fmt.Errorf("query 不能为空")
	}
	upper := strings.ToUpper(cleaned)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("仅允许只读查询（SELECT/WITH 开头）")
	}
	// 拒绝多语句（仅允许结尾一个分号）
	body := strings.TrimRight(cleaned, "; \t\n\r\t")
	if strings.Contains(body, ";") {
		return fmt.Errorf("仅允许单条查询语句")
	}
	// 词边界拒绝写/DDL 关键字（含 CTE 内 DELETE/UPDATE 等）
	for _, re := range sqlWriteKeywordRes {
		if loc := re.FindStringIndex(body); loc != nil {
			return fmt.Errorf("包含不允许的关键字 %s（仅支持只读查询）", strings.ToUpper(body[loc[0]:loc[1]]))
		}
	}
	tables := extractQueryTables(body)
	// CTE 别名是查询内部定义，不是物理表，跳过白名单校验
	cteNames := extractCTENames(body)
	cteSet := make(map[string]bool, len(cteNames))
	for _, c := range cteNames {
		cteSet[strings.ToLower(c)] = true
	}
	physical := 0
	for _, tb := range tables {
		if cteSet[strings.ToLower(tb)] {
			continue
		}
		physical++
		found := false
		for _, a := range allowedTables {
			if strings.EqualFold(a, tb) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("query 引用的表 %s 不在白名单内", tb)
		}
	}
	if physical == 0 {
		return fmt.Errorf("未能从 query 中识别任何物理表")
	}
	return nil
}

// extractCTENames 提取 WITH 子句定义的 CTE 别名。
func extractCTENames(query string) []string {
	var names []string
	for _, m := range sqlCTERe.FindAllStringSubmatch(query, -1) {
		names = append(names, m[1])
	}
	return names
}

// extractQueryTables 提取 FROM/JOIN 后引用的表名（含逗号列表与子查询；schema.table 取末段）。
func extractQueryTables(query string) []string {
	var tables []string
	seen := make(map[string]bool)
	for _, m := range sqlTableRefRe.FindAllStringSubmatch(query, -1) {
		for _, raw := range strings.Split(m[1], ",") {
			name := strings.TrimSpace(raw)
			name = strings.Trim(name, "\"'`[]")
			if idx := strings.LastIndex(name, "."); idx >= 0 {
				name = name[idx+1:]
			}
			if name == "" || !sqlIdentRe.MatchString(name) {
				continue
			}
			key := strings.ToLower(name)
			if !seen[key] {
				seen[key] = true
				tables = append(tables, name)
			}
		}
	}
	return tables
}

// stripSQLComments 去除 SQL 行注释（--）与块注释（/* */），避免注释内容干扰校验。
func stripSQLComments(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "--"):
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				i = len(s)
			} else {
				i += end + 4
			}
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}