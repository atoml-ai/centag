package database

import (
	"os"
	"path/filepath"
	"testing"
)

// TC-DB-EVO-001 附属：046 迁移脚本成对存在（sqlite/postgresql 方言齐全）。
func TestMigration046Paired(t *testing.T) {
	for _, suffix := range []string{"sqlite", "postgresql"} {
		path := filepath.Join("migrations", "046_agent_evolution_log."+suffix+".sql")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("缺失迁移脚本 %s: %v", path, err)
		}
	}
}
