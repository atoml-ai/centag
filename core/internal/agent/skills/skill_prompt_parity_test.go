package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// manifest 渲染的 system_prompt 与原 Skill.BuildPrompt 逐行对等（R06）。
// 关键断言：
//   - system_prompt 包含固定工具清单、执行规则关键句
//   - system_prompt 包含原 Steps 各条（编号）
//   - 与 BuildPrompt(userInput) 在排除用户请求段后一致
func TestSkillPromptParity(t *testing.T) {
	manifestDir := "../../../../config/initdata/pipeline-templates/common"
	builtinConstructors := map[string]func() *Skill{
		"status-check":       NewStatusCheckSkill,
		"config-analysis":    NewConfigAnalysisSkill,
		"error-diagnosis":    NewErrorDiagnosisSkill,
		"log-analysis":       NewLogAnalysisSkill,
		"strategy-recommend": NewStrategyRecommendSkill,
	}

	for name, constructor := range builtinConstructors {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(manifestDir, "agent-skill-"+name+".yaml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read manifest: %v", err)
			}
			p, err := ParseSkillPluginManifest(data)
			if err != nil {
				t.Fatalf("parse manifest: %v", err)
			}
			def := p.GetSkillDefinition()

			original := constructor()
			build := original.BuildPrompt("")

			// 1) 工具清单关键句存在
			for _, key := range []string{
				"你可以使用以下工具（必须实际调用",
				"- read_config：读取 centag 配置文件",
				"- read_log：读取 centag 日志文件",
				"- read_database：查询 centag 数据库",
				"- write_config：写入配置文件",
				"- analyze：分析数据并生成报告",
				"- system_info：获取当前操作系统",
				"- centag_info：获取 centag 数据目录",
				"不要臆测路径",
				"必须通过调用工具获取真实数据",
				"请按照以下步骤执行",
			} {
				if !strings.Contains(def.SystemPrompt, key) {
					t.Errorf("system_prompt missing key: %q", key)
				}
			}

			// 2) Steps 各条存在（编号形式）
			for i, step := range original.Steps {
				numbered := strings.Join([]string{string(rune('1' + i)), ". ", step}, "")
				if !strings.Contains(def.SystemPrompt, numbered) && !strings.Contains(def.SystemPrompt, step) {
					t.Errorf("system_prompt missing step[%d]: %q", i, step)
				}
			}

			// 3) 与 BuildPrompt 在用户请求段之前一致（前缀 + steps）
			wantPrefix := build
			if idx := strings.Index(build, "用户请求:"); idx >= 0 {
				wantPrefix = build[:idx]
			}
			if got := normalizeTrailing(def.SystemPrompt); got != normalizeTrailing(wantPrefix) {
				t.Errorf("system_prompt mismatch:\n--- manifest ---\n%s\n--- build ---\n%s", def.SystemPrompt, wantPrefix)
			}

			// 4) tools 声明对等
			if len(def.Tools) != len(original.Tools) {
				t.Errorf("tools len = %d, want %d", len(def.Tools), len(original.Tools))
			}
		})
	}
}

func normalizeTrailing(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}
