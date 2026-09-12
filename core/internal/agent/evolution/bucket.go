package evolution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// HeaderEvolutionDryrun dryrun 标记请求头（T4：显式标记 + evol-tag 分桶，
// 不污染常规 usage 聚合）。
//
// 注意：本头当前仅由 proxy 记账路径单点消费（token_usage_record.go）；
// 未来新增任何 token/计费记账路径，必须先检查本头并分流 dryrun 用量，
// 否则 dryrun 请求会漏入常规 usage 聚合（破坏 TC-BILL-EVO-002 口径）。
const HeaderEvolutionDryrun = "X-Evolution-Dryrun"

// TagDryrun evol-tag 分桶标记值。
const TagDryrun = "evol-dryrun"

var dataDirMu sync.Mutex
var globalDataDir string

// SetDataDir 全局 bootstrap（server 启动注入；幂等）。
func SetDataDir(dir string) {
	dataDirMu.Lock()
	defer dataDirMu.Unlock()
	if globalDataDir == "" {
		globalDataDir = dir
	}
}

// SetDataDirForce 测试专用：强制覆盖全局 dataDir。
func SetDataDirForce(dir string) {
	dataDirMu.Lock()
	defer dataDirMu.Unlock()
	globalDataDir = dir
}

// IsDryrunHeader 判定标记头是否声明 dryrun（1/true/yes，大小写不敏感）。
func IsDryrunHeader(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// DryrunUsageRecord dryrun 请求用量（evol-tag 分桶行）。
type DryrunUsageRecord struct {
	Time             string `json:"time"`
	UserID           int64  `json:"user_id"`
	APIKeyID         int64  `json:"api_key_id"`
	SessionID        string `json:"session_id"`
	Model            string `json:"model"`
	Backend          string `json:"backend"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Success          bool   `json:"success"`
	DeptTag          string `json:"dept_tag"`
	Source           string `json:"source"`
	RequestID        string `json:"request_id"`
	Bucket           string `json:"bucket"`
}

// AppendDryrunUsage 落 var/agent/evolution/dryrun-usage.jsonl（evol-tag 分桶）。
// 追加语义（jsonl，尾行可裁剪），不落 token_usage/token_usage_daily —— 常规
// 计费聚合零变化（TC-BILL-EVO-002）。
func AppendDryrunUsage(rec DryrunUsageRecord) error {
	dataDirMu.Lock()
	dir := globalDataDir
	dataDirMu.Unlock()
	if dir == "" {
		return nil
	}
	rec.Bucket = TagDryrun
	if rec.Time == "" {
		rec.Time = now()
	}
	path := jsonPath(dir, "dryrun-usage.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}
