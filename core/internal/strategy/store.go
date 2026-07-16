package strategy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/logger"
)

const customStrategiesFile = "custom_strategies.json"

// Store 自定义策略存储
type Store struct {
	mu       sync.RWMutex
	dataDir  string
	filePath string
	items    map[string]*CustomStrategy // id -> strategy
}

var (
	globalStore *Store
	storeOnce   sync.Once
)

// GetStore 获取全局策略存储
func GetStore() *Store {
	storeOnce.Do(func() {
		dataDir := resolveDataDir()
		globalStore = &Store{
			dataDir:  dataDir,
			filePath: filepath.Join(dataDir, customStrategiesFile),
			items:    make(map[string]*CustomStrategy),
		}
		if err := globalStore.load(); err != nil {
			logger.Warnf("[Strategy] Failed to load custom strategies: %v", err)
		}
	})
	return globalStore
}

// resolveDataDir 解析数据目录
func resolveDataDir() string {
	// 优先用配置文件所在目录
	for _, dir := range []string{"./configs", "./data", "."} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return "."
}

// load 从文件加载
func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		return nil // 文件不存在时视为空，正常
	}
	if err != nil {
		return err
	}

	var list []*CustomStrategy
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("unmarshal custom strategies: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]*CustomStrategy, len(list))
	for _, item := range list {
		s.items[item.ID] = item
	}
	logger.Infof("[Strategy] Loaded %d custom strategies from %s", len(list), s.filePath)
	return nil
}

// save 持久化到文件
func (s *Store) save() error {
	list := make([]*CustomStrategy, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal custom strategies: %w", err)
	}

	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("write custom strategies file: %w", err)
	}
	return nil
}

// nameToID 将用户名称转换为合法 ID（小写字母、数字、连字符）
func nameToID(name string) string {
	id := strings.ToLower(name)
	// 非法字符替换为连字符
	re := regexp.MustCompile(`[^a-z0-9\-_]+`)
	id = re.ReplaceAllString(id, "-")
	// 去掉首尾连字符
	id = strings.Trim(id, "-")
	if id == "" {
		id = fmt.Sprintf("custom-%d", time.Now().UnixMilli())
	}
	return "custom-" + id
}

// List 返回所有自定义策略列表
func (s *Store) List() []*CustomStrategy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*CustomStrategy, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	return list
}

// Get 按 ID 获取
func (s *Store) Get(id string) (*CustomStrategy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	return item, ok
}

// Create 创建新自定义策略
func (s *Store) Create(req *CustomStrategy) (*CustomStrategy, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("strategy name is required")
	}
	if err := validateWeights(req.Weights); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.items {
		if existing.Name == req.Name {
			return nil, fmt.Errorf("strategy with name %q already exists (id: %s)", req.Name, existing.ID)
		}
	}

	id := nameToID(req.Name)

	if _, exists := s.items[id]; exists {
		return nil, fmt.Errorf("strategy with name %q already exists (id: %s)", req.Name, id)
	}

	// 内置策略名称保护
	if _, isBuiltin := GetBuiltin(id); isBuiltin {
		return nil, fmt.Errorf("cannot use built-in strategy name: %s", req.Name)
	}

	now := time.Now()
	item := &CustomStrategy{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Weights:     req.Weights,
		Strictness:  req.Strictness,
		Tolerance:   req.Tolerance,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.items[id] = item

	if err := s.save(); err != nil {
		delete(s.items, id)
		return nil, fmt.Errorf("save failed: %w", err)
	}

	logger.Infof("[Strategy] Created custom strategy: %s (id=%s)", item.Name, item.ID)
	return item, nil
}

// Update 更新自定义策略
func (s *Store) Update(id string, req *CustomStrategy) (*CustomStrategy, error) {
	if err := validateWeights(req.Weights); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("strategy not found: %s", id)
	}

	existing.Description = req.Description
	existing.Weights = req.Weights
	existing.Strictness = req.Strictness
	existing.Tolerance = req.Tolerance
	existing.UpdatedAt = time.Now()

	if err := s.save(); err != nil {
		return nil, fmt.Errorf("save failed: %w", err)
	}

	logger.Infof("[Strategy] Updated custom strategy: %s (id=%s)", existing.Name, id)
	return existing, nil
}

// Delete 删除自定义策略
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return fmt.Errorf("strategy not found: %s", id)
	}

	delete(s.items, id)

	if err := s.save(); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	logger.Infof("[Strategy] Deleted custom strategy: %s", id)
	return nil
}

// validateWeights 验证权重配置合法性（每个维度 0-1，允许不等于 1.0 会在运算时归一化）
func validateWeights(w WeightBreakdown) error {
	if w.NameSimilarity < 0 || w.CapacityMatch < 0 || w.FamilyMatch < 0 {
		return fmt.Errorf("weights cannot be negative")
	}
	if w.NameSimilarity+w.CapacityMatch+w.FamilyMatch <= 0 {
		return fmt.Errorf("at least one weight must be greater than 0")
	}
	return nil
}

// ListAll 返回内置策略 + 自定义策略统一列表
func ListAll() []StrategyListItem {
	result := make([]StrategyListItem, 0)

	for _, b := range BuiltinStrategies {
		result = append(result, StrategyListItem{
			ID:          b.ID,
			Name:        b.Name,
			Description: b.Description,
			Weights:     b.Weights,
			IsBuiltin:   true,
		})
	}

	for _, c := range GetStore().List() {
		t := c.CreatedAt
		result = append(result, StrategyListItem{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Weights:     c.Weights,
			IsBuiltin:   false,
			Strictness:  c.Strictness,
			Tolerance:   c.Tolerance,
			CreatedAt:   &t,
		})
	}

	return result
}
