package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"centag/core/pkg/logger"
)

// LoadInitialBackends 从 config/initdata/initial-backends.json 加载初始化后端配置
// 仅在数据库为空（首次启动）时执行
func LoadInitialBackends() error {
	// 获取后端管理器
	manager := getBackendManager()
	if manager == nil {
		return fmt.Errorf("backend manager not initialized - call SetBackendManager() first")
	}
	
	// 检查是否已有后端配置
	existingBackends := manager.List()
	if len(existingBackends) > 0 {
		logger.Infof("已有 %d 个后端配置，跳过初始化", len(existingBackends))
		return nil
	}

	logger.Info("未检测到后端配置，开始加载初始化数据...")

	// 读取初始化文件
	initData, err := readInitialBackendsFile()
	if err != nil {
		// 文件不存在时不报错，只是记录日志
		if os.IsNotExist(err) {
			logger.Info("初始化文件不存在，跳过初始化")
			return nil
		}
		return fmt.Errorf("读取初始化文件失败：%w", err)
	}

	if len(initData.Backends) == 0 {
		logger.Info("初始化文件中没有后端配置")
		return nil
	}

	addedCount := 0
	for _, backend := range initData.Backends {
		// 处理环境变量替换（如 ${ANTHROPIC_API_KEY}）
		backend.APIKey = replaceEnvVars(backend.APIKey)
		backend.BaseURL = replaceEnvVars(backend.BaseURL)

		// 添加时间戳
		now := time.Now().Format(time.RFC3339)
		backend.CreatedAt = now
		backend.UpdatedAt = now

		// 添加到管理器
		if err := manager.Add(&backend); err != nil {
			logger.Warnf("添加后端配置失败 [%s]: %v", backend.ID, err)
			continue
		}
		addedCount++
		logger.Infof("添加后端配置：%s (%s)", backend.Name, backend.ID)
	}

	if addedCount == 0 {
		logger.Info("未添加任何后端配置")
		return nil
	}

	// 持久化到数据库
	if err := manager.Save(); err != nil {
		return fmt.Errorf("保存后端配置失败：%w", err)
	}

	logger.Infof("初始化完成：已添加 %d 个后端配置", addedCount)
	return nil
}
// initialBackendsData 初始化数据结构
type initialBackendsData struct {
	Version     string                     `json:"version"`
	Description string                     `json:"description"`
	Backends    []BackendConfig            `json:"backends"`
}

// readInitialBackendsFile 读取初始化文件
func readInitialBackendsFile() (*initialBackendsData, error) {
	root := initdataRoot()
	if root == "" {
		return nil, os.ErrNotExist
	}

	filePath := filepath.Join(root, "initial-backends.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	logger.Infof("从 %s 读取初始化数据", filePath)

	var initData initialBackendsData
	if err := json.Unmarshal(data, &initData); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败：%w", err)
	}

	return &initData, nil
}

// initdataRoot 返回 initdata 根目录，与 bootstrap.InitdataRoot 保持一致（避免 import cycle）。
func initdataRoot() string {
	if p := os.Getenv("INITDATA_PATH"); p != "" {
		return p
	}
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	execDir := filepath.Dir(execPath)
	root := projectRootFromExecDir(execDir)
	if root == "" {
		return ""
	}
	return filepath.Join(root, "config", "initdata")
}

func projectRootFromExecDir(execDir string) string {
	if filepath.Base(execDir) == "bin" {
		parent := filepath.Dir(execDir)
		if filepath.Base(parent) == "var" {
			return filepath.Dir(parent)
		}
		return parent
	}
	return execDir
}

// replaceEnvVars 替换环境变量（格式：${VAR_NAME}）
func replaceEnvVars(s string) string {
	if !strings.Contains(s, "${") {
		return s
	}

	result := s
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		varName := result[start+2 : end]
		envValue := os.Getenv(varName)
		if envValue == "" {
			envValue = "${" + varName + "}" // 保留原样
		}

		result = result[:start] + envValue + result[end+1:]
	}

	return result
}
