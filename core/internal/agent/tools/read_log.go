package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
)

// ReadLogTool 读取日志文件工具
type ReadLogTool struct {
	dataDir string
}

// NewReadLogTool 创建读取日志文件工具
func NewReadLogTool(dataDir string) agentcore.Tool {
	return &ReadLogTool{
		dataDir: dataDir,
	}
}

// Name 返回工具名称
func (t *ReadLogTool) Name() string {
	return "read_log"
}

// Description 返回工具描述
func (t *ReadLogTool) Description() string {
	return "读取centag日志文件"
}

// Parameters 返回参数定义
func (t *ReadLogTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "日志文件路径（相对于centag数据目录）",
			},
			"lines": map[string]any{
				"type":        "integer",
				"description": "读取的行数（默认100行）",
			},
			"filter": map[string]any{
				"type":        "string",
				"description": "过滤关键词",
			},
		},
		"required": []string{"path"},
	}
}

// IsReadOnly 返回是否为只读工具
func (t *ReadLogTool) IsReadOnly() bool {
	return true
}

// ParamSchema 返回参数模式
func (t *ReadLogTool) ParamSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "日志文件路径（相对于centag数据目录）",
			},
			"lines": map[string]any{
				"type":        "integer",
				"description": "读取的行数（默认100行）",
			},
			"filter": map[string]any{
				"type":        "string",
				"description": "过滤关键词",
			},
		},
		"required": []string{"path"},
	}
}

// Execute 执行工具
func (t *ReadLogTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	path, _ := params["path"].(string)
	
	// 获取可选参数
	lines := 100
	if linesParam, ok := params["lines"].(float64); ok {
		lines = int(linesParam)
	}
	
	filter := ""
	if filterParam, ok := params["filter"].(string); ok {
		filter = filterParam
	}

	// path 为空时列出数据目录下可读的日志文件，帮助模型选择
	if path == "" {
		return listLogCandidates(t.dataDir)
	}

	// 路径隔离校验（任务9 / R03）：拒绝逃逸 dataDir 的路径
	fullPath, err := secureResolve(t.dataDir, path)
	if err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("打开日志文件失败: %v", err)}, nil
	}

	// 打开文件
	file, err := os.Open(fullPath)
	if err != nil {
		// 相对 dataDir 未找到时，尝试探测常见日志位置
		if alt := findLogByBasename(t.dataDir, filepath.Base(path)); alt != "" {
			fullPath = alt
			file, err = os.Open(fullPath)
		}
		if err != nil {
			return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("打开日志文件失败: %v。可用日志文件：\n%s", err, listLogCandidatesContent(t.dataDir))}, nil
		}
	}
	defer file.Close()
	
	// 读取文件
	var result []string
	scanner := bufio.NewScanner(file)
	lineCount := 0
	
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		
		// 应用过滤器
		if filter != "" && !strings.Contains(line, filter) {
			continue
		}
		
		result = append(result, line)
		
		// 限制行数
		if len(result) >= lines {
			break
		}
	}
	
	if err := scanner.Err(); err != nil {
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("读取日志文件失败: %v", err)}, nil
	}
	
	if len(result) == 0 {
		return &agentcore.ToolResult{Content: "没有找到匹配的日志条目"}, nil
	}
	
	return &agentcore.ToolResult{Content: strings.Join(result, "\n")}, nil
}

// resolvePath 解析路径：绝对路径原样返回，否则相对 dataDir
func resolvePath(dataDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dataDir, p)
}

// findLogByBasename 按文件名在数据目录下递归查找日志
func findLogByBasename(dataDir, base string) string {
	var found string
	_ = filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), base) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// listLogCandidatesContent 返回日志候选列表文本
func listLogCandidatesContent(dataDir string) string {
	res, _ := listLogCandidates(dataDir)
	if res == nil {
		return "（未找到日志文件）"
	}
	return res.Content
}

// listLogCandidates 列出数据目录下的日志文件候选（path 为空时帮助模型选择）
func listLogCandidates(dataDir string) (*agentcore.ToolResult, error) {
	var b strings.Builder
	b.WriteString("未指定日志路径。请指定 path 参数（绝对路径或相对数据目录），可选日志文件：\n")

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > 5 {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			cur := filepath.Join(dir, e.Name())
			if e.IsDir() {
				walk(cur, depth+1)
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, ".log") {
				rel, _ := filepath.Rel(dataDir, cur)
				fmt.Fprintf(&b, "- %s\n", rel)
			}
		}
	}
	walk(dataDir, 0)

	if !strings.Contains(b.String(), "- ") {
		b.WriteString("（未找到日志文件，请检查数据目录）\n")
	}
	return &agentcore.ToolResult{Content: b.String()}, nil
}