package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// FilePipelineStore persists pipelines as YAML files under a directory.
// Used by the minimal edition (no database). Tenant APIs degrade to global files.
type FilePipelineStore struct {
	dir string
	mu  sync.Mutex
}

// NewFilePipelineStore creates a YAML-backed pipeline store.
// dir is typically <dataDir>/pipeline-templates.
func NewFilePipelineStore(dir string) (*FilePipelineStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("pipeline file store dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create pipeline store dir: %w", err)
	}
	return &FilePipelineStore{dir: dir}, nil
}

func (s *FilePipelineStore) pathFor(id string) string {
	return filepath.Join(s.dir, id+".yaml")
}

func (s *FilePipelineStore) write(pipeline *AgentPatternPipeline) error {
	if pipeline == nil || pipeline.ID == "" {
		return fmt.Errorf("pipeline id is required")
	}
	for i := range pipeline.Nodes {
		pipeline.Nodes[i].Normalize()
	}
	data, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("marshal pipeline %s: %w", pipeline.ID, err)
	}
	tmp := s.pathFor(pipeline.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp pipeline file: %w", err)
	}
	if err := os.Rename(tmp, s.pathFor(pipeline.ID)); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename pipeline file: %w", err)
	}
	return nil
}

func (s *FilePipelineStore) readFile(path string) (*AgentPatternPipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p AgentPatternPipeline
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if p.ID == "" {
		base := filepath.Base(path)
		p.ID = strings.TrimSuffix(base, filepath.Ext(base))
	}
	for i := range p.Nodes {
		p.Nodes[i].Normalize()
	}
	return &p, nil
}

func (s *FilePipelineStore) Create(pipeline *AgentPatternPipeline) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(pipeline)
}

func (s *FilePipelineStore) CreateForTenant(_ string, pipeline *AgentPatternPipeline) error {
	return s.Create(pipeline)
}

func (s *FilePipelineStore) Get(id string) (*AgentPatternPipeline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.readFile(s.pathFor(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("pipeline not found")
		}
		return nil, err
	}
	return p, nil
}

func (s *FilePipelineStore) GetByTenant(_, id string) (*AgentPatternPipeline, error) {
	return s.Get(id)
}

func (s *FilePipelineStore) GetByShortcutCode(code string) (*AgentPatternPipeline, error) {
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		if p.ShortcutCode == code {
			return p, nil
		}
	}
	return nil, fmt.Errorf("pipeline not found")
}

func (s *FilePipelineStore) Update(pipeline *AgentPatternPipeline) error {
	return s.Create(pipeline)
}

func (s *FilePipelineStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.pathFor(id))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *FilePipelineStore) DeleteForTenant(_, id string) error {
	return s.Delete(id)
}

func (s *FilePipelineStore) List() ([]*AgentPatternPipeline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]*AgentPatternPipeline, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		if strings.HasSuffix(name, ".tmp") {
			continue
		}
		p, err := s.readFile(filepath.Join(s.dir, name))
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *FilePipelineStore) ListByTenant(string) ([]*AgentPatternPipeline, error) {
	return s.List()
}

func (s *FilePipelineStore) ListEnabled() ([]*AgentPatternPipeline, error) {
	return s.List()
}

func (s *FilePipelineStore) ListEnabledByTenant(string) ([]*AgentPatternPipeline, error) {
	return s.List()
}

// Execution history is not persisted in the file store (minimal edition).

func (s *FilePipelineStore) RecordExecution(*ExecutionRecord) error { return nil }

func (s *FilePipelineStore) GetExecutionHistory(string, int) ([]*ExecutionRecord, error) {
	return nil, nil
}

func (s *FilePipelineStore) GetExecution(int64) (*ExecutionRecord, error) {
	return nil, fmt.Errorf("execution history not available in file store")
}
