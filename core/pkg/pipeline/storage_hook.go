package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"centag/core/pkg/storage"
)

// StorageHook 处理流水线的自动存储操作。
// 在节点执行前后、流水线完成时触发存储读写。
// KV、Vector、Knowledge 三态存储对业务透明，统一通过标准接口存取。
// 通过 global_config.storage + global_config.hooks YAML 配置启用/禁用。
type StorageHook struct {
	// provider 为 nil 时存储不可用，所有操作静默跳过
	manager *storage.Manager
	// cfg 全局存储配置
	cfg StorageHookConfig
	// hookCfg 钩子行为配置（从 hooks[].config 提取）
	hookCfg HookBehaviorConfig
	// storageName 目标存储后端名称（从 hooks[].storage_name 提取，为空则用默认存储）
	storageName string
	// storageType 存储策略：kv(默认)、knowledge、vector
	storageType string
	// namespace 存储命名空间
	namespace string
	// pipelineID 流水线 ID
	pipelineID string
	// logger 日志输出
	logger Logger
	// kvStoreOverride 测试注入的 mock KVStore（nil 时通过 manager 获取）
	kvStoreOverride storage.KVStore
	// knowledgeStoreOverride 测试注入的 mock KnowledgeDataStore（nil 时通过 manager 获取）
	knowledgeStoreOverride storage.KnowledgeDataStore
	// vectorStoreOverride 测试注入的 mock VectorStore（nil 时通过 manager 获取）
	vectorStoreOverride storage.VectorStore
}

// HookBehaviorConfig 从 YAML hooks[].config 解析的钩子行为开关
type HookBehaviorConfig struct {
	SaveUserProgress        bool `json:"save_user_progress" yaml:"save_user_progress"`
	SaveConversationHistory bool `json:"save_conversation_history" yaml:"save_conversation_history"`
	SaveSceneContext        bool `json:"save_scene_context" yaml:"save_scene_context"`
	SaveCodeSnippets        bool `json:"save_code_snippets" yaml:"save_code_snippets"`
	SaveSolutions           bool `json:"save_solutions" yaml:"save_solutions"`
	TrackFileChanges        bool `json:"track_file_changes" yaml:"track_file_changes"`
}

// NewStorageHook 创建存储钩子实例。
// manager 为 nil 表示存储不可用，所有操作静默降级。
func NewStorageHook(manager *storage.Manager, pipeline *AgentPatternPipeline, logger Logger) *StorageHook {
	if pipeline == nil {
		return &StorageHook{manager: manager, logger: logger}
	}

	cfg := StorageHookConfig{}
	if pipeline.GlobalConfig.StorageConfig != nil {
		cfg = *pipeline.GlobalConfig.StorageConfig
	}

	hookCfg := HookBehaviorConfig{}
	targetStorage := ""    // 从第一个 storage 钩子提取目标存储名称
	targetStorageType := "" // 从第一个 storage 钩子提取存储类型
	for _, h := range pipeline.GlobalConfig.Hooks {
		if h.Type == "storage" {
			// 提取目标存储名称
			if h.StorageName != "" && targetStorage == "" {
				targetStorage = h.StorageName
			}
			// 提取存储类型
			if h.StorageType != "" && targetStorageType == "" {
				targetStorageType = h.StorageType
			}
			// 解析钩子配置
			if v, ok := h.Config["save_user_progress"].(bool); ok {
				hookCfg.SaveUserProgress = v
			}
			if v, ok := h.Config["save_conversation_history"].(bool); ok {
				hookCfg.SaveConversationHistory = v
			}
			if v, ok := h.Config["save_scene_context"].(bool); ok {
				hookCfg.SaveSceneContext = v
			}
			if v, ok := h.Config["save_code_snippets"].(bool); ok {
				hookCfg.SaveCodeSnippets = v
			}
			if v, ok := h.Config["save_solutions"].(bool); ok {
				hookCfg.SaveSolutions = v
			}
			if v, ok := h.Config["track_file_changes"].(bool); ok {
				hookCfg.TrackFileChanges = v
			}
			break
		}
	}

	namespace := pipeline.GlobalConfig.StorageNamespace(pipeline.ID)

	return &StorageHook{
		manager:     manager,
		cfg:         cfg,
		hookCfg:     hookCfg,
		storageName: targetStorage,
		storageType: targetStorageType,
		namespace:   namespace,
		pipelineID:  pipeline.ID,
		logger:      logger,
	}
}

