package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
)

// AnalyzeTool 分析工具
// 职责边界：本工具只产出统计事实（规模、级别分布、高频异常行、数值概览），
// 结论性判断由 LLM 基于这些事实完成。
type AnalyzeTool struct{}

// NewAnalyzeTool 创建分析工具
func NewAnalyzeTool() agentcore.Tool {
	return &AnalyzeTool{}
}

// Name 返回工具名称
func (t *AnalyzeTool) Name() string {
	return "analyze"
}

// Description 返回工具描述
func (t *AnalyzeTool) Description() string {
	return "分析数据并生成报告"
}

// Parameters 返回参数定义
func (t *AnalyzeTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "string",
				"description": "要分析的数据",
			},
			"type": map[string]any{
				"type":        "string",
				"description": "分析类型（status, config, error, log, strategy）",
				"enum":        []string{"status", "config", "error", "log", "strategy"},
			},
		},
		"required": []string{"data", "type"},
	}
}

// IsReadOnly 返回是否为只读工具
func (t *AnalyzeTool) IsReadOnly() bool {
	return true
}

// ParamSchema 返回参数模式
func (t *AnalyzeTool) ParamSchema() map[string]any {
	return t.Parameters().(map[string]any)
}

// Execute 执行工具
func (t *AnalyzeTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	data, ok := params["data"].(string)
	if !ok {
		return &agentcore.ToolResult{IsError: true, Content: "missing 'data' parameter"}, nil
	}

	analysisType, ok := params["type"].(string)
	if !ok {
		return &agentcore.ToolResult{IsError: true, Content: "missing 'type' parameter"}, nil
	}

	switch analysisType {
	case "status", "config", "error", "log", "strategy":
	default:
		return &agentcore.ToolResult{IsError: true, Content: fmt.Sprintf("不支持的分析类型: %s", analysisType)}, nil
	}

	return &agentcore.ToolResult{Content: summarizeData(data, analysisType)}, nil
}

// severityMarkers 级别标记 → 归一化级别（按行匹配，大小写不敏感）。
var severityMarkers = []struct {
	marker string
	level  string
}{
	{"fatal", "FATAL"}, {"panic", "FATAL"},
	{"error", "ERROR"}, {"err", "ERROR"}, {"失败", "ERROR"}, {"错误", "ERROR"},
	{"warn", "WARN"}, {"警告", "WARN"},
	{"info", "INFO"},
	{"debug", "DEBUG"}, {"trace", "DEBUG"},
}

// maxTopLines 异常行 TOP N 上限。
const maxTopLines = 5

// maxLineSample 单行采样截断长度，避免超长行撑爆上下文。
const maxLineSample = 200

// lineStat 单个异常行的频次统计。
type lineStat struct {
	line  string
	count int
}

