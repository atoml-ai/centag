package tools

import (
	"reflect"
	"testing"
)

// TestIntersectAllowedTools 空/部分/完全交集三用例（任务8 / R02 验收）。
func TestIntersectAllowedTools(t *testing.T) {
	all := []string{"read_config", "read_log", "read_database", "write_config", "analyze", "system_info", "centag_info"}

	// 完全交集
	got := IntersectAllowedTools(all, all)
	if !reflect.DeepEqual(got, all) {
		t.Errorf("full intersect = %v, want %v", got, all)
	}

	// 部分交集：保持 skill 声明顺序
	skillTools := []string{"analyze", "read_database", "read_config"}
	want := []string{"analyze", "read_database", "read_config"}
	got = IntersectAllowedTools(skillTools, all)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("partial intersect = %v, want %v", got, want)
	}

	// 空交集
	got = IntersectAllowedTools([]string{"nonexistent_tool"}, all)
	if len(got) != 0 {
		t.Errorf("empty intersect = %v, want none", got)
	}

	// skill 声明被白名单拒绝的部分
	got = IntersectAllowedTools([]string{"write_config", "read_config"}, []string{"read_config"})
	if !reflect.DeepEqual(got, []string{"read_config"}) {
		t.Errorf("intersect with denial = %v, want [read_config]", got)
	}
}
