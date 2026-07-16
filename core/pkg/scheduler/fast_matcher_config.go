package scheduler

import (
	"encoding/json"
	"os"
	"sync"

	"centag/core/pkg/logger"
)

// FastMatcherConfig 快速匹配器配置
type FastMatcherConfig struct {
	// CategoryToTask category 到任务类型的映射
	CategoryToTask map[string]TaskType `json:"category_to_task"`

	// TaskToBackends 任务类型到后端推荐列表的映射
	TaskToBackends map[TaskType][]BackendRecommendationConfig `json:"task_to_backends"`
}

// BackendRecommendationConfig 后端推荐配置（用于序列化）
type BackendRecommendationConfig struct {
	BackendID string   `json:"backend_id"`
	Model     string   `json:"model"`
	Priority  int      `json:"priority"`
	Reason    string   `json:"reason"`
	Fallbacks []string `json:"fallbacks"`
}

// DefaultFastMatcherConfig 返回默认配置
func DefaultFastMatcherConfig() *FastMatcherConfig {
	return &FastMatcherConfig{
		CategoryToTask: map[string]TaskType{
			// 代码生成
			"code":       TaskCodeGeneration,
			"代码":        TaskCodeGeneration,
			"python":     TaskCodeGeneration,
			"py":         TaskCodeGeneration,
			"java":       TaskCodeGeneration,
			"javascript": TaskCodeGeneration,
			"js":         TaskCodeGeneration,
			"typescript": TaskCodeGeneration,
			"ts":         TaskCodeGeneration,
			"go":         TaskCodeGeneration,
			"golang":     TaskCodeGeneration,
			"rust":       TaskCodeGeneration,
			"cpp":        TaskCodeGeneration,
			"c++":        TaskCodeGeneration,
			"php":        TaskCodeGeneration,
			"ruby":       TaskCodeGeneration,
			"swift":      TaskCodeGeneration,
			"kotlin":     TaskCodeGeneration,
			"sql":        TaskCodeGeneration,
			"shell":      TaskCodeGeneration,
			"bash":       TaskCodeGeneration,
			"程序":        TaskCodeGeneration,
			"函数":        TaskCodeGeneration,
			"方法":        TaskCodeGeneration,
			"脚本":        TaskCodeGeneration,
			"算法":        TaskCodeGeneration,
			"类":          TaskCodeGeneration,
			"接口":        TaskCodeGeneration,
			"模块":        TaskCodeGeneration,
			"库":          TaskCodeGeneration,
			"leetcode":   TaskCodeGeneration,
			"实现":        TaskCodeGeneration,
			"编写":        TaskCodeGeneration,

			// 翻译
			"translate":   TaskTranslation,
			"translation": TaskTranslation,
			"翻译":         TaskTranslation,

			// 摘要
			"summary":  TaskLongText,
			"摘要":      TaskLongText,
			"总结":      TaskLongText,

			// 创意写作
			"creative": TaskCreative,
			"story":    TaskCreative,
			"poem":     TaskCreative,
			"创意":      TaskCreative,
			"写作":      TaskCreative,
			"小说":      TaskCreative,
			"诗歌":      TaskCreative,
			"故事":      TaskCreative,

			// 分析推理
			"analysis": TaskAnalysis,
			"reason":   TaskComplexReasoning,
			"math":     TaskComplexReasoning,
			"推理":      TaskComplexReasoning,
			"分析":      TaskAnalysis,
			"数学":      TaskComplexReasoning,
			"逻辑":      TaskComplexReasoning,
			"数据":      TaskAnalysis,
			"图表":      TaskAnalysis,

			// 对话（默认）
			"chat": TaskSimpleChat,
		},
		TaskToBackends: map[TaskType][]BackendRecommendationConfig{
			TaskCodeGeneration: {
				{BackendID: "bigmodel", Model: "GLM-4-flash", Priority: 1, Reason: "代码生成能力强，GLM-4-flash 免费"},
				{BackendID: "deepseek", Model: "deepseek-coder", Priority: 2, Reason: "DeepSeek Coder 专业代码模型"},
				{BackendID: "ppio", Model: "deepseek-coder", Priority: 3, Reason: "PPIO 提供 DeepSeek Coder"},
				{BackendID: "ollama-local", Model: "codellama", Priority: 4, Reason: "本地代码模型（降级方案）"},
			},
			TaskSimpleChat: {
				{BackendID: "ollama-local", Model: "llama3", Priority: 1, Reason: "本地模型，零成本，隐私保护"},
				{BackendID: "bigmodel", Model: "GLM-4-flash", Priority: 2, Reason: "智谱 GLM-4-flash 免费"},
				{BackendID: "deepseek", Model: "deepseek-chat", Priority: 3, Reason: "DeepSeek Chat 性价比高"},
			},
			TaskComplexReasoning: {
				{BackendID: "deepseek", Model: "deepseek-reasoner", Priority: 1, Reason: "DeepSeek Reasoner 推理能力强"},
				{BackendID: "bigmodel", Model: "GLM-4", Priority: 2, Reason: "智谱 GLM-4 综合能力强"},
				{BackendID: "ppio", Model: "deepseek-reasoner", Priority: 3, Reason: "PPIO 提供 DeepSeek Reasoner"},
			},
			TaskLongText: {
				{BackendID: "bigmodel", Model: "GLM-4-32K", Priority: 1, Reason: "智谱 GLM-4-32K 支持长文本"},
				{BackendID: "deepseek", Model: "deepseek-chat", Priority: 2, Reason: "DeepSeek Chat 支持长上下文"},
				{BackendID: "ppio", Model: "kimi", Priority: 3, Reason: "Kimi 擅长长文档处理"},
			},
			TaskTranslation: {
				{BackendID: "bigmodel", Model: "GLM-4-flash", Priority: 1, Reason: "智谱 GLM-4-flash 免费，翻译质量好"},
				{BackendID: "deepseek", Model: "deepseek-chat", Priority: 2, Reason: "DeepSeek Chat 翻译能力强"},
			},
			TaskCreative: {
				{BackendID: "bigmodel", Model: "GLM-4", Priority: 1, Reason: "智谱 GLM-4 创意写作能力强"},
				{BackendID: "deepseek", Model: "deepseek-chat", Priority: 2, Reason: "DeepSeek Chat 创意能力好"},
			},
			TaskAnalysis: {
				{BackendID: "bigmodel", Model: "GLM-4", Priority: 1, Reason: "智谱 GLM-4 数据分析能力强"},
				{BackendID: "deepseek", Model: "deepseek-reasoner", Priority: 2, Reason: "DeepSeek Reasoner 推理分析能力强"},
			},
			TaskEmbedding: {
				{BackendID: "ollama-local", Model: "nomic-embed-text", Priority: 1, Reason: "本地嵌入模型，零成本"},
			},
		},
	}
}