// summarizeData 对任意文本数据生成结构化统计摘要：
// 规模 / 级别分布 / 高频异常行 TOP N / 数值列概览 / 类型相关关注点。
func summarizeData(data, analysisType string) string {
	lines := strings.Split(data, "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "=== 分析摘要 (%s) ===\n\n", analysisType)

	// 规模
	b.WriteString("## 规模\n")
	fmt.Fprintf(&b, "- 行数: %d；字符数: %d\n", len(lines), len(data))

	// 级别分布（按行计数，避免子串重复计数）
	levels := map[string]int{}
	unmarked := 0
	freq := map[string]int{}
	for _, ln := range lines {
		lower := strings.ToLower(ln)
		matched := false
		for _, m := range severityMarkers {
			if strings.Contains(lower, m.marker) {
				levels[m.level]++
				matched = true
				break
			}
		}
		if !matched {
			unmarked++
		}
		// 收集疑似异常行用于频次统计
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") ||
			strings.Contains(lower, "fatal") || strings.Contains(lower, "panic") ||
			strings.Contains(lower, "warn") || strings.Contains(lower, "错误") ||
			strings.Contains(lower, "失败") {
			key := strings.TrimSpace(ln)
			if key != "" && len(key) <= maxLineSample {
				freq[key]++
			} else if key != "" {
				freq[key[:maxLineSample]]++
			}
		}
	}

	b.WriteString("\n## 级别分布（按关键词逐行归类）\n")
	order := []string{"FATAL", "ERROR", "WARN", "INFO", "DEBUG"}
	for _, lv := range order {
		if levels[lv] > 0 {
			fmt.Fprintf(&b, "- %s: %d 行\n", lv, levels[lv])
		}
	}
	if unmarked > 0 {
		fmt.Fprintf(&b, "- 未标记: %d 行\n", unmarked)
	}

	// 高频异常行
	if len(freq) > 0 {
		stats := make([]lineStat, 0, len(freq))
		for k, v := range freq {
			stats = append(stats, lineStat{k, v})
		}
		sort.Slice(stats, func(i, j int) bool {
			if stats[i].count != stats[j].count {
				return stats[i].count > stats[j].count
			}
			return stats[i].line < stats[j].line
		})
		b.WriteString("\n## 异常行 TOP " + strconv.Itoa(maxTopLines) + "（按出现次数）\n")
		for i, s := range stats {
			if i >= maxTopLines {
				break
			}
			fmt.Fprintf(&b, "%d. [x%d] %s\n", i+1, s.count, truncate(s.line, maxLineSample))
		}
	}

	// 数值列概览（针对 CSV/TSV 或含数字的表格数据）
	if nums := numericOverview(lines); nums != "" {
		b.WriteString("\n## 数值概览\n" + nums)
	}

	// 类型相关关注点
	b.WriteString("\n## 建议关注点\n")
	for _, tip := range typeTips(analysisType, levels) {
		b.WriteString("- " + tip + "\n")
	}
	b.WriteString("- 本工具仅输出统计事实；根因分析与结论请基于以上数据自行完成\n")

	return b.String()
}

// typeTips 按分析类型给出关注点提示。
func typeTips(analysisType string, levels map[string]int) []string {
	tips := map[string][]string{
		"status": {
			"对照 backends/system_config 确认运行时状态与配置一致",
			"存在 ERROR/FATAL 时优先检查后端连通性与熔断状态",
		},
		"config": {
			"核对 mode→pipeline 映射与 pipelines 表实际定义是否一致",
			"确认默认后端/模型指向有效条目",
		},
		"error": {
			"结合 scheduler_decisions/pipeline_executions 定位首次出错环节",
			"区分瞬时错误与持续性错误（看时间分布）",
		},
		"log": {
			"先看异常行 TOP 是否集中在同一组件/同一时间窗",
			"对照 token_usage/user_request_logs 判断是否伴随流量突增",
		},
		"strategy": {
			"用量类数据建议按天聚合对比趋势（token_usage_daily）",
			"评估缓存收益用 cache_savings；路由质量用 pipeline_executions 成功率",
		},
	}
	out := append([]string{}, tips[analysisType]...)
	if levels["ERROR"]+levels["FATAL"] > 0 {
		out = append(out, fmt.Sprintf("检测到 %d 行错误级别记录，优先处理", levels["ERROR"]+levels["FATAL"]))
	}
	return out
}

// numericOverview 提取逗号/制表符分隔数据的数值列 min/max/sum/count（仅前 1000 行采样）。
func numericOverview(lines []string) string {
	const sampleMax = 1000
	type acc struct {
		count int
		min   float64
		max   float64
		sum   float64
	}
	cols := map[int]*acc{}
	colNames := map[int]string{}
	sampled := 0
	for _, ln := range lines {
		if sampled >= sampleMax {
			break
		}
		fields := strings.FieldsFunc(ln, func(r rune) bool { return r == ',' || r == '\t' })
		if len(fields) < 2 {
			continue
		}
		sampled++
		for i, f := range fields {
			f = strings.TrimSpace(strings.Trim(f, `"'`))
			if v, err := strconv.ParseFloat(f, 64); err == nil {
				a := cols[i]
				if a == nil {
					a = &acc{min: v, max: v}
					cols[i] = a
				}
				a.count++
				a.sum += v
				if v < a.min {
					a.min = v
				}
				if v > a.max {
					a.max = v
				}
			} else if _, isNum := cols[i]; !isNum && colNames[i] == "" {
				colNames[i] = f
			}
		}
	}
	if len(cols) == 0 {
		return ""
	}
	idxs := make([]int, 0, len(cols))
	for i := range cols {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)
	var b strings.Builder
	for _, i := range idxs {
		a := cols[i]
		name := colNames[i]
		if name == "" {
			name = fmt.Sprintf("第%d列", i+1)
		}
		fmt.Fprintf(&b, "- %s: count=%d min=%g max=%g sum=%g\n", name, a.count, a.min, a.max, a.sum)
	}
	return b.String()
}

// truncate 截断超长字符串。
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