// IsEnabled 检查存储钩子是否启用。
// KV/Knowledge/Vector 任一可用且配置了 enabled 即可。
func (h *StorageHook) IsEnabled() bool {
	if h == nil || !h.cfg.Enabled {
		return false
	}
	if h.kvStoreOverride != nil || h.knowledgeStoreOverride != nil || h.vectorStoreOverride != nil {
		return true
	}
	if h.manager == nil {
		return false
	}
	return true
}

// kvStore 获取 KVStore，失败返回 nil。
// 当 kvStoreOverride 不为 nil 时（测试场景），优先返回注入的 mock 实例。
// 当 storageName 不为空时，使用指定的存储后端；否则使用默认存储。
func (h *StorageHook) kvStore() storage.KVStore {
	if h.kvStoreOverride != nil {
		return h.kvStoreOverride
	}
	if h.manager == nil {
		return nil
	}

	var kv storage.KVStore
	var err error
	if h.storageName != "" {
		kv, err = h.manager.GetKVStore(h.storageName)
	} else {
		kv, err = h.manager.GetDefaultKVStore()
	}
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("storage hook: kv store unavailable, continuing without persistence",
				"error", err.Error(),
				"pipeline_id", h.pipelineID,
				"storage_name", h.storageName,
			)
		}
		return nil
	}
	return kv
}

// knowledgeStore 获取 KnowledgeDataStore，失败返回 nil。
// 当 knowledgeStoreOverride 不为 nil 时（测试场景），优先返回注入的 mock 实例。
func (h *StorageHook) knowledgeStore() storage.KnowledgeDataStore {
	if h.knowledgeStoreOverride != nil {
		return h.knowledgeStoreOverride
	}
	if h.manager == nil {
		return nil
	}

	var ks storage.KnowledgeDataStore
	var err error
	ks, err = h.manager.GetDataStore(h.storageName)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("storage hook: knowledge store unavailable, continuing without persistence",
				"error", err.Error(),
				"pipeline_id", h.pipelineID,
				"storage_name", h.storageName,
			)
		}
		return nil
	}
	return ks
}

// vectorStore 获取 VectorStore，失败返回 nil。
func (h *StorageHook) vectorStore() storage.VectorStore {
	if h.vectorStoreOverride != nil {
		return h.vectorStoreOverride
	}
	if h.manager == nil {
		return nil
	}

	var vs storage.VectorStore
	var err error
	vs, err = h.manager.GetVectorStore(h.storageName)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("storage hook: vector store unavailable, continuing without persistence",
				"error", err.Error(),
				"pipeline_id", h.pipelineID,
				"storage_name", h.storageName,
			)
		}
		return nil
	}
	return vs
}

// activeStoreType 返回当前生效的存储类型，空字符串视为 "kv"
func (h *StorageHook) activeStoreType() string {
	if h.storageType == "knowledge" || h.storageType == "vector" {
		return h.storageType
	}
	return "kv"
}

// toDataDocument 将 key + content 转换为统一的 DataDocument
func (h *StorageHook) toDataDocument(key, content string, docType string, execCtx *ExecutionContext) *storage.DataDocument {
	doc := &storage.DataDocument{
		ID:         fmt.Sprintf("%s:%s:%d", h.pipelineID, docType, time.Now().UnixNano()),
		Key:        key,
		Content:    content,
		Collection: h.namespace,
		DataType:   storage.DataType(h.activeStoreType()),
		Metadata: map[string]interface{}{
			"pipeline_id": h.pipelineID,
			"type":        docType,
		},
		CreatedAt: time.Now(),
	}
	// 从执行上下文提取 session_id 和 request_id 用于分类和检索
	if execCtx != nil {
		if sessionID, ok := execCtx.GetVariable("session_id"); ok {
			doc.Metadata["session_id"] = sessionID
		}
		if requestID, ok := execCtx.GetVariable("request_id"); ok {
			doc.Metadata["request_id"] = requestID
		}
	}
	return doc
}

