package agent

import (
	"sync"
	"testing"
)

// TestRuntimeEngineConcurrentEnsureBackendNoRace 验证 EnsureBackend/RefreshToken
// 在并发调用下不再产生数据竞争（P0-2 回归，需 -race 运行）。
func TestRuntimeEngineConcurrentEnsureBackendNoRace(t *testing.T) {
	e := NewRuntimeEngine(&AgentConfig{}, t.TempDir(), nil)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				opts := AgentEngineOptions{
					BaseURL:    "http://127.0.0.1:1",
					Token:      string(rune('a'+i%26)) + "-jwt",
					BackendID:  "backend",
					PipelineID: "p",
					SessionID:  "s",
				}
				if j%7 == 0 {
					opts.BackendID = "backend-2" // 触发重建分支
				}
				e.EnsureBackend(opts)
				e.RefreshToken(opts.Token)
				_ = e.BackendSnapshot()
			}
		}(i)
	}
	wg.Wait()
}

// TestBackendSnapshotTracksLastApplied 验证快照记录最近一次生效的选项。
func TestBackendSnapshotTracksLastApplied(t *testing.T) {
	e := NewRuntimeEngine(&AgentConfig{}, t.TempDir(), nil)
	e.EnsureBackend(AgentEngineOptions{BaseURL: "http://a", Token: "t1", BackendID: "b1"})
	snap := e.BackendSnapshot()
	if snap.Token != "t1" || snap.BackendID != "b1" {
		t.Fatalf("snapshot mismatch: %+v", snap)
	}
	// 同 backend 更新 token：走更新分支
	e.EnsureBackend(AgentEngineOptions{BaseURL: "http://a", Token: "t2", BackendID: "b1"})
	if snap = e.BackendSnapshot(); snap.Token != "t2" {
		t.Fatalf("token update not tracked: %+v", snap)
	}
	// 空 token 不覆盖已记录值（与 RefreshToken 的空串保护一致）
	e.EnsureBackend(AgentEngineOptions{BaseURL: "", Token: "", BackendID: "b1"})
	if snap = e.BackendSnapshot(); snap.Token != "t2" {
		t.Fatalf("empty opts must not clobber snapshot: %+v", snap)
	}
}
