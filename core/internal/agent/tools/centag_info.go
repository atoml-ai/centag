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
	dataDir string
	dbPath  string
}

// NewCentagInfoTool 创建 centag 信息工具
func NewCentagInfoTool(dataDir, dbPath string) agentcore.Tool {
	return &CentagInfoTool{dataDir: dataDir, dbPath: dbPath}
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
	b.WriteString("- 可用表: backends(后端配置), system_config(系统配置), pipelines(流水线), token_usage(用量), api_keys, users, audit_logs(审计日志), agent_sessions/agent_messages(agent会话) 等\n")

	// 配置说明
	b.WriteString("\n## 配置说明\n")
	b.WriteString("- 配置存储在数据库 system_config 表（key-value JSON），无独立 yaml/json 文件\n")
	b.WriteString("- 后端配置在 backends 表；默认后端/模型在 system_config\n")
	b.WriteString("- 配置文件读取可用 read_config 工具，或 read_database 查询 system_config/backends 表\n")

	// 关键目录
	b.WriteString("\n## 目录结构\n")
	b.WriteString(fmt.Sprintf("- 配置/数据: %s/lib/<edition>/ 或 %s/var/\n", t.dataDir, t.dataDir))
	b.WriteString("- 日志: 通常位于 <data_dir>/lib/<edition>/logs/ 或 <data_dir>/var/logs/\n")

	return &agentcore.ToolResult{Content: b.String()}, nil
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
