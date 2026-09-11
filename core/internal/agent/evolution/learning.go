package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LearningArchive learning 归档：var/agent/learning/（宿主路径，centag_info 暴露路径）。
type LearningArchive struct {
	dir string
}

// NewLearningArchive 构造归档器。
func NewLearningArchive(dataDir string) *LearningArchive {
	return &LearningArchive{dir: filepath.Join(dataDir, "var", "agent", "learning")}
}

// Record 写入一条 learning 归档，返回相对 dataDir 的归档路径。
// 内容为：提案 + 效果度量 + 归档时间。
func (l *LearningArchive) Record(proposalID string, measure, content any) (string, error) {
	doc := map[string]any{
		"proposal_id": proposalID,
		"measure":     measure,
		"content":     content,
		"archived_at": now(),
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("learning 序列化: %w", err)
	}
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return "", fmt.Errorf("创建 learning 目录: %w", err)
	}
	name := fmt.Sprintf("learning-%s-%d.json", proposalID, timeNow().Unix())
	full := filepath.Join(l.dir, name)
	if err := os.WriteFile(full, b, 0o600); err != nil {
		return "", fmt.Errorf("写 learning 归档: %w", err)
	}
	return "var/agent/learning/" + name, nil
}

// Dir 返回归档目录（相对 dataDir）。
func (l *LearningArchive) Dir() string { return "var/agent/learning" }

// Read 读取归档文件（测试/审计用）：rel 为 Record 返回的归档相对路径。
func (l *LearningArchive) Read(rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(l.dir, filepath.Base(filepath.FromSlash(rel))))
}
