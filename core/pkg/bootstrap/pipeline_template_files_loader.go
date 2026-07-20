package bootstrap

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"centag/core/pkg/logger"

	"gopkg.in/yaml.v3"
)

// editionDirMap 定义每个版本需要加载的子目录。
// common 目录始终包含在所有版本中。
var editionDirMap = map[string][]string{
	"minimal": {"common"},
	"personal": {"common", "personal"},
	"team":    {"common", "personal"},
}

// LoadInitialPipelineTemplatesFromFiles 从全局和 Profile 的 config/initdata/pipeline-templates
// 目录加载 YAML 模板描述文件。Profile 模板覆盖全局同名模板。
// 仅支持 .yaml/.yml 格式。
func LoadInitialPipelineTemplatesFromFiles() []InitialPipelineTemplate {
	return LoadInitialPipelineTemplatesWithEdition("")
}

// LoadInitialPipelineTemplatesWithEdition 根据版本加载流水线模板。
// 目录结构：
//
//	pipeline-templates/
//	  common/    — 所有版本都加载
//	  personal/  — personal/team 版加载
// edition 为空时加载所有子目录（向后兼容）。
func LoadInitialPipelineTemplatesWithEdition(edition string) []InitialPipelineTemplate {
	globalRoot, profileRoot := InitdataRoots()
	if globalRoot == "" && profileRoot == "" {
		logger.Info("bootstrap: INITDATA_PATH / ProjectRoot 未确定，跳过流水线模板加载")
		return nil
	}

	// 确定要扫描的子目录
	dirsToLoad := resolveEditionSubdirs(edition)

	// 1. Load global templates (base)
	tmplMap := make(map[string]InitialPipelineTemplate)
	if globalRoot != "" {
		globalDir := filepath.Join(globalRoot, "pipeline-templates")
		loaded := loadEditionTemplates(globalDir, dirsToLoad, tmplMap)
		if loaded {
			logger.Infof("bootstrap: 从全局配置加载流水线模板: %s (%d 个)", globalDir, len(tmplMap))
		}
	}

	// 2. Load profile templates (overlay)
	if profileRoot != "" && profileRoot != globalRoot {
		profileDir := filepath.Join(profileRoot, "pipeline-templates")
		loaded := loadEditionTemplates(profileDir, dirsToLoad, tmplMap)
		if loaded {
			logger.Infof("bootstrap: 从 Profile 配置加载流水线模板: %s (%d 个覆盖)", profileDir, len(tmplMap))
		}
	}

	if len(tmplMap) == 0 {
		logger.Info("bootstrap: 未找到流水线模板（全局或 Profile）")
		return nil
	}

	// Convert map to sorted slice by ID
	ids := make([]string, 0, len(tmplMap))
	for id := range tmplMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	templates := make([]InitialPipelineTemplate, 0, len(tmplMap))
	for _, id := range ids {
		templates = append(templates, tmplMap[id])
	}

	logger.Infof("bootstrap: 合并后流水线模板共 %d 个 (edition=%q, dirs=%v)", len(templates), edition, dirsToLoad)
	return templates
}

// resolveEditionSubdirs 根据版本返回需要加载的子目录列表。
func resolveEditionSubdirs(edition string) []string {
	if edition == "" {
		// 向后兼容：加载所有子目录
		return []string{"common", "personal", "minimal"}
	}
	if dirs, ok := editionDirMap[edition]; ok {
		return dirs
	}
	// 未知版本：只加载 common
	logger.Warnf("bootstrap: 未知版本 %q，仅加载 common 目录", edition)
	return []string{"common"}
}

// loadEditionTemplates 从 baseDir 下的指定子目录加载模板。
func loadEditionTemplates(baseDir string, subdirs []string, tmplMap map[string]InitialPipelineTemplate) bool {
	anyLoaded := false
	for _, sub := range subdirs {
		dir := filepath.Join(baseDir, sub)
		if loadYAMLFilesFromDir(dir, tmplMap) {
			anyLoaded = true
		}
	}
	return anyLoaded
}

// loadYAMLFilesFromDir 从目录加载所有 YAML 文件到 tmplMap。
// 同 ID 模板会被覆盖（profile 覆盖 global）。
func loadYAMLFilesFromDir(dirPath string, tmplMap map[string]InitialPipelineTemplate) bool {
	if st, err := os.Stat(dirPath); err != nil || !st.IsDir() {
		return false
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		logger.Warnf("bootstrap: 读取流水线模板目录失败: %v", err)
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
			continue
		}

		fullPath := filepath.Join(dirPath, name)
		data, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			logger.Warnf("bootstrap: 读取流水线模板文件失败 %s: %v", fullPath, readErr)
			continue
		}

		var tmpl InitialPipelineTemplate
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			logger.Warnf("bootstrap: 解析流水线模板文件失败 %s: %v", fullPath, err)
			continue
		}
		if strings.TrimSpace(tmpl.ID) == "" {
			logger.Warnf("bootstrap: 流水线模板缺少 id，跳过文件 %s", fullPath)
			continue
		}
		tmplMap[tmpl.ID] = tmpl
	}
	return true
}

// loadPipelineTemplatesFromDir loads YAML templates from a directory (backward compat).
func loadPipelineTemplatesFromDir(dirPath string, tmplMap map[string]InitialPipelineTemplate) bool {
	return loadYAMLFilesFromDir(dirPath, tmplMap)
}
