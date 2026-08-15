package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestStore skill manifest 持久化抽象（T7）。
// personal：文件目录（data/agent-skills/）；team：system_config + DB（DBManifestStore）。
type ManifestStore interface {
	// List 列出 store 内的自定义 manifest 文件名（不含扩展名）。
	List() ([]string, error)
	// Load 读取指定 manifest 内容。
	Load(name string) ([]byte, error)
	// Save 保存 manifest（覆盖同名）。
	Save(name string, data []byte) error
	// Delete 删除 manifest。
	Delete(name string) error
}

// FileManifestStore 文件版 skill manifest 存储（personal / minimal 发行版）。
// 目录：<dataDir>/agent-skills/
type FileManifestStore struct {
	dir string
}

// NewFileManifestStore 创建文件版存储。目录不存在时首次写入会创建。
func NewFileManifestStore(dir string) *FileManifestStore {
	return &FileManifestStore{dir: dir}
}

// Dir 返回存储目录。
func (s *FileManifestStore) Dir() string { return s.dir }

// manifestFileName 规范 manifest 文件名：agent-skill-<name>.yaml。
func manifestFileName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "agent-skill-") {
		return name + ".yaml"
	}
	return "agent-skill-" + name + ".yaml"
}

// List 列出目录下 agent-skill-*.yaml（不含扩展名，保持稳定排序）。
func (s *FileManifestStore) List() ([]string, error) {
	if st, err := os.Stat(s.dir); err != nil || !st.IsDir() {
		return nil, nil
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("list skill manifests: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "agent-skill-") || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		names = append(names, strings.TrimSuffix(strings.TrimPrefix(name, "agent-skill-"), ".yaml"))
	}
	sort.Strings(names)
	return names, nil
}

// Load 读取 manifest。
func (s *FileManifestStore) Load(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.dir, manifestFileName(name)))
}

// Save 保存 manifest。
func (s *FileManifestStore) Save(name string, data []byte) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create skill manifest dir: %w", err)
	}
	return os.WriteFile(filepath.Join(s.dir, manifestFileName(name)), data, 0o644)
}

// Delete 删除 manifest。文件不存在时返回错误。
func (s *FileManifestStore) Delete(name string) error {
	path := filepath.Join(s.dir, manifestFileName(name))
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("delete skill manifest %s: %w", name, err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete skill manifest %s: %w", name, err)
	}
	return nil
}

// LoadToRegistry 把 store 中的自定义 manifest 载入注册表（custom=true）。
func (s *FileManifestStore) LoadToRegistry(r *SkillPluginRegistry) error {
	names, err := s.List()
	if err != nil {
		return err
	}
	var sources []ManifestSource
	for _, n := range names {
		data, err := s.Load(n)
		if err != nil {
			return fmt.Errorf("load skill manifest %s: %w", n, err)
		}
		sources = append(sources, ManifestSource{
			ManifestData: map[string][]byte{n + ".yaml": data},
			Custom:       true,
		})
	}
	if len(sources) == 0 {
		return nil
	}
	return r.LoadFromSources(sources)
}
