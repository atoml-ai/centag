package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPermissionPathIsolation(t *testing.T) {
	dataDir := t.TempDir()

	// dataDir 内文件（通过）
	inner := filepath.Join(dataDir, "config.yaml")
	if err := os.WriteFile(inner, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := secureResolve(dataDir, "config.yaml")
	if err != nil {
		t.Fatalf("inner path rejected: %v", err)
	}
	// macOS 下 /var 是 /private/var 的符号链接，EvalSymlinks 会展开；比较时同样展开
	dataDirReal, _ := filepath.EvalSymlinks(dataDir)
	if !strings.HasPrefix(got, dataDirReal) {
		t.Errorf("inner resolve = %q, want under %q", got, dataDirReal)
	}

	// dataDir 外绝对路径（拒绝）
	outer := filepath.Join(filepath.Dir(dataDir), "outside.txt")
	if err := os.WriteFile(outer, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := secureResolve(dataDir, outer); err == nil {
		t.Error("absolute path outside dataDir should be rejected")
	}

	// ../ 相对路径逃逸（拒绝）
	if _, err := secureResolve(dataDir, "../outside.txt"); err == nil {
		t.Error("`../` path should be rejected")
	}
	if _, err := secureResolve(dataDir, "sub/../../outside.txt"); err == nil {
		t.Error("nested `../` path should be rejected")
	}

	// 符号链接指向外部（拒绝）
	link := filepath.Join(dataDir, "evil-link")
	if err := os.Symlink(outer, link); err == nil {
		if _, err := secureResolve(dataDir, "evil-link"); err == nil {
			t.Error("symlink escaping dataDir should be rejected")
		}
	} else if !strings.Contains(err.Error(), "operation not supported") {
		t.Fatalf("symlink create: %v", err)
	}

	// 空 path 拒绝
	if _, err := secureResolve(dataDir, ""); err == nil {
		t.Error("empty path should be rejected")
	}
}

func TestSecureResolve_RejectsSubdirBoundary(t *testing.T) {
	dataDir := t.TempDir()
	// 前缀相似但越界的路径（sibling 目录），确保前缀校验不误放行
	outerDir := filepath.Dir(dataDir)
	sibling := filepath.Join(outerDir, "sibling.txt")
	if err := os.WriteFile(sibling, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 拼一个看起来在 dataDir 前缀下的路径，实际指向 sibling
	// e.g. dataDir=/tmp/x/data, path=../sibling.txt → 应拒绝
	if _, err := secureResolve(dataDir, "../sibling.txt"); err == nil {
		t.Error("`../sibling.txt` should be rejected")
	}
}

func TestSecureResolve_EmptyDataDir(t *testing.T) {
	if _, err := secureResolve("", "foo"); err == nil {
		t.Error("empty dataDir with relative path should be rejected")
	}
}