// key 生成存储键
func (h *StorageHook) key(suffix string) string {
	return fmt.Sprintf("pipeline:%s:%s", h.namespace, suffix)
}

// nodeKey 生成节点存储键
func (h *StorageHook) nodeKey(nodeID, suffix string) string {
	return fmt.Sprintf("pipeline:%s:node:%s:%s", h.namespace, nodeID, suffix)
}

// ──────────── 钩子方法 ────────────

// OnNodeStart 节点执行前：从存储加载上下文
func (h *StorageHook) OnNodeStart(ctx context.Context, nodeID string, execCtx *ExecutionContext) {
	if !h.IsEnabled() {
		return
	}

	switch h.activeStoreType() {
	case "knowledge":
		h.onNodeStartKnowledge(ctx, nodeID, execCtx)
	default:
		h.onNodeStartKV(ctx, nodeID, execCtx)
	}
}

func (h *StorageHook) onNodeStartKV(ctx context.Context, nodeID string, execCtx *ExecutionContext) {
	kv := h.kvStore()
	if kv == nil {
		return
	}

	// 加载场景上下文（教育流水线）
	if h.hookCfg.SaveSceneContext {
		sceneKey := h.key("scene_context")
		if data, err := kv.GetBytes(ctx, sceneKey); err == nil && len(data) > 0 {
			var sceneCtx map[string]interface{}
			if json.Unmarshal(data, &sceneCtx) == nil {
				for k, v := range sceneCtx {
					execCtx.SetVariable(k, v)
				}
				if h.logger != nil {
					h.logger.Debug("storage hook: loaded scene context",
						"node_id", nodeID,
						"keys", len(sceneCtx),
					)
				}
			}
		}
	}

	// 加载用户进度
	if h.hookCfg.SaveUserProgress {
		userID, _ := execCtx.GetVariable("user_id")
		progressKey := h.key(fmt.Sprintf("user:%v:progress", userID))
		if data, err := kv.GetBytes(ctx, progressKey); err == nil && len(data) > 0 {
			var progress map[string]interface{}
			if json.Unmarshal(data, &progress) == nil {
				execCtx.SetVariable("user_progress", progress)
				if h.logger != nil {
					h.logger.Debug("storage hook: loaded user progress",
						"node_id", nodeID,
						"user_id", userID,
					)
				}
			}
		}
	}
}

func (h *StorageHook) onNodeStartKnowledge(ctx context.Context, nodeID string, execCtx *ExecutionContext) {
	ks := h.knowledgeStore()
	if ks == nil {
		return
	}

	// 加载场景上下文
	if h.hookCfg.SaveSceneContext {
		results, err := ks.Retrieve(ctx, "", 1, map[string]interface{}{
			"pipeline_id": h.pipelineID,
			"type":        "scene_context",
		})
		if err == nil && len(results) > 0 {
			var sceneCtx map[string]interface{}
			if json.Unmarshal([]byte(results[0].Document.Content), &sceneCtx) == nil {
				for k, v := range sceneCtx {
					execCtx.SetVariable(k, v)
				}
				if h.logger != nil {
					h.logger.Debug("storage hook: loaded scene context from knowledge store",
						"node_id", nodeID,
						"keys", len(sceneCtx),
					)
				}
			}
		}
	}

	// 加载用户进度
	if h.hookCfg.SaveUserProgress {
		userID, _ := execCtx.GetVariable("user_id")
		results, err := ks.Retrieve(ctx, "", 1, map[string]interface{}{
			"pipeline_id": h.pipelineID,
			"type":        "user_progress",
			"user_id":     fmt.Sprintf("%v", userID),
		})
		if err == nil && len(results) > 0 {
			var progress map[string]interface{}
			if json.Unmarshal([]byte(results[0].Document.Content), &progress) == nil {
				execCtx.SetVariable("user_progress", progress)
				if h.logger != nil {
					h.logger.Debug("storage hook: loaded user progress from knowledge store",
						"node_id", nodeID,
						"user_id", userID,
					)
				}
			}
		}
	}
}

