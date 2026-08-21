package pipeline

import (
	"testing"

	"centag/core/internal/cache"
)

// 验证 cache 节点经插件路径创建后仍返回原始 *CacheNode，
// 使 engine 的 facade 注入（类型断言）生效。
func TestCreateFromConfigCacheReturnsRawNode(t *testing.T) {
	registry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(registry); err != nil {
		t.Fatalf("RegisterBuiltinNodes failed: %v", err)
	}

	config := PipelineNodeConfig{
		ID:            "cache_read",
		Type:          NodeTypeCache,
		Kind:          "cache.access",
		Implementation: "builtin.cache",
		Name:          "Cache Read",
	}
	nodeConfig := NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":         "read",
			"strategy":          "exact",
			"read_storage_name": "postgresql",
		},
	}

	node, err := registry.CreateFromConfig(config, nodeConfig)
	if err != nil {
		t.Fatalf("CreateFromConfig failed: %v", err)
	}

	cacheNode, ok := node.(*CacheNode)
	if !ok {
		t.Fatalf("expected *CacheNode, got %T — facade injection would be skipped", node)
	}

	// 模拟 engine 注入
	mgr, err := cache.NewManager(&cache.CacheConfig{Enabled: true})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	facade := cache.NewFacade(mgr)
	cacheNode.SetCacheFacade(facade)
	if m := facade.Manager(); m != nil {
		cacheNode.SetCacheManager(m)
	}

	if cacheNode.cacheFacade == nil {
		t.Fatal("cacheFacade should be set after injection")
	}
	if cacheNode.CacheManager == nil {
		t.Fatal("CacheManager should be set after injection")
	}
}
