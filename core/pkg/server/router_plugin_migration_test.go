package server

import (
	"fmt"
	"testing"

	"centag/core/pkg/pipeline"
)

type migrationTestStore struct {
	failUpdate bool
	updatedIDs []string
}

func (m *migrationTestStore) Create(p *pipeline.AgentPatternPipeline) error { return nil }
func (m *migrationTestStore) CreateForTenant(tenantID string, p *pipeline.AgentPatternPipeline) error {
	return nil
}
func (m *migrationTestStore) Get(id string) (*pipeline.AgentPatternPipeline, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *migrationTestStore) GetByTenant(tenantID, id string) (*pipeline.AgentPatternPipeline, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *migrationTestStore) GetByShortcutCode(code string) (*pipeline.AgentPatternPipeline, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *migrationTestStore) Update(p *pipeline.AgentPatternPipeline) error {
	if m.failUpdate {
		return fmt.Errorf("update failed")
	}
	m.updatedIDs = append(m.updatedIDs, p.ID)
	return nil
}
func (m *migrationTestStore) Delete(id string) error                                { return nil }
func (m *migrationTestStore) DeleteForTenant(tenantID, id string) error             { return nil }
func (m *migrationTestStore) List() ([]*pipeline.AgentPatternPipeline, error)       { return nil, nil }
func (m *migrationTestStore) ListByTenant(tenantID string) ([]*pipeline.AgentPatternPipeline, error) {
	return nil, nil
}
func (m *migrationTestStore) ListEnabled() ([]*pipeline.AgentPatternPipeline, error) { return nil, nil }
func (m *migrationTestStore) ListEnabledByTenant(tenantID string) ([]*pipeline.AgentPatternPipeline, error) {
	return nil, nil
}
func (m *migrationTestStore) RecordExecution(log *pipeline.ExecutionRecord) error { return nil }
func (m *migrationTestStore) GetExecutionHistory(pipelineID string, limit int) ([]*pipeline.ExecutionRecord, error) {
	return nil, nil
}
func (m *migrationTestStore) GetExecution(id int64) (*pipeline.ExecutionRecord, error) { return nil, nil }

func TestMigrateRouterImplementationInPipeline(t *testing.T) {
	p := &pipeline.AgentPatternPipeline{
		ID:   "test",
		Name: "test",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:             "router-builtin",
				Type:           pipeline.NodeTypeRouter,
				Implementation: "builtin.router",
			},
			{
				ID:             "router-empty",
				Type:           pipeline.NodeTypeRouter,
				Implementation: "",
			},
			{
				ID:             "router-custom",
				Type:           pipeline.NodeTypeRouter,
				Implementation: "business.custom_router",
			},
			{
				ID:             "gen",
				Type:           pipeline.NodeTypeGenerator,
				Implementation: "builtin.generator",
			},
		},
	}

	changed, count := migrateRouterImplementationInPipeline(p)
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if count != 2 {
		t.Fatalf("expected updated nodes=2, got %d", count)
	}
	if got := p.Nodes[0].Implementation; got != "business.router" {
		t.Fatalf("expected node0 implementation business.router, got %s", got)
	}
	if got := p.Nodes[1].Implementation; got != "business.router" {
		t.Fatalf("expected node1 implementation business.router, got %s", got)
	}
	if got := p.Nodes[2].Implementation; got != "business.custom_router" {
		t.Fatalf("expected node2 implementation unchanged, got %s", got)
	}
	if got := p.Nodes[3].Implementation; got != "builtin.generator" {
		t.Fatalf("expected generator implementation unchanged, got %s", got)
	}
}

func TestMigrateRouterImplementationInPipeline_NoChange(t *testing.T) {
	p := &pipeline.AgentPatternPipeline{
		ID:   "test2",
		Name: "test2",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:             "router-custom",
				Type:           pipeline.NodeTypeRouter,
				Implementation: "business.router",
			},
			{
				ID:             "gen",
				Type:           pipeline.NodeTypeGenerator,
				Implementation: "builtin.generator",
			},
		},
	}

	changed, count := migrateRouterImplementationInPipeline(p)
	if changed {
		t.Fatalf("expected changed=false")
	}
	if count != 0 {
		t.Fatalf("expected updated nodes=0, got %d", count)
	}
}

func TestMigrateRouterImplementationsToBusinessPlugin(t *testing.T) {
	registry := pipeline.NewPipelineRegistry()
	store := &migrationTestStore{}

	p1 := &pipeline.AgentPatternPipeline{
		ID:   "router-mode",
		Name: "router-mode",
		Nodes: []pipeline.PipelineNodeConfig{
			{ID: "router", Type: pipeline.NodeTypeRouter, Implementation: "builtin.router"},
			{ID: "gen", Type: pipeline.NodeTypeGenerator, Implementation: "builtin.generator"},
		},
	}
	p2 := &pipeline.AgentPatternPipeline{
		ID:   "custom",
		Name: "custom",
		Nodes: []pipeline.PipelineNodeConfig{
			{ID: "router", Type: pipeline.NodeTypeRouter, Implementation: "business.router"},
		},
	}
	if err := registry.Register(p1); err != nil {
		t.Fatalf("register p1: %v", err)
	}
	if err := registry.Register(p2); err != nil {
		t.Fatalf("register p2: %v", err)
	}

	updatedPipelines, updatedNodes, err := migrateRouterImplementationsToBusinessPlugin(registry, store)
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if updatedPipelines != 1 || updatedNodes != 1 {
		t.Fatalf("expected pipelines=1 nodes=1, got pipelines=%d nodes=%d", updatedPipelines, updatedNodes)
	}
	if len(store.updatedIDs) != 1 || store.updatedIDs[0] != "router-mode" {
		t.Fatalf("unexpected updated IDs: %v", store.updatedIDs)
	}
}

func TestMigrateRouterImplementationsToBusinessPlugin_UpdateError(t *testing.T) {
	registry := pipeline.NewPipelineRegistry()
	store := &migrationTestStore{failUpdate: true}

	p := &pipeline.AgentPatternPipeline{
		ID:   "router-mode",
		Name: "router-mode",
		Nodes: []pipeline.PipelineNodeConfig{
			{ID: "router", Type: pipeline.NodeTypeRouter, Implementation: "builtin.router"},
		},
	}
	if err := registry.Register(p); err != nil {
		t.Fatalf("register pipeline: %v", err)
	}

	_, _, err := migrateRouterImplementationsToBusinessPlugin(registry, store)
	if err == nil {
		t.Fatalf("expected migrate error")
	}
}

