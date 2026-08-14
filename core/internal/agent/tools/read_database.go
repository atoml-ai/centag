package tools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"edgeag/pkg/agentcore"
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
				"description": "SQL查询语句（可选）",
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
				"description": "SQL查询语句（可选）",
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