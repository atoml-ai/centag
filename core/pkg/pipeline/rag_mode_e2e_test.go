package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"centag/core/pkg/bootstrap"
)

func TestRAGModeTemplate_ValidateAndTopology(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadRAGTemplate(t)
	p := CreatePipelineFromTemplate(tmpl, nil)
	if p == nil {
		t.Fatal("CreatePipelineFromTemplate returned nil")
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p.ShortcutCode != "#rag" {
		t.Fatalf("shortcut = %q, want #rag", p.ShortcutCode)
	}

	graph := NewExecutionGraph(p)
	layers, err := graph.LayeredTopologicalSort()
	if err != nil {
		t.Fatalf("LayeredTopologicalSort: %v", err)
	}
	if len(layers) < 2 {
		t.Fatalf("layer_count = %d, want >= 2", len(layers))
	}
	if layers[0][0] != "cache_read" {
		t.Fatalf("first layer = %v, want cache_read first", layers[0])
	}

	nodeByID := map[string]PipelineNodeConfig{}
	for _, n := range p.Nodes {
		nodeByID[n.ID] = n
	}
	for _, id := range []string{"question_splitter", "rag_retrieval", "generator", "answer_synthesizer", "cache_write"} {
		n, ok := nodeByID[id]
		if !ok {
			t.Fatalf("missing node %s", id)
		}
		if n.Condition == "" {
			t.Fatalf("node %s should have cache_miss condition", id)
		}
	}
	if deps := nodeByID["token_usage"].DependsOn; len(deps) != 1 || deps[0] != "cache_write" {
		t.Fatalf("token_usage depends_on = %v", deps)
	}
	if cc := nodeByID["answer_synthesizer"].Config.CustomConfig; cc == nil || cc["enable_citation"] != true {
		t.Fatalf("answer_synthesizer should enable_citation=true, got %v", cc)
	}
}

func TestRAGMode_CacheHitSkipsRetrievalPath(t *testing.T) {
	execCtx := NewExecutionContext(nil)
	execCtx.SetResult("cache_read", &NodeOutput{
		Metadata: map[string]interface{}{"cache_hit": true},
	})
	eval := NewConditionEvaluator(execCtx)
	cond := "{{.cache_read.metadata.cache_hit}} == false"
	if eval.Evaluate(cond) {
		t.Fatal("expected cache_hit=true to skip miss-only nodes")
	}
}

func TestRAGMode_CacheMissRunsRetrievalPath(t *testing.T) {
	execCtx := NewExecutionContext(nil)
	execCtx.SetResult("cache_read", &NodeOutput{
		Metadata: map[string]interface{}{"cache_hit": false},
	})
	eval := NewConditionEvaluator(execCtx)
	cond := "{{.cache_read.metadata.cache_hit}} == false"
	if !eval.Evaluate(cond) {
		t.Fatal("expected cache_hit=false to run miss path")
	}
}

func mustProjectRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return root
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		dir := filepath.Join(root, "config", "initdata", "pipeline-templates")
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			// Found the templates dir; treat as project root.
			return root
		}
		// 兼容测试 fixture 目录
		if _, err := os.Stat(filepath.Join(root, "pipeline-templates", "README.md")); err == nil {
			return root
		}
		root = filepath.Dir(root)
	}
	t.Fatal("project root not found (config/initdata/pipeline-templates 或 ./pipeline-templates 均未找到)")
	return ""
}

func mustLoadRAGTemplate(t *testing.T) PatternTemplate {
	t.Helper()
	for _, raw := range bootstrap.LoadInitialPipelineTemplatesFromFiles() {
		if raw.ID == "rag-mode" {
			return convertBootstrapTemplate(raw)
		}
	}
	t.Skip("rag-mode template not in builtin initdata (see pipeline-templates README)")
	return PatternTemplate{}
}

func convertBootstrapTemplate(raw bootstrap.InitialPipelineTemplate) PatternTemplate {
	tmpl := PatternTemplate{
		ID:            raw.ID,
		SchemaVersion: raw.SchemaVersion,
		Name:          raw.Name,
		Description:   raw.Description,
		ShortcutCode:  raw.ShortcutCode,
		Metadata:      raw.Metadata,
	}
	if raw.GlobalConfig != nil {
		tmpl.GlobalConfig = &GlobalPipelineConfig{
			Timeout:       raw.GlobalConfig.Timeout,
			MaxRetries:    raw.GlobalConfig.MaxRetries,
			BypassOnError: raw.GlobalConfig.BypassOnError,
			ParallelLimit: raw.GlobalConfig.ParallelLimit,
			LogLevel:      raw.GlobalConfig.LogLevel,
		}
		if raw.GlobalConfig.Storage != nil {
			tmpl.GlobalConfig.StorageConfig = &StorageHookConfig{
				Enabled:       raw.GlobalConfig.Storage.Enabled,
				Namespace:     raw.GlobalConfig.Storage.Namespace,
				AutoSave:      raw.GlobalConfig.Storage.AutoSave,
				SaveInterval:  raw.GlobalConfig.Storage.SaveInterval,
				RetentionDays: raw.GlobalConfig.Storage.RetentionDays,
			}
		}
		for _, h := range raw.GlobalConfig.Hooks {
			hook := HookConfig{
				Type: h.Type,
				On:   append([]string{}, h.On...),
			}
			if h.Config != nil {
				hook.Config = make(map[string]interface{}, len(h.Config))
				for k, v := range h.Config {
					hook.Config[k] = v
				}
			}
			tmpl.GlobalConfig.Hooks = append(tmpl.GlobalConfig.Hooks, hook)
		}
	}
	for _, node := range raw.Nodes {
		pn := PipelineNodeConfig{
			ID:             node.ID,
			Type:           NodeType(node.Type),
			Kind:           node.Kind,
			Implementation: node.Implementation,
			Name:           node.Name,
			Backend:        node.Backend,
			Model:          node.Model,
			Config: NodeConfig{
				Backend:        node.Config.Backend,
				Model:          node.Config.Model,
				PromptTemplate: node.Config.PromptTemplate,
				SystemPrompt:   node.Config.SystemPrompt,
				CustomConfig:   node.Config.CustomConfig,
				TemplateVars:   node.Config.TemplateVars,
			},
			Inputs:      node.Inputs,
			Timeout:     node.Timeout,
			Condition:   node.Condition,
			DependsOn:   node.DependsOn,
			NextNodes:   node.NextNodes,
			RouteConfig: convertRouteConfig(node.RouteConfig),
		}
		if node.Retry != nil {
			pn.Retry = &RetryConfig{
				MaxAttempts:     node.Retry.MaxAttempts,
				BackoffStrategy: node.Retry.BackoffStrategy,
				InitialDelay:    node.Retry.InitialDelay,
				MaxDelay:        node.Retry.MaxDelay,
			}
		}
		tmpl.Nodes = append(tmpl.Nodes, pn)
	}
	// 归一化所有节点
	for i := range tmpl.Nodes {
		tmpl.Nodes[i].Normalize()
	}
	return tmpl
}

func convertRouteConfig(rc *bootstrap.InitialRouteConfig) *RouteConfig {
	if rc == nil {
		return nil
	}
	return &RouteConfig{
		RouterNodeID: rc.RouterNodeID,
		RouteValue:   rc.RouteValue,
		IsDefault:    rc.IsDefault,
	}
}