// OnNodeComplete 节点执行后：保存结果到存储
func (h *StorageHook) OnNodeComplete(ctx context.Context, nodeID string, output *NodeOutput, execCtx *ExecutionContext) {
	if !h.IsEnabled() || output == nil {
		return
	}

	ttl := time.Duration(h.cfg.RetentionDays) * 24 * time.Hour
	if h.cfg.RetentionDays <= 0 {
		ttl = 0 // 永不过期
	}

	switch h.activeStoreType() {
	case "knowledge":
		h.onNodeCompleteKnowledge(ctx, nodeID, output, execCtx)
	default:
		h.onNodeCompleteKV(ctx, nodeID, output, execCtx, ttl)
	}
}

func (h *StorageHook) onNodeCompleteKV(ctx context.Context, nodeID string, output *NodeOutput, execCtx *ExecutionContext, ttl time.Duration) {
	kv := h.kvStore()
	if kv == nil {
		return
	}

	// 保存节点输出
	if output.Content != "" {
		outputKey := h.nodeKey(nodeID, "output")
		if err := kv.Set(ctx, outputKey, output.Content, ttl); err != nil && h.logger != nil {
			h.logger.Warn("storage hook: failed to save node output",
				"node_id", nodeID,
				"error", err.Error(),
			)
		}
	}

	// 保存对话历史
	if h.hookCfg.SaveConversationHistory {
		h.saveConversationHistory(ctx, kv, nodeID, output, execCtx, ttl)
	}

	// 保存代码片段（编程流水线）
	if h.hookCfg.SaveCodeSnippets && strings.Contains(output.Content, "```") {
		h.saveCodeSnippets(ctx, kv, output, execCtx, ttl)
	}

	// 保存解决方案（编程流水线）
	if h.hookCfg.SaveSolutions {
		h.saveSolution(ctx, kv, nodeID, output, execCtx, ttl)
	}
}

func (h *StorageHook) onNodeCompleteKnowledge(ctx context.Context, nodeID string, output *NodeOutput, execCtx *ExecutionContext) {
	ks := h.knowledgeStore()
	if ks == nil {
		return
	}

	// 保存节点输出
	if output.Content != "" {
		outputKey := h.nodeKey(nodeID, "output")
		doc := h.toDataDocument(outputKey, output.Content, "node_output", execCtx)
		doc.Metadata["node_id"] = nodeID
		if err := ks.Store(ctx, doc); err != nil && h.logger != nil {
			h.logger.Warn("storage hook: failed to save node output to knowledge store",
				"node_id", nodeID,
				"error", err.Error(),
			)
		}
	}

	// 保存对话历史
	if h.hookCfg.SaveConversationHistory {
		h.saveConversationHistoryToKnowledge(ctx, ks, nodeID, output, execCtx)
	}

	// 保存代码片段
	if h.hookCfg.SaveCodeSnippets && strings.Contains(output.Content, "```") {
		h.saveCodeSnippetsToKnowledge(ctx, ks, output, execCtx)
	}

	// 保存解决方案
	if h.hookCfg.SaveSolutions {
		h.saveSolutionToKnowledge(ctx, ks, nodeID, output, execCtx)
	}
}

