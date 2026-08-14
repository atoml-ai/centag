package agent

import (
	"time"
)

// AgentConfig Agent配置
type AgentConfig struct {
	Enabled     bool          `json:"enabled" yaml:"enabled"`
	MaxTurns    int           `json:"max_turns" yaml:"max_turns"`
	MaxTokens   int           `json:"max_tokens" yaml:"max_tokens"`
	Timeout     time.Duration `json:"timeout" yaml:"timeout"`
	ToolTimeout time.Duration `json:"tool_timeout" yaml:"tool_timeout"`
	
	// 文件系统权限
	Filesystem FilesystemConfig `json:"filesystem" yaml:"filesystem"`
	
	// 数据库权限
	Database DatabaseConfig `json:"database" yaml:"database"`
	
	// 工具权限
	Tools ToolsConfig `json:"tools" yaml:"tools"`
	
	// Skill配置
	Skills SkillsConfig `json:"skills" yaml:"skills"`
}

// FilesystemConfig 文件系统权限配置
type FilesystemConfig struct {
	AllowedDirs []string `json:"allowed_dirs" yaml:"allowed_dirs"`
	DeniedDirs  []string `json:"denied_dirs" yaml:"denied_dirs"`
}

// DatabaseConfig 数据库权限配置
type DatabaseConfig struct {
	AllowedTables  []string `json:"allowed_tables" yaml:"allowed_tables"`
	ReadOnlyTables []string `json:"read_only_tables" yaml:"read_only_tables"`
	DeniedTables   []string `json:"denied_tables" yaml:"denied_tables"`
}

// ToolsConfig 工具权限配置
type ToolsConfig struct {
	Allowed        []string `json:"allowed" yaml:"allowed"`
	Denied         []string `json:"denied" yaml:"denied"`
	RequireConfirm []string `json:"require_confirm" yaml:"require_confirm"`
}

// SkillsConfig Skill配置
type SkillsConfig struct {
	InternalOnly bool     `json:"internal_only" yaml:"internal_only"`
	Enabled      []string `json:"enabled" yaml:"enabled"`
}

// DefaultAgentConfig 返回默认配置
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		Enabled:     false,
		MaxTurns:    20,
		MaxTokens:   4096,
		Timeout:     10 * time.Minute,
		ToolTimeout: 30 * time.Second,
		
		Filesystem: FilesystemConfig{
			AllowedDirs: []string{
				"config",
				"logs",
				"data",
				"var",
			},
			DeniedDirs: []string{
				"bin",
				"lib",
				"tmp",
			},
		},
		
		Database: DatabaseConfig{
			AllowedTables: []string{
				"agent_sessions",
				"agent_messages",
			},
			ReadOnlyTables: []string{
				"system_config",
				"backends",
				"pipelines",
				"pipeline_templates",
			},
			DeniedTables: []string{
				"users",
				"api_keys",
				"billing",
			},
		},
		
		Tools: ToolsConfig{
			Allowed: []string{
				"read_config",
				"read_log",
				"read_database",
				"write_config",
				"analyze",
				"system_info",
				"centag_info",
			},
			Denied: []string{
				"bash",
				"exec",
				"run_command",
				"shell",
			},
			RequireConfirm: []string{
				"write_config",
			},
		},
		
		Skills: SkillsConfig{
			InternalOnly: true,
			Enabled: []string{
				"status-check",
				"config-analysis",
				"error-diagnosis",
				"log-analysis",
				"strategy-recommend",
			},
		},
	}
}