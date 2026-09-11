package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TargetKeyState 目标当前生效参数（evolution 运行域，落 var/agent/evolution/targets.json）。
// 语义：apply 后的新参数即宿主行为面的下一轮生效值；rollback 恢复快照。
type TargetKeyState map[string]any

// TargetStateStore 目标状态存储（宿主文件域 var/agent/evolution/）。
type TargetStateStore struct {
	path string
}

// NewTargetStateStore dataDir/var/agent/evolution/targets.json。
func NewTargetStateStore(dataDir string) *TargetStateStore {
	return &TargetStateStore{path: filepath.Join(dataDir, "var", "agent", "evolution", "targets.json")}
}

// jsonPath evolution 域内文件路径 dataDir/var/agent/evolution/<name>。
func jsonPath(dataDir, name string) string {
	return filepath.Join(dataDir, "var", "agent", "evolution", name)
}

// writeFileAtomic 原子落盘（tmp + rename）。
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建目录: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("写文件: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("替换文件: %w", err)
	}
	return nil
}

// Load 全量读取（缺文件返回空 map）。
func (t *TargetStateStore) Load() (map[string]TargetKeyState, error) {
	out := make(map[string]TargetKeyState)
	b, err := os.ReadFile(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("读目标状态: %w", err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("解析目标状态: %w", err)
	}
	return out, nil
}

// Save 全量写回（原子替换）。
func (t *TargetStateStore) Save(all map[string]TargetKeyState) error {
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化目标状态: %w", err)
	}
	return writeFileAtomic(t.path, b)
}

// Get 单目标当前参数（缺失返回空 map）。
func (t *TargetStateStore) Get(target string) (TargetKeyState, error) {
	all, err := t.Load()
	if err != nil {
		return nil, err
	}
	if all[target] != nil {
		return all[target], nil
	}
	return TargetKeyState{}, nil
}

// Set 单目标写回。
func (t *TargetStateStore) Set(target string, state TargetKeyState) error {
	all, err := t.Load()
	if err != nil {
		return err
	}
	all[target] = state
	return t.Save(all)
}
