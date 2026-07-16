package bootstrap

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"centag/core/pkg/logger"

	"gopkg.in/yaml.v3"
)

// LoadInitialPipelineTemplatesFromFiles 从全局和 Profile 的 config/initdata/pipeline-templates
// 目录加载 YAML 模板描述文件。Profile 模板覆盖全局同名模板。
// 仅支持 .yaml/.yml 格式。
func LoadInitialPipelineTemplatesFromFiles() []InitialPipelineTemplate {
	globalRoot, profileRoot := InitdataRoots()
	if globalRoot == "" && profileRoot == "" {
		logger.Info("bootstrap: INITDATA_PATH / ProjectRoot 未确定，跳过流水线模板加载")
		return nil
	}

	// 1. Load global templates (base)
	tmplMap := make(map[string]InitialPipelineTemplate)
	if globalRoot != "" {
		globalDir := filepath.Join(globalRoot, "pipeline-templates")
		globalLoaded := loadPipelineTemplatesFromDir(globalDir, tmplMap)
		if globalLoaded {
			logger.Infof("bootstrap: 从全局配置加载流水线模板: %s (%d 个)", globalDir, len(tmplMap))
		}
	}

	// 2. Load profile templates (overlay)
	if profileRoot != "" && profileRoot != globalRoot {
		profileDir := filepath.Join(profileRoot, "pipeline-templates")
		profileLoaded := loadPipelineTemplatesFromDir(profileDir, tmplMap)
		if profileLoaded {
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

	logger.Infof("bootstrap: 合并后流水线模板共 %d 个", len(templates))
	return templates
}

// loadPipelineTemplatesFromDir loads YAML templates from a directory into the provided map.
// Templates with the same ID will be overwritten (allowing profile to override global).
// Returns true if the directory exists and was processed.
func loadPipelineTemplatesFromDir(dirPath string, tmplMap map[string]InitialPipelineTemplate) bool {
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