// OnPipelineComplete 流水线完成后：归档执行记录
func (h *StorageHook) OnPipelineComplete(ctx context.Context, execCtx *ExecutionContext) {
	if !h.IsEnabled() {
		return
	}

	ttl := time.Duration(h.cfg.RetentionDays) * 24 * time.Hour
	if h.cfg.RetentionDays <= 0 {
		ttl = 0
	}

	switch h.activeStoreType() {
	case "knowledge":
		h.onPipelineCompleteKnowledge(ctx, execCtx)
	default:
		h.onPipelineCompleteKV(ctx, execCtx, ttl)
	}
}

// ──────────── 内部辅助方法 ────────────

// saveConversationHistory 保存对话历史到列表存储
func (h *StorageHook) saveConversationHistory(ctx context.Context, kv storage.KVStore, nodeID string, output *NodeOutput, execCtx *ExecutionContext, ttl time.Duration) {
	historyKey := h.key("history")

	// 读取现有历史
	var history []ConversationTurn
	if data, err := kv.GetBytes(ctx, historyKey); err == nil && len(data) > 0 {
		json.Unmarshal(data, &history)
	}

	// 获取输入
	inputContent := ""
	if v, ok := execCtx.GetVariable("input"); ok {
		inputContent = fmt.Sprintf("%v", v)
	}

	// 从执行上下文提取 session_id 和 request_id
	sessionID, _ := execCtx.GetVariable("session_id")
	requestID, _ := execCtx.GetVariable("request_id")

	// 追加新记录
	turn := ConversationTurn{
		Input:     inputContent,
		Output:    output.Content,
		NodeID:    nodeID,
		SessionID: fmt.Sprintf("%v", sessionID),
		RequestID: fmt.Sprintf("%v", requestID),
		Timestamp: time.Now(),
	}
	history = append(history, turn)

	// 限制历史记录数量
	if len(history) > 100 {
		history = history[len(history)-100:]
	}

	if err := kv.Set(ctx, historyKey, history, ttl); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save conversation history",
			"error", err.Error(),
		)
	}
}

// saveCodeSnippets 提取并保存代码片段
func (h *StorageHook) saveCodeSnippets(ctx context.Context, kv storage.KVStore, output *NodeOutput, execCtx *ExecutionContext, ttl time.Duration) {
	snippets := extractCodeSnippets(output.Content)
	if len(snippets) == 0 {
		return
	}

	snippetsKey := h.key("code_snippets")

	var existing []CodeSnippet
	if data, err := kv.GetBytes(ctx, snippetsKey); err == nil && len(data) > 0 {
		json.Unmarshal(data, &existing)
	}

	// 从执行上下文提取 session_id 和 request_id
	sessionID, _ := execCtx.GetVariable("session_id")
	requestID, _ := execCtx.GetVariable("request_id")

	for _, s := range snippets {
		existing = append(existing, CodeSnippet{
			ID:          fmt.Sprintf("snippet_%d", time.Now().UnixNano()),
			Language:    s.Language,
			Code:        s.Code,
			Description: s.Description,
			SessionID:   fmt.Sprintf("%v", sessionID),
			RequestID:   fmt.Sprintf("%v", requestID),
			CreatedAt:   time.Now(),
		})
	}

	if len(existing) > 200 {
		existing = existing[len(existing)-200:]
	}

	if err := kv.Set(ctx, snippetsKey, existing, ttl); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save code snippets",
			"error", err.Error(),
		)
	}
}

