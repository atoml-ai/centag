package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ErrPathOutsideDataDir 表示路径越界到数据目录之外（任务9 / R03 路径隔离）。
var ErrPathOutsideDataDir = fmt.Errorf("path is outside the allowed data directory")

// secureResolve 解析用户提供的 path 为 dataDir 内的绝对路径，并校验不越界（任务9/R03）。
//
// 校验覆盖四类逃逸：
//   - dataDir 内路径（通过）
//   - 绝对路径指向 dataDir 外（拒绝）
//   - `../` 相对路径逃逸（拒绝）
//   - 符号链接指向 dataDir 外（拒绝）
//
// 返回的路径保证位于 dataDir 之下。dataDir 为空时仅做形态校验（不含根校验）。
func secureResolve(dataDir, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is empty")
	}
	if dataDir == "" {
		return "", fmt.Errorf("data dir is empty")
	}
	// 目标路径：绝对路径原样，相对路径拼到 dataDir
	candidate := p
	if !filepath.IsAbs(p) {
		candidate = filepath.Join(dataDir, p)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	abs = filepath.Clean(abs)

	rootAbs, err := filepath.Abs(dataDir)
	if err != nil {
		return "", fmt.Errorf("resolve data dir: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)

	// 符号链接解析：成功时以解析后的真实路径为准（防 symlink 逃逸）
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		real = filepath.Clean(real)
		rootReal := rootAbs
		if rr, err := filepath.EvalSymlinks(rootAbs); err == nil {
			rootReal = rr
		}
		rootReal = filepath.Clean(rootReal)
		if !isWithin(rootReal, real) {
			return "", fmt.Errorf("%w: %q", ErrPathOutsideDataDir, p)
		}
		return real, nil
	}

	// 目标不存在（无法解析 symlink）：基于未解析路径做前缀校验
	if !isWithin(rootAbs, abs) {
		return "", fmt.Errorf("%w: %q", ErrPathOutsideDataDir, p)
	}
	return abs, nil
}

// isWithin 判断 path 是否在 root 之内（含 root 自身）。
func isWithin(root, path string) bool {
	if root == "" {
		return false
	}
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
