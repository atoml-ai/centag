package agent

import (
	"path/filepath"
	"strings"
)

// PermissionManager 权限管理器
type PermissionManager struct {
	config  *AgentConfig
	dataDir string
}

// NewPermissionManager 创建权限管理器
func NewPermissionManager(config *AgentConfig, dataDir string) *PermissionManager {
	return &PermissionManager{
		config:  config,
		dataDir: dataDir,
	}
}

// IsPathAllowed 检查路径是否允许访问
func (m *PermissionManager) IsPathAllowed(path string) (bool, error) {
	// 1. 解析绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	
	// 2. 解析符号链接（防止符号链接攻击）
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// 文件不存在时使用绝对路径
		realPath = absPath
	}
	
	// 3. 检查是否在允许的目录中
	for _, allowedDir := range m.config.Filesystem.AllowedDirs {
		allowedPath := filepath.Join(m.dataDir, allowedDir)
		allowedAbs, _ := filepath.Abs(allowedPath)
		if strings.HasPrefix(realPath, allowedAbs) {
			return true, nil
		}
	}
	
	return false, nil
}

// GetDeniedPath 获取拒绝访问的路径（用于错误信息）
func (m *PermissionManager) GetDeniedPath(path string) string {
	return "路径 " + path + " 不在允许访问的范围内，只能访问centag目录内的文件"
}

// IsTableAllowed 检查表是否允许访问
func (m *PermissionManager) IsTableAllowed(table string, isWrite bool) bool {
	// 检查是否在允许的表中
	allowed := false
	for _, t := range m.config.Database.AllowedTables {
		if strings.EqualFold(t, table) {
			allowed = true
			break
		}
	}
	
	if !allowed {
		return false
	}
	
	// 如果是写操作，检查是否为只读表
	if isWrite {
		for _, t := range m.config.Database.ReadOnlyTables {
			if strings.EqualFold(t, table) {
				return false
			}
		}
	}
	
	return true
}

// GetDeniedTable 获取拒绝访问的表（用于错误信息）
func (m *PermissionManager) GetDeniedTable(table string, isWrite bool) string {
	if isWrite {
		return "表 " + table + " 不允许写入，只能写入agent_sessions和agent_messages表"
	}
	return "表 " + table + " 不允许访问"
}

// IsToolAllowed 检查工具是否允许使用
func (m *PermissionManager) IsToolAllowed(toolName string) bool {
	// 检查是否在禁止列表中
	for _, t := range m.config.Tools.Denied {
		if strings.EqualFold(t, toolName) {
			return false
		}
	}
	
	// 检查是否在允许列表中
	for _, t := range m.config.Tools.Allowed {
		if strings.EqualFold(t, toolName) {
			return true
		}
	}
	
	return false
}

// IsConfirmRequired 检查是否需要确认
func (m *PermissionManager) IsConfirmRequired(toolName string) bool {
	for _, t := range m.config.Tools.RequireConfirm {
		if strings.EqualFold(t, toolName) {
			return true
		}
	}
	return false
}

// IsSkillAllowed 检查skill是否允许使用
func (m *PermissionManager) IsSkillAllowed(skillName string) bool {
	// 如果只允许内置skill，检查skill是否在启用列表中
	if m.config.Skills.InternalOnly {
		for _, s := range m.config.Skills.Enabled {
			if strings.EqualFold(s, skillName) {
				return true
			}
		}
		return false
	}
	
	return true
}