// saveSolution 保存解决方案记录
func (h *StorageHook) saveSolution(ctx context.Context, kv storage.KVStore, nodeID string, output *NodeOutput, execCtx *ExecutionContext, ttl time.Duration) {
	inputContent := ""
	if v, ok := execCtx.GetVariable("input"); ok {
		inputContent = fmt.Sprintf("%v", v)
	}

	// 从执行上下文提取 session_id 和 request_id
	sessionID, _ := execCtx.GetVariable("session_id")
	requestID, _ := execCtx.GetVariable("request_id")

	solution := CodingSolution{
		TaskDescription: inputContent,
		Solution:        output.Content,
		Success:         true,
		SessionID:       fmt.Sprintf("%v", sessionID),
		RequestID:       fmt.Sprintf("%v", requestID),
		Timestamp:       time.Now(),
	}

	solutionsKey := h.key("solutions")

	var existing []CodingSolution
	if data, err := kv.GetBytes(ctx, solutionsKey); err == nil && len(data) > 0 {
		json.Unmarshal(data, &existing)
	}

	_ = nodeID // reserve for future use
	existing = append(existing, solution)
	if len(existing) > 100 {
		existing = existing[len(existing)-100:]
	}

	if err := kv.Set(ctx, solutionsKey, existing, ttl); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save solution",
			"error", err.Error(),
		)
	}
}

// saveUserProgress 保存用户学习进度
func (h *StorageHook) saveUserProgress(ctx context.Context, kv storage.KVStore, execCtx *ExecutionContext, ttl time.Duration) {
	userID, _ := execCtx.GetVariable("user_id")
	if userID == nil {
		return
	}

	// 从执行上下文提取 session_id 和 request_id
	sessionID, _ := execCtx.GetVariable("session_id")
	requestID, _ := execCtx.GetVariable("request_id")

	// 构建进度数据
	progress := map[string]interface{}{
		"last_updated": time.Now().Format(time.RFC3339),
		"pipeline_id":  h.pipelineID,
		"node_count":   len(execCtx.GetAllResults()),
		"session_id":   sessionID,
		"request_id":   requestID,
	}

	// 合并已有进度
	if existing, ok := execCtx.GetVariable("user_progress"); ok {
		if p, ok := existing.(map[string]interface{}); ok {
			for k, v := range p {
				progress[k] = v
			}
		}
	}

	progressKey := h.key(fmt.Sprintf("user:%v:progress", userID))
	if err := kv.Set(ctx, progressKey, progress, ttl); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save user progress",
			"user_id", userID,
			"error", err.Error(),
		)
	}
}

// saveSceneContext 保存教育流水线场景上下文
func (h *StorageHook) saveSceneContext(ctx context.Context, kv storage.KVStore, execCtx *ExecutionContext, ttl time.Duration) {
	sceneData := make(map[string]interface{})

	// 提取场景信息
	if scene, ok := execCtx.GetVariable("scene"); ok {
		sceneData["scene"] = scene
	}
	if scene, ok := execCtx.GetVariable("selected_scene"); ok {
		sceneData["selected_scene"] = scene
	}

	// 从执行上下文提取 session_id 和 request_id
	sessionID, _ := execCtx.GetVariable("session_id")
	requestID, _ := execCtx.GetVariable("request_id")
	sceneData["session_id"] = sessionID
	sceneData["request_id"] = requestID

	allResults := execCtx.GetAllResults()
	sceneData["node_results"] = len(allResults)

	lastOutput := execCtx.GetLastOutput()
	if lastOutput != nil {
		sceneData["last_output_length"] = len(lastOutput.Content)
	}

	sceneData["last_updated"] = time.Now().Format(time.RFC3339)

	sceneKey := h.key("scene_context")
	if err := kv.Set(ctx, sceneKey, sceneData, ttl); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save scene context",
			"error", err.Error(),
		)
	}
}

// ──────────── KV 路径: OnPipelineComplete ────────────

func (h *StorageHook) onPipelineCompleteKV(ctx context.Context, execCtx *ExecutionContext, ttl time.Duration) {
	kv := h.kvStore()
	if kv == nil {
		return
	}

	execLog := execCtx.GetExecutionLog()

	logKey := h.key(fmt.Sprintf("execution:%d", time.Now().UnixNano()))
	if err := kv.Set(ctx, logKey, execLog, ttl); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save execution log",
			"error", err.Error(),
		)
	}

	if h.hookCfg.SaveUserProgress {
		h.saveUserProgress(ctx, kv, execCtx, ttl)
	}

	if h.hookCfg.SaveSceneContext {
		h.saveSceneContext(ctx, kv, execCtx, ttl)
	}

	if h.logger != nil {
		h.logger.Debug("storage hook: pipeline complete archived",
			"pipeline_id", h.pipelineID,
			"duration_ms", execLog.Duration,
			"total_tokens", execLog.TotalTokens,
		)
	}
}

