package groupmodel

import "testing"

// 回归：流水线合并后旧 ID（transparent-proxy 等）必须与现行 ID 互相匹配，
// 否则 Team 计划白名单会出现“允许新名拒绝旧名”（或反之）的裂缝。
func TestIsAllowedPipelineLegacyAlias(t *testing.T) {
	pol := &EffectivePolicy{
		ResourcesConfigured: true,
		AllowPipelines:      []string{"transparent", "smart-scheduling", "centag-ops-router", "cache-pipeline"},
	}
	for _, pid := range []string{
		"transparent",       // 现行 ID
		"transparent-proxy", // 旧 ID → transparent
		"direct-backend",    // 旧 ID → transparent
		"fixed-egress",      // 旧 ID → transparent
		"router-mode",       // 旧 ID → router-pipeline？不在名单 → 应拒绝
	} {
		want := pid != "router-mode"
		if got := pol.IsAllowedPipeline(pid); got != want {
			t.Fatalf("IsAllowedPipeline(%q) = %v, want %v", pid, got, want)
		}
	}

	// 反向：白名单存的是旧名（历史数据），请求用新名也应放行。
	old := &EffectivePolicy{
		ResourcesConfigured: true,
		AllowPipelines:      []string{"transparent-proxy", "router-mode"},
	}
	for _, pid := range []string{"transparent", "router-pipeline"} {
		if !old.IsAllowedPipeline(pid) {
			t.Fatalf("legacy allowlist should admit canonical %q", pid)
		}
	}

	// 白名单外仍拒绝。
	if pol.IsAllowedPipeline("router-pipeline") {
		t.Fatal("router-pipeline outside allowlist should be denied")
	}

	// 未配置资源（custom 模式）一律拒绝，别名不改变 fail-closed 语义。
	none := &EffectivePolicy{AllowPipelines: []string{"transparent"}}
	if none.IsAllowedPipeline("transparent") {
		t.Fatal("ResourcesConfigured=false must deny")
	}
}
