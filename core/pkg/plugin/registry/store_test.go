package registry

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMemoryStore_Register(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plugin := &PluginMetadata{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "Test plugin",
		Author:      "test-author",
		Category:    "test",
		Tags:        []string{"test", "example"},
		DownloadURL: "https://example.com/plugin.zip",
		Checksum:    "abc123",
	}

	err := store.Register(ctx, plugin)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if plugin.ID == "" {
		t.Error("Plugin ID should be generated")
	}

	if plugin.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestMemoryStore_Get(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plugin := &PluginMetadata{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "Test plugin",
		DownloadURL: "https://example.com/plugin.zip",
		Checksum:    "abc123",
	}

	store.Register(ctx, plugin)

	// 获取插件
	retrieved, err := store.Get(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Name != plugin.Name {
		t.Errorf("Name mismatch: got %s, want %s", retrieved.Name, plugin.Name)
	}

	// 获取不存在的插件
	_, err = store.Get(ctx, "non-existent")
	if err == nil {
		t.Error("Should return error for non-existent plugin")
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 注册多个插件
	plugins := []*PluginMetadata{
		{Name: "plugin-a", Version: "1.0.0", Category: "cat1", Author: "author1", DownloadURL: "url1", Checksum: "sum1"},
		{Name: "plugin-b", Version: "1.0.0", Category: "cat2", Author: "author2", DownloadURL: "url2", Checksum: "sum2"},
		{Name: "plugin-c", Version: "1.0.0", Category: "cat1", Author: "author1", DownloadURL: "url3", Checksum: "sum3"},
	}

	for _, p := range plugins {
		store.Register(ctx, p)
	}

	// 测试列出所有插件
	resp, err := store.List(ctx, &ListPluginsRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if resp.Total != 3 {
		t.Errorf("Expected 3 plugins, got %d", resp.Total)
	}

	// 测试按分类过滤
	resp, err = store.List(ctx, &ListPluginsRequest{Category: "cat1", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List with category filter failed: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Expected 2 plugins in cat1, got %d", resp.Total)
	}

	// 测试按作者过滤
	resp, err = store.List(ctx, &ListPluginsRequest{Author: "author1", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List with author filter failed: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Expected 2 plugins by author1, got %d", resp.Total)
	}
}

func TestMemoryStore_Search(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plugins := []*PluginMetadata{
		{Name: "awesome-plugin", Version: "1.0.0", Description: "An awesome plugin", DownloadURL: "url1", Checksum: "sum1"},
		{Name: "test-plugin", Version: "1.0.0", Description: "A test plugin", DownloadURL: "url2", Checksum: "sum2"},
	}

	for _, p := range plugins {
		store.Register(ctx, p)
	}

	// 按名称搜索
	resp, err := store.List(ctx, &ListPluginsRequest{Search: "awesome", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 result for 'awesome', got %d", resp.Total)
	}

	// 按描述搜索
	resp, err = store.List(ctx, &ListPluginsRequest{Search: "test", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 result for 'test', got %d", resp.Total)
	}
}

func TestMemoryStore_Rate(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plugin := &PluginMetadata{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "Test plugin",
		DownloadURL: "https://example.com/plugin.zip",
		Checksum:    "abc123",
	}

	store.Register(ctx, plugin)

	// 评分
	err := store.Rate(ctx, plugin.ID, "user1", 5, "Great plugin!")
	if err != nil {
		t.Fatalf("Rate failed: %v", err)
	}

	// 再次评分（同一用户，应更新）
	err = store.Rate(ctx, plugin.ID, "user1", 4, "Good plugin")
	if err != nil {
		t.Fatalf("Rate update failed: %v", err)
	}

	// 另一个用户评分
	err = store.Rate(ctx, plugin.ID, "user2", 3, "Okay")
	if err != nil {
		t.Fatalf("Rate failed: %v", err)
	}

	// 获取评分
	rating, count, err := store.GetRating(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("GetRating failed: %v", err)
	}

	// 内存存储不检查重复用户，所以平均分是 (5 + 4 + 3) / 3 = 4.0
	if rating != 4.0 {
		t.Errorf("Expected rating 4.0, got %f", rating)
	}

	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}

func TestMemoryStore_DownloadCount(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plugin := &PluginMetadata{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "Test plugin",
		DownloadURL: "https://example.com/plugin.zip",
		Checksum:    "abc123",
	}

	store.Register(ctx, plugin)

	// 增加下载计数
	for i := 0; i < 5; i++ {
		err := store.IncrementDownloadCount(ctx, plugin.ID)
		if err != nil {
			t.Fatalf("IncrementDownloadCount failed: %v", err)
		}
	}

	// 验证
	retrieved, err := store.Get(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.DownloadCount != 5 {
		t.Errorf("Expected download count 5, got %d", retrieved.DownloadCount)
	}
}

func TestMemoryStore_ListVersions(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 注册同一插件的多个版本
	versions := []string{"1.0.0", "1.1.0", "2.0.0"}
	for _, v := range versions {
		plugin := &PluginMetadata{
			Name:        "test-plugin",
			Version:     v,
			Description: "Test plugin",
			DownloadURL: "https://example.com/plugin.zip",
			Checksum:    "abc123",
		}
		store.Register(ctx, plugin)
	}

	// 获取第一个版本的 ID
	plugin1, _ := store.Get(ctx, "test-plugin@1.0.0")

	// 列出版本
	versionList, err := store.ListVersions(ctx, plugin1.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	if len(versionList) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(versionList))
	}

	// 获取特定版本
	v2, err := store.GetVersion(ctx, plugin1.ID, "2.0.0")
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if v2.Version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", v2.Version)
	}
}

func TestMemoryStore_Sort(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 创建有时间差的插件
	plugins := []*PluginMetadata{
		{Name: "plugin-a", Version: "1.0.0", DownloadCount: 100, Rating: 4.5, DownloadURL: "url1", Checksum: "sum1"},
		{Name: "plugin-b", Version: "1.0.0", DownloadCount: 50, Rating: 3.5, DownloadURL: "url2", Checksum: "sum2"},
		{Name: "plugin-c", Version: "1.0.0", DownloadCount: 200, Rating: 5.0, DownloadURL: "url3", Checksum: "sum3"},
	}

	for _, p := range plugins {
		store.Register(ctx, p)
		time.Sleep(10 * time.Millisecond) // 确保时间不同
	}

	// 按下载量排序
	resp, err := store.List(ctx, &ListPluginsRequest{SortBy: "download_count", SortOrder: "desc", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List with sort failed: %v", err)
	}

	if len(resp.Plugins) < 3 {
		t.Fatalf("Expected 3 plugins, got %d", len(resp.Plugins))
	}

	if resp.Plugins[0].Name != "plugin-c" {
		t.Errorf("Expected first plugin to be plugin-c (highest downloads), got %s", resp.Plugins[0].Name)
	}

	// 按评分排序
	resp, err = store.List(ctx, &ListPluginsRequest{SortBy: "rating", SortOrder: "desc", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List with rating sort failed: %v", err)
	}

	if resp.Plugins[0].Name != "plugin-c" {
		t.Errorf("Expected first plugin to be plugin-c (highest rating), got %s", resp.Plugins[0].Name)
	}

	// 按名称排序
	resp, err = store.List(ctx, &ListPluginsRequest{SortBy: "name", SortOrder: "asc", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List with name sort failed: %v", err)
	}

	if resp.Plugins[0].Name != "plugin-a" {
		t.Errorf("Expected first plugin to be plugin-a (alphabetical), got %s", resp.Plugins[0].Name)
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plugin := &PluginMetadata{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "Original description",
		DownloadURL: "https://example.com/plugin.zip",
		Checksum:    "abc123",
	}

	err := store.Register(ctx, plugin)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	originalUpdatedAt := plugin.UpdatedAt

	// 等待一小段时间确保时间不同
	time.Sleep(10 * time.Millisecond)

	// 更新插件
	plugin.Description = "Updated description"
	err = store.Update(ctx, plugin)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 验证更新时间已更新
	if !plugin.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after Update")
	}

	// 验证描述已更新
	retrieved, err := store.Get(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Description != "Updated description" {
		t.Errorf("Expected updated description, got %s", retrieved.Description)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plugin := &PluginMetadata{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "Test plugin",
		DownloadURL: "https://example.com/plugin.zip",
		Checksum:    "abc123",
	}

	err := store.Register(ctx, plugin)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 删除插件
	err = store.Delete(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证已删除
	_, err = store.Get(ctx, plugin.ID)
	if err == nil {
		t.Error("Expected error after delete")
	}

	// 删除不存在的插件
	err = store.Delete(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error when deleting non-existent plugin")
	}
}

func TestMemoryStore_DeleteWithVersions(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 注册同一插件的多个版本
	versions := []string{"1.0.0", "1.1.0", "2.0.0"}
	for _, v := range versions {
		p := &PluginMetadata{
			Name:        "test-plugin",
			Version:     v,
			Description: "Test plugin",
			DownloadURL: "https://example.com/plugin.zip",
			Checksum:    "abc123",
		}
		store.Register(ctx, p)
	}

	// 获取第一个版本
	p1, _ := store.Get(ctx, "test-plugin@1.0.0")

	// 删除前获取版本列表
	versionsBefore, _ := store.ListVersions(ctx, p1.ID)
	if len(versionsBefore) != 3 {
		t.Errorf("Expected 3 versions before delete, got %d", len(versionsBefore))
	}

	// 删除一个版本
	err := store.Delete(ctx, p1.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证该版本已删除
	_, err = store.Get(ctx, p1.ID)
	if err == nil {
		t.Error("Expected error after delete")
	}

	// 验证其他版本仍在 - 使用任意一个剩余版本的 ID 查询
	p2, _ := store.Get(ctx, "test-plugin@1.1.0")
	versions2, err := store.ListVersions(ctx, p2.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	// 应该还有2个版本
	if len(versions2) != 2 {
		t.Errorf("Expected 2 versions after delete, got %d", len(versions2))
	}
}

func TestMemoryStore_TagFilter(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plugins := []*PluginMetadata{
		{Name: "plugin-a", Version: "1.0.0", Tags: []string{"ai", "nlp"}, DownloadURL: "url1", Checksum: "sum1"},
		{Name: "plugin-b", Version: "1.0.0", Tags: []string{"ai", "vision"}, DownloadURL: "url2", Checksum: "sum2"},
		{Name: "plugin-c", Version: "1.0.0", Tags: []string{"database"}, DownloadURL: "url3", Checksum: "sum3"},
	}

	for _, p := range plugins {
		store.Register(ctx, p)
	}

	// 按标签过滤 - ai
	resp, err := store.List(ctx, &ListPluginsRequest{
		Tags:     []string{"ai"},
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("List with tag filter failed: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Expected 2 plugins with tag 'ai', got %d", resp.Total)
	}

	// 按标签过滤 - nlp
	resp, err = store.List(ctx, &ListPluginsRequest{
		Tags:     []string{"nlp"},
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("List with tag filter failed: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 plugin with tag 'nlp', got %d", resp.Total)
	}
}

func TestMemoryStore_Pagination(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 注册多个插件
	for i := 0; i < 25; i++ {
		p := &PluginMetadata{
			Name:        fmt.Sprintf("plugin-%02d", i),
			Version:     "1.0.0",
			Description: "Test plugin",
			DownloadURL: fmt.Sprintf("https://example.com/plugin-%02d.zip", i),
			Checksum:    fmt.Sprintf("sum%d", i),
		}
		store.Register(ctx, p)
	}

	// 测试第一页
	resp, err := store.List(ctx, &ListPluginsRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List page 1 failed: %v", err)
	}

	if resp.Total != 25 {
		t.Errorf("Expected total 25, got %d", resp.Total)
	}

	if len(resp.Plugins) != 10 {
		t.Errorf("Expected 10 plugins on page 1, got %d", len(resp.Plugins))
	}

	if resp.TotalPages != 3 {
		t.Errorf("Expected 3 total pages, got %d", resp.TotalPages)
	}

	// 测试最后一页
	resp, err = store.List(ctx, &ListPluginsRequest{Page: 3, PageSize: 10})
	if err != nil {
		t.Fatalf("List page 3 failed: %v", err)
	}

	if len(resp.Plugins) != 5 {
		t.Errorf("Expected 5 plugins on page 3, got %d", len(resp.Plugins))
	}

	// 测试超出范围
	resp, err = store.List(ctx, &ListPluginsRequest{Page: 10, PageSize: 10})
	if err != nil {
		t.Fatalf("List page 10 failed: %v", err)
	}

	if len(resp.Plugins) != 0 {
		t.Errorf("Expected 0 plugins on page 10, got %d", len(resp.Plugins))
	}
}

func TestMemoryStore_PageSizeLimit(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 注册几个插件
	for i := 0; i < 5; i++ {
		p := &PluginMetadata{
			Name:        fmt.Sprintf("plugin-%d", i),
			Version:     "1.0.0",
			DownloadURL: "url",
			Checksum:    "sum",
		}
		store.Register(ctx, p)
	}

	// 测试超过最大页面大小的限制
	resp, err := store.List(ctx, &ListPluginsRequest{Page: 1, PageSize: 200})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if resp.PageSize != 100 {
		t.Errorf("Expected PageSize to be limited to 100, got %d", resp.PageSize)
	}
}

func TestMemoryStore_UpdateRating(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plugin := &PluginMetadata{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "Test plugin",
		DownloadURL: "https://example.com/plugin.zip",
		Checksum:    "abc123",
	}

	store.Register(ctx, plugin)

	// 添加评分
	store.Rate(ctx, plugin.ID, "user1", 5, "Great!")
	store.Rate(ctx, plugin.ID, "user2", 3, "Okay")

	// 验证评分
	rating, count, err := store.GetRating(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("GetRating failed: %v", err)
	}

	expectedRating := (5.0 + 3.0) / 2.0
	if rating != expectedRating {
		t.Errorf("Expected rating %f, got %f", expectedRating, rating)
	}

	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}