// ──────────── Knowledge 路径: OnPipelineComplete ────────────

func (h *StorageHook) onPipelineCompleteKnowledge(ctx context.Context, execCtx *ExecutionContext) {
	ks := h.knowledgeStore()
	if ks == nil {
		return
	}

	execLog := execCtx.GetExecutionLog()
	logData, err := json.Marshal(execLog)
	if err == nil {
		logKey := h.key(fmt.Sprintf("execution:%d", time.Now().UnixNano()))
		doc := h.toDataDocument(logKey, string(logData), "execution_log", execCtx)
		if err := ks.Store(ctx, doc); err != nil && h.logger != nil {
			h.logger.Warn("storage hook: failed to save execution log to knowledge store",
				"error", err.Error(),
			)
		}
	}

	if h.hookCfg.SaveUserProgress {
		h.saveUserProgressToKnowledge(ctx, ks, execCtx)
	}

	if h.hookCfg.SaveSceneContext {
		h.saveSceneContextToKnowledge(ctx, ks, execCtx)
	}

	if h.logger != nil {
		h.logger.Debug("storage hook: pipeline complete archived to knowledge store",
			"pipeline_id", h.pipelineID,
		)
	}
}

// ──────────── Knowledge 写入 Helper ────────────

func (h *StorageHook) saveConversationHistoryToKnowledge(ctx context.Context, ks storage.KnowledgeDataStore, nodeID string, output *NodeOutput, execCtx *ExecutionContext) {
	inputContent := ""
	if v, ok := execCtx.GetVariable("input"); ok {
		inputContent = fmt.Sprintf("%v", v)
	}

	turn := ConversationTurn{
		Input:     inputContent,
		Output:    output.Content,
		NodeID:    nodeID,
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(turn)
	if err != nil {
		return
	}

	historyKey := h.key("history")
	doc := h.toDataDocument(historyKey, string(jsonData), "conversation_turn", execCtx)
	doc.Metadata["node_id"] = nodeID
	if err := ks.Store(ctx, doc); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save conversation turn to knowledge store",
			"error", err.Error(),
		)
	}
}

func (h *StorageHook) saveCodeSnippetsToKnowledge(ctx context.Context, ks storage.KnowledgeDataStore, output *NodeOutput, execCtx *ExecutionContext) {
	snippets := extractCodeSnippets(output.Content)
	for _, s := range snippets {
		snippetData, err := json.Marshal(s)
		if err != nil {
			continue
		}
		snippetKey := h.key(fmt.Sprintf("snippet:%d", time.Now().UnixNano()))
		doc := h.toDataDocument(snippetKey, string(snippetData), "code_snippet", execCtx)
		doc.Metadata["language"] = s.Language
		if err := ks.Store(ctx, doc); err != nil && h.logger != nil {
			h.logger.Warn("storage hook: failed to save code snippet to knowledge store",
				"error", err.Error(),
			)
		}
	}
}

func (h *StorageHook) saveSolutionToKnowledge(ctx context.Context, ks storage.KnowledgeDataStore, nodeID string, output *NodeOutput, execCtx *ExecutionContext) {
	inputContent := ""
	if v, ok := execCtx.GetVariable("input"); ok {
		inputContent = fmt.Sprintf("%v", v)
	}

	solution := CodingSolution{
		TaskDescription: inputContent,
		Solution:        output.Content,
		Success:         true,
		Timestamp:       time.Now(),
	}
	jsonData, err := json.Marshal(solution)
	if err != nil {
		return
	}

	solutionsKey := h.key(fmt.Sprintf("solution:%d", time.Now().UnixNano()))
	doc := h.toDataDocument(solutionsKey, string(jsonData), "solution", execCtx)
	doc.Metadata["node_id"] = nodeID
	if err := ks.Store(ctx, doc); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save solution to knowledge store",
			"error", err.Error(),
		)
	}
}

