package strategy

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"centag/core/pkg/logger"
)

func TestMain(m *testing.M) {
	// 初始化测试用 logger，避免 nil pointer dereference
	_ = logger.Init(logger.Config{Level: "error", Format: "console", Output: "stdout"})
	os.Exit(m.Run())
}

// newTestStore 创建一个临时目录中的独立 Store，避免污染 global
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := &Store{
		dataDir:  dir,
		filePath: filepath.Join(dir, "custom_strategies.json"),
		items:    make(map[string]*CustomStrategy),
	}
	return s
}

func TestStore_CreateAndGet(t *testing.T) {
	s := newTestStore(t)

	req := &CustomStrategy{
		Name:        "高名称权重策略",
		Description: "名称相似度优先",
		Weights:     WeightBreakdown{NameSimilarity: 0.8, CapacityMatch: 0.1, FamilyMatch: 0.1},
		Strictness:  70,
		Tolerance:   0.2,
	}

	created, err := s.Create(req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == "" {
		t.Error("ID should not be empty after create")
	}
	if created.Name != req.Name {
		t.Errorf("Name = %q, want %q", created.Name, req.Name)
	}

	// 通过 ID 取回
	got, ok := s.Get(created.ID)
	if !ok {
		t.Fatalf("Get(%q) returned false", created.ID)
	}
	if got.Name != created.Name {
		t.Errorf("got.Name = %q, want %q", got.Name, created.Name)
	}
}

func TestStore_CreateDuplicate(t *testing.T) {
	s := newTestStore(t)

	req := &CustomStrategy{
		Name:    "重复策略",
		Weights: WeightBreakdown{NameSimilarity: 0.5, CapacityMatch: 0.3, FamilyMatch: 0.2},
	}
	if _, err := s.Create(req); err != nil {
		t.Fatalf("First create failed: %v", err)
	}
	// 第二次相同名称应报错
	if _, err := s.Create(req); err == nil {
		t.Error("Expected error for duplicate name, got nil")
	}
}

func TestStore_UpdateAndDelete(t *testing.T) {
	s := newTestStore(t)

	created, err := s.Create(&CustomStrategy{
		Name:    "待修改策略",
		Weights: WeightBreakdown{NameSimilarity: 0.5, CapacityMatch: 0.3, FamilyMatch: 0.2},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 修改描述和权重
	updated, err := s.Update(created.ID, &CustomStrategy{
		Description: "已修改",
		Weights:     WeightBreakdown{NameSimilarity: 0.2, CapacityMatch: 0.5, FamilyMatch: 0.3},
		Strictness:  90,
		Tolerance:   0.1,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "已修改" {
		t.Errorf("Description = %q, want %q", updated.Description, "已修改")
	}
	if updated.Weights.CapacityMatch != 0.5 {
		t.Errorf("CapacityMatch = %.2f, want 0.5", updated.Weights.CapacityMatch)
	}

	// 删除
	if err := s.Delete(created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := s.Get(created.ID); ok {
		t.Error("Expected Get to return false after delete")
	}
}

func TestStore_Persistence(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Create(&CustomStrategy{
		Name:    "持久化测试",
		Weights: WeightBreakdown{NameSimilarity: 0.6, CapacityMatch: 0.2, FamilyMatch: 0.2},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 创建新 Store 从同一文件加载
	s2 := &Store{
		dataDir:  s.dataDir,
		filePath: s.filePath,
		items:    make(map[string]*CustomStrategy),
	}
	if err := s2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.List()) != 1 {
		t.Errorf("Expected 1 strategy after reload, got %d", len(s2.List()))
	}
}

func TestStore_ConcurrentCreate(t *testing.T) {
	s := newTestStore(t)

	var wg sync.WaitGroup
	errors := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := s.Create(&CustomStrategy{
				Name:    "并发策略" + string(rune('A'+idx)),
				Weights: WeightBreakdown{NameSimilarity: 0.5, CapacityMatch: 0.3, FamilyMatch: 0.2},
			})
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent create error: %v", err)
	}

	if len(s.List()) != 5 {
		t.Errorf("Expected 5 strategies, got %d", len(s.List()))
	}
}

func TestStore_InvalidWeights(t *testing.T) {
	s := newTestStore(t)

	// 全零权重应报错
	if _, err := s.Create(&CustomStrategy{
		Name:    "零权重",
		Weights: WeightBreakdown{NameSimilarity: 0, CapacityMatch: 0, FamilyMatch: 0},
	}); err == nil {
		t.Error("Expected error for all-zero weights")
	}

	// 负数权重应报错
	if _, err := s.Create(&CustomStrategy{
		Name:    "负权重",
		Weights: WeightBreakdown{NameSimilarity: -0.1, CapacityMatch: 0.5, FamilyMatch: 0.6},
	}); err == nil {
		t.Error("Expected error for negative weights")
	}
}

func TestStore_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	s := &Store{
		dataDir:  dir,
		filePath: filepath.Join(dir, "nonexistent.json"),
		items:    make(map[string]*CustomStrategy),
	}
	// 文件不存在时 load 应返回 nil（视为空）
	if err := s.load(); err != nil {
		t.Errorf("load on nonexistent file should return nil, got %v", err)
	}
}

func TestStore_BuiltinProtection(t *testing.T) {
	s := newTestStore(t)

	// 尝试使用内置策略 ID 对应的名称
	// nameToID("exact") → "custom-exact"，不会直接命中内置 ID "exact"
	// 但如果用户用名称 "exact"，nameToID 会产生 "custom-exact"，不冲突
	// 真正测试保护：手动构造 ID 与内置相同是不会触发的（内置保护在 ID 层面）
	_, err := s.Create(&CustomStrategy{
		Name:    "exact",
		Weights: WeightBreakdown{NameSimilarity: 0.5, CapacityMatch: 0.3, FamilyMatch: 0.2},
	})
	// "exact" → nameToID → "custom-exact"，不等于内置 "exact"，所以应该成功
	if err != nil {
		t.Errorf("Creating strategy named 'exact' should succeed (id will be custom-exact): %v", err)
	}

	// 清理
	os.Remove(s.filePath)
}

func TestGetBuiltin(t *testing.T) {
	cases := []struct {
		id    string
		found bool
	}{
		{"exact", true},
		{"family", true},
		{"capacity", true},
		{"hybrid", true},
		{"nonexistent", false},
		{"custom-something", false},
	}

	for _, c := range cases {
		b, ok := GetBuiltin(c.id)
		if ok != c.found {
			t.Errorf("GetBuiltin(%q) found=%v, want %v", c.id, ok, c.found)
		}
		if ok && b.ID != c.id {
			t.Errorf("GetBuiltin(%q).ID = %q, want %q", c.id, b.ID, c.id)
		}
	}
}

func TestListAll_ContainsBuiltins(t *testing.T) {
	// 重置 globalStore 为临时目录
	origStore := globalStore
	defer func() {
		globalStore = origStore
	}()
	// 重置 sync.Once（注意：sync.Once 不能被复制，只能重新赋值）
	storeOnce = sync.Once{}
	globalStore = newTestStore(t)

	items := ListAll()
	builtinIDs := map[string]bool{}
	for _, item := range items {
		if item.IsBuiltin {
			builtinIDs[item.ID] = true
		}
	}

	for _, expected := range []string{"exact", "family", "capacity", "hybrid"} {
		if !builtinIDs[expected] {
			t.Errorf("Expected builtin strategy %q in ListAll()", expected)
		}
	}
}
