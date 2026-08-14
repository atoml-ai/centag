package tools

import (
	"database/sql"

	"edgeag/pkg/agentcore"
)

// ToolRegistry 工具注册表
type ToolRegistry struct {
	registry agentcore.ToolRegistry
}

// NewToolRegistry 创建工具注册表
func NewToolRegistry(dataDir string, db *sql.DB, allowedTables []string) *ToolRegistry {
	registry := agentcore.NewToolRegistry()
	
	// 注册内置工具
	registry.Register(NewReadConfigTool(dataDir))
	registry.Register(NewReadLogTool(dataDir))
	registry.Register(NewReadDatabaseTool(db, allowedTables))
	registry.Register(NewWriteConfigTool(dataDir))
	registry.Register(NewAnalyzeTool())
	
	return &ToolRegistry{
		registry: registry,
	}
}

// GetRegistry 获取底层工具注册表
func (r *ToolRegistry) GetRegistry() agentcore.ToolRegistry {
	return r.registry
}

// ListTools 列出所有工具
func (r *ToolRegistry) ListTools() []agentcore.Tool {
	return r.registry.ListTools()
}