func (h *StorageHook) saveUserProgressToKnowledge(ctx context.Context, ks storage.KnowledgeDataStore, execCtx *ExecutionContext) {
	userID, _ := execCtx.GetVariable("user_id")
	if userID == nil {
		return
	}

	progress := map[string]interface{}{
		"last_updated": time.Now().Format(time.RFC3339),
		"pipeline_id":  h.pipelineID,
		"node_count":   len(execCtx.GetAllResults()),
	}

	if existing, ok := execCtx.GetVariable("user_progress"); ok {
		if p, ok := existing.(map[string]interface{}); ok {
			for k, v := range p {
				progress[k] = v
			}
		}
	}

	jsonData, err := json.Marshal(progress)
	if err != nil {
		return
	}

	progressKey := h.key(fmt.Sprintf("user:%v:progress", userID))
	doc := h.toDataDocument(progressKey, string(jsonData), "user_progress", execCtx)
	doc.Metadata["user_id"] = fmt.Sprintf("%v", userID)
	if err := ks.Store(ctx, doc); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save user progress to knowledge store",
			"user_id", userID,
			"error", err.Error(),
		)
	}
}

func (h *StorageHook) saveSceneContextToKnowledge(ctx context.Context, ks storage.KnowledgeDataStore, execCtx *ExecutionContext) {
	sceneData := make(map[string]interface{})

	if scene, ok := execCtx.GetVariable("scene"); ok {
		sceneData["scene"] = scene
	}
	if scene, ok := execCtx.GetVariable("selected_scene"); ok {
		sceneData["selected_scene"] = scene
	}

	allResults := execCtx.GetAllResults()
	sceneData["node_results"] = len(allResults)

	lastOutput := execCtx.GetLastOutput()
	if lastOutput != nil {
		sceneData["last_output_length"] = len(lastOutput.Content)
	}

	sceneData["last_updated"] = time.Now().Format(time.RFC3339)
	jsonData, err := json.Marshal(sceneData)
	if err != nil {
		return
	}

	sceneKey := h.key("scene_context")
	doc := h.toDataDocument(sceneKey, string(jsonData), "scene_context", execCtx)
	if err := ks.Store(ctx, doc); err != nil && h.logger != nil {
		h.logger.Warn("storage hook: failed to save scene context to knowledge store",
			"error", err.Error(),
		)
	}
}

// extractCodeSnippets 从输出内容中提取代码块
func extractCodeSnippets(content string) []codeSnippet {
	var snippets []codeSnippet
	lines := strings.Split(content, "\n")
	inCode := false
	var lang string
	var buf strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") && !inCode {
			inCode = true
			lang = strings.TrimPrefix(trimmed, "```")
			lang = strings.TrimSpace(lang)
			buf.Reset()
			continue
		}
		if strings.HasPrefix(trimmed, "```") && inCode {
			// 关闭标记只能是 ``` 后跟可选空白，带语言标识符的不是关闭标记
			afterBackticks := strings.TrimPrefix(trimmed, "```")
			if strings.TrimSpace(afterBackticks) != "" {
				// 非空白内容 → 是内容行，不是关闭标记
				buf.WriteString(line)
				buf.WriteString("\n")
				continue
			}
			inCode = false
			code := strings.TrimSpace(buf.String())
			if code != "" {
				snippets = append(snippets, codeSnippet{
					Language:    strings.TrimSpace(lang),
					Code:        code,
					Description: "",
				})
			}
			continue
		}
		if inCode {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	return snippets
}

// codeSnippet 内部代码片段结构
type codeSnippet struct {
	Language    string `json:"language"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

// ConversationTurn 对话轮次记录
type ConversationTurn struct {
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	NodeID    string    `json:"node_id"`
	SessionID string    `json:"session_id,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
