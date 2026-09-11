package evolution

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// IsDryrunHeader 标记形式归一化（1/true/yes，大小写不敏感；其余不识别）。
func TestIsDryrunHeaderNormalization(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "TRUE", "Yes"} {
		if !IsDryrunHeader(v) {
			t.Errorf("%q 应识别为 dryrun", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no", "on"} {
		if IsDryrunHeader(v) {
			t.Errorf("%q 不应被视为 dryrun", v)
		}
	}
}

// AppendDryrunUsage/Read jsonl 分桶行落盘（evol-tag 分桶可观测）。
func TestAppendDryrunUsageBucket(t *testing.T) {
	dir := t.TempDir()
	SetDataDirForce(dir)
	defer SetDataDirForce("")
	if err := AppendDryrunUsage(DryrunUsageRecord{
		UserID:      7,
		Model:       "gpt-4o",
		Backend:     "azure",
		TotalTokens: 120,
		Success:     true,
		RequestID:   "req-1",
	}); err != nil {
		t.Fatalf("AppendDryrunUsage: %v", err)
	}
	// 全局 dataDir 未注入时静默跳过（不报错）。
	SetDataDirForce("")
	if err := AppendDryrunUsage(DryrunUsageRecord{UserID: 1}); err != nil {
		t.Fatalf("无 dataDir 应跳过而非报错: %v", err)
	}
	SetDataDirForce(dir)
	defer SetDataDirForce("")
	if err := AppendDryrunUsage(DryrunUsageRecord{
		UserID:      8,
		Model:       "gpt-4o",
		Backend:     "azure",
		TotalTokens: 90,
		Success:     true,
		RequestID:   "req-2",
	}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(jsonPath(dir, "dryrun-usage.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	wantIDs := []int64{7, 8}
	i := 0
	for sc.Scan() {
		var rec DryrunUsageRecord
		if err := json.Unmarshal([]byte(sc.Text()), &rec); err != nil {
			t.Fatalf("jsonl 行解析: %v", err)
		}
		if rec.Bucket != TagDryrun {
			t.Fatalf("bucket = %q", rec.Bucket)
		}
		if rec.Time == "" {
			t.Fatal("时间戳缺失")
		}
		if i < len(wantIDs) && rec.UserID != wantIDs[i] {
			t.Fatalf("行 %d user = %d", i, rec.UserID)
		}
		i++
	}
	if i != 2 {
		t.Fatalf("行数 = %d", i)
	}
	if !strings.Contains(HeaderEvolutionDryrun, "Dryrun") {
		t.Fatal("契约头名异常")
	}
}
