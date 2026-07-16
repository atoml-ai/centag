package pipeline

import (
	"context"
	"fmt"

	"centag/core/pkg/backend"
	"centag/core/pkg/storage"
)

// 注意：backend 包仅用于 SetBackendManager 的类型声明，未来可进一步移除。

// ExecutorResult 执行器结果（适配 scheduler 包）
type ExecutorResult struct {
	BackendID string
	Model     string
	Content   string
	Tokens    int
	LatencyMs int64
}

// PipelineScheduler 流水线调度器 - 集成到现有调度系统
type PipelineScheduler struct {
	engine         *PipelineEngine
	registry       *PipelineRegistry
	configLoader   *YAMLConfigLoader
	logger         Logger
	storageManager *storage.Manager

	// 模式映射: 旧模式key -> 流水线ID
	modeMapping map[string]string
	// 后端管理器
	backendMgr *backend.Manager
}

// NewPipelineScheduler 创建流水线调度器
func NewPipelineScheduler(
	nodeRegistry *NodeRegistry,
	pipelineRegistry *PipelineRegistry,
	logger Logger,
	storageManager *storage.Manager,
) *PipelineScheduler {
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, logger, storageManager)

	return &PipelineScheduler{
		engine:         engine,
		registry:       pipelineRegistry,
		configLoader:   NewYAMLConfigLoader(pipelineRegistry),
		logger:         logger,
		storageManager: storageManager,
		modeMapping:    make(map[string]string),
	}
}

// SetBackendManager 设置后端管理器（保留用于未来扩展）
func (ps *PipelineScheduler) SetBackendManager(mgr *backend.Manager) {
	ps.backendMgr = mgr
}

// RegisterModeMapping 注册旧模式到流水线的映射
func (ps *PipelineScheduler) RegisterModeMapping(modeKey, pipelineID string) {
	ps.modeMapping[modeKey] = pipelineID
}

// GetPipelineForMode 获取模式对应的流水线
func (ps *PipelineScheduler) GetPipelineForMode(modeKey string) (*AgentPatternPipeline, bool) {
	pipelineID, exists := ps.modeMapping[modeKey]
	if !exists {
		return nil, false
	}

	pipeline := ps.registry.Get(pipelineID)
	if pipeline == nil {
		return nil, false
	}

	return pipeline, true
}

// ExecuteMode 执行指定模式
func (ps *PipelineScheduler) ExecuteMode(ctx context.Context, modeKey string, input *PipelineInput) (*PipelineOutput, error) {
	pipeline, exists := ps.GetPipelineForMode(modeKey)
	if !exists {
		return nil, fmt.Errorf("no pipeline mapping for mode: %s", modeKey)
	}

	return ps.engine.Execute(ctx, pipeline.ID, input)
}

// LoadPipelinesFromDirectory 从目录加载流水线配置
func (ps *PipelineScheduler) LoadPipelinesFromDirectory(dirPath string) error {
	pipelines, err := ps.configLoader.LoadFromDirectory(dirPath)
	if err != nil {
		return fmt.Errorf("failed to load pipelines: %w", err)
	}

	ps.logger.Info("loaded pipelines from directory",
		"dir", dirPath,
		"count", len(pipelines))

	return nil
}

// LoadPipelineFromFile 从文件加载单个流水线
func (ps *PipelineScheduler) LoadPipelineFromFile(filePath string) (*AgentPatternPipeline, error) {
	pipeline, err := ps.configLoader.LoadFromFile(filePath)
	if err != nil {
		return nil, err
	}

	ps.logger.Info("loaded pipeline from file",
		"file", filePath,
		"id", pipeline.ID)

	return pipeline, nil
}

// ListRegisteredPipelines 列出所有注册的流水线
func (ps *PipelineScheduler) ListRegisteredPipelines() []*AgentPatternPipeline {
	return ps.registry.List()
}

// GetPipeline 获取指定流水线
func (ps *PipelineScheduler) GetPipeline(id string) (*AgentPatternPipeline, bool) {
	pipeline := ps.registry.Get(id)
	if pipeline == nil {
		return nil, false
	}
	return pipeline, true
}

// UnregisterPipeline 注销流水线
func (ps *PipelineScheduler) UnregisterPipeline(id string) {
	ps.registry.Remove(id)
	// 清理模式映射
	for mode, pid := range ps.modeMapping {
		if pid == id {
			delete(ps.modeMapping, mode)
		}
	}
}

// Engine 返回流水线执行引擎（供 scheduler 包使用）
func (ps *PipelineScheduler) Engine() *PipelineEngine {
	return ps.engine
}

// Registry 返回流水线注册表（供 scheduler 包使用）
func (ps *PipelineScheduler) Registry() *PipelineRegistry {
	return ps.registry
}


