package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"edgeag/pkg/agentcore"
)

// CentagAgent centag Agent适配器
type CentagAgent struct {
	config     *AgentConfig
	agent      *agentcore.Agent
	backend    agentcore.LLMBackend
	registry   agentcore.ToolRegistry
	permission *CentagPermission
}

// NewCentagAgent 创建centag Agent
func NewCentagAgent(config *AgentConfig, backend agentcore.LLMBackend, registry agentcore.ToolRegistry) *CentagAgent {
	// 创建权限控制
	permission := NewCentagPermission(config, "")

	// 创建edgeag Agent
	opts := agentcore.AgentOptions{
		MaxTurns:  config.MaxTurns,
		MaxTokens: config.MaxTokens,
	}

	policy := agentcore.NewPermissionPolicy(config.Tools.Denied)
	agent := agentcore.NewAgent(backend, registry, opts, policy, &agentcore.RuntimeConfig{
		Timeout: config.Timeout,
	})

	return &CentagAgent{
		config:     config,
		agent:      agent,
		backend:    backend,
		registry:   registry,
		permission: permission,
	}
}

// Prompt 生成系统提示词
func (a *CentagAgent) Prompt(userInput string) string {
	return fmt.Sprintf(`你是一个centag运维助手，负责管理和诊断centag系统。

你可以：
1. 读取配置文件了解当前设置
2. 读取日志文件查找错误信息
3. 查询数据库获取运行时状态
4. 分析系统状态并提供建议

用户请求: %s

请按照以下步骤执行：
1. 使用read_config工具读取配置文件
2. 使用read_log工具读取日志文件（如果需要）
3. 使用read_database工具查询数据库（如果需要）
4. 使用analyze工具分析结果
5. 输出分析报告`, userInput)
}

// GetAgent 获取底层edgeag Agent
func (a *CentagAgent) GetAgent() *agentcore.Agent {
	return a.agent
}

// SetModel 设置模型
func (a *CentagAgent) SetModel(model string) {
	if a.agent != nil {
		a.agent.SetModel(model)
	}
}

// GetRegistry 获取工具注册表
func (a *CentagAgent) GetRegistry() agentcore.ToolRegistry {
	return a.registry
}

// CentagPermission centag权限控制
type CentagPermission struct {
	config  *AgentConfig
	dataDir string
}

// NewCentagPermission 创建centag权限控制
func NewCentagPermission(config *AgentConfig, dataDir string) *CentagPermission {
	return &CentagPermission{
		config:  config,
		dataDir: dataDir,
	}
}

// IsPathAllowed 检查路径是否允许访问
func (p *CentagPermission) IsPathAllowed(path string) (bool, error) {
	// 1. 解析绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	
	// 2. 检查是否在允许的目录中
	for _, allowedDir := range p.config.Filesystem.AllowedDirs {
		allowedPath := filepath.Join(p.dataDir, allowedDir)
		allowedAbs, _ := filepath.Abs(allowedPath)
		if strings.HasPrefix(absPath, allowedAbs) {
			return true, nil
		}
	}
	
	return false, nil
}

// IsTableAllowed 检查表是否允许访问
func (p *CentagPermission) IsTableAllowed(table string, isWrite bool) bool {
	// 检查是否在允许的表中
	allowed := false
	for _, t := range p.config.Database.AllowedTables {
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
		for _, t := range p.config.Database.ReadOnlyTables {
			if strings.EqualFold(t, table) {
				return false
			}
		}
	}
	
	return true
}

// IsToolAllowed 检查工具是否允许使用
func (p *CentagPermission) IsToolAllowed(toolName string) bool {
	// 检查是否在禁止列表中
	for _, t := range p.config.Tools.Denied {
		if strings.EqualFold(t, toolName) {
			return false
		}
	}
	
	// 检查是否在允许列表中
	for _, t := range p.config.Tools.Allowed {
		if strings.EqualFold(t, toolName) {
			return true
		}
	}
	
	return false
}

// IsConfirmRequired 检查是否需要确认
func (p *CentagPermission) IsConfirmRequired(toolName string) bool {
	for _, t := range p.config.Tools.RequireConfirm {
		if strings.EqualFold(t, toolName) {
			return true
		}
	}
	return false
}