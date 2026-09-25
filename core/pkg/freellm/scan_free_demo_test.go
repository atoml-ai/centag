//go:build freellm_demo

package freellm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/logger"
)

// TestScanFreeKeyless 演示“自动获取免费大模型”能力：
//  1. 用一个临时文件 store 真实落盘注册全部 keyless 后端；
//  2. 重新加载验证持久化；
//  3. best-effort 探测连通性并拉取真实模型列表（需外网，超时可容忍）。
func TestScanFreeKeyless(t *testing.T) {
	dir, err := os.MkdirTemp("", "freellm-scan")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	storePath := filepath.Join(dir, "backends.json")

	if err := logger.Init(logger.Config{Level: "info", Format: "console", Output: "stdout"}); err != nil {
		t.Fatalf("logger init: %v", err)
	}

	mgr := backend.NewManager()
	mgr.SetStore(backend.NewFileBackendStore(storePath))
	if err := mgr.Load(); err != nil {
		t.Fatalf("load store: %v", err)
	}

	ids, err := ScanKeyless(mgr, "")
	if err != nil {
		t.Fatalf("ScanKeyless: %v", err)
	}

	fmt.Printf("\n=== [1] ScanKeyless 注册了 %d 个零配置(keyless)免费后端 ===\n", len(ids))
	for _, id := range ids {
		c, _ := mgr.Get(id)
		fmt.Printf("  • %-16s %-22s %s\n     seed_models=%d probe=%s\n",
			c.ID, c.Name, c.BaseURL, len(c.SupportedModels), c.ProbeModel)
	}

	// 重新加载，验证确实落盘
	mgr2 := backend.NewManager()
	mgr2.SetStore(backend.NewFileBackendStore(storePath))
	if err := mgr2.Load(); err != nil {
		t.Fatalf("reload store: %v", err)
	}
	fmt.Printf("\n=== [2] 重新加载 store：持久化了 %d 个后端 ===\n", len(mgr2.List()))

	// best-effort 连通性探测 + 真实模型发现（需外网，超时可容忍）
	fmt.Printf("\n=== [3] 连通性探测 + 模型发现（best-effort，超时容忍）===\n")
	for _, id := range ids {
		if c, e := mgr.Get(id); e == nil {
			c.Timeout = 15 // 演示用，缩短超时
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	results, perr := mgr.ProbeAllBackends(ctx, true)
	if perr != nil {
		fmt.Printf("  探测返回错误（已容忍）: %v\n", perr)
	}
	reachable := 0
	for _, r := range results {
		status := "✗ 不可达"
		if r.Success {
			status = "✓ 可达"
			reachable++
		}
		fmt.Printf("  %s %-16s models=%d rt=%dms %s\n", status, r.BackendID, r.ModelsCount, r.ResponseTime, r.Error)
	}
	fmt.Printf("\n=== 小结：keyless 后端 %d/%d 个在当前环境可达 ===\n", reachable, len(ids))
}