// FastMatcherConfigManager 快速匹配器配置管理器
type FastMatcherConfigManager struct {
	mu       sync.RWMutex
	config   *FastMatcherConfig
	filePath string
}

// NewFastMatcherConfigManager 创建配置管理器
func NewFastMatcherConfigManager(filePath string) *FastMatcherConfigManager {
	return &FastMatcherConfigManager{
		config:   DefaultFastMatcherConfig(),
		filePath: filePath,
	}
}

// Load 从文件加载配置
func (m *FastMatcherConfigManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Infof("快速匹配器配置文件不存在，使用默认配置: %s", m.filePath)
			return nil
		}
		return err
	}

	var config FastMatcherConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	m.config = &config
	logger.Infof("成功加载快速匹配器配置: %s", m.filePath)
	return nil
}

// Save 保存配置到文件
func (m *FastMatcherConfigManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.filePath, data, 0644)
}

// GetConfig 获取配置
func (m *FastMatcherConfigManager) GetConfig() *FastMatcherConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *FastMatcherConfigManager) UpdateConfig(config *FastMatcherConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// AddCategoryMapping 添加 category 到任务类型的映射
func (m *FastMatcherConfigManager) AddCategoryMapping(category string, taskType TaskType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.CategoryToTask[category] = taskType
}

// RemoveCategoryMapping 移除 category 映射
func (m *FastMatcherConfigManager) RemoveCategoryMapping(category string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.config.CategoryToTask, category)
}

// AddBackendRecommendation 添加后端推荐
func (m *FastMatcherConfigManager) AddBackendRecommendation(taskType TaskType, rec BackendRecommendationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.TaskToBackends[taskType] = append(m.config.TaskToBackends[taskType], rec)
}

// RemoveBackendRecommendation 移除后端推荐
func (m *FastMatcherConfigManager) RemoveBackendRecommendation(taskType TaskType, backendID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	recs := m.config.TaskToBackends[taskType]
	for i, rec := range recs {
		if rec.BackendID == backendID {
			m.config.TaskToBackends[taskType] = append(recs[:i], recs[i+1:]...)
			return
		}
	}
}
