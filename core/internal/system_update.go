package internal

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"centag/core/pkg/logger"
)

// SystemUpdateHandler 系统更新处理器
type SystemUpdateHandler struct {
	updateConfigPath string
}

// UpdateConfig 更新配置结构
type UpdateConfig struct {
	Version     string         `yaml:"version"`
	Description string         `yaml:"description"`
	Files       []FileSpec     `yaml:"files"`
	InitScripts []InitScript   `yaml:"init_scripts"`
	PreChecks   []PreCheck     `yaml:"pre_checks"`
	Rollback    RollbackConfig `yaml:"rollback"`
	Behavior    BehaviorConfig `yaml:"behavior"`
}

// FileSpec 文件规格
type FileSpec struct {
	Source      string `yaml:"source"`
	Target      string `yaml:"target"`
	Permission  string `yaml:"permission"`
	Backup      bool   `yaml:"backup"`
	Recursive   bool   `yaml:"recursive"`
	Description string `yaml:"description"`
}

// InitScript 初始化脚本
type InitScript struct {
	Script      string `yaml:"script"`
	Description string `yaml:"description"`
	Timeout     int    `yaml:"timeout"`
	Required    bool   `yaml:"required"`
}

// PreCheck 预检查
type PreCheck struct {
	Check    string   `yaml:"check"`
	MinSpace string   `yaml:"min_space"`
	Paths    []string `yaml:"paths"`
}

// RollbackConfig 回滚配置
type RollbackConfig struct {
	Enabled     bool   `yaml:"enabled"`
	KeepBackups int    `yaml:"keep_backups"`
	BackupDir   string `yaml:"backup_dir"`
}

// BehaviorConfig 行为配置
type BehaviorConfig struct {
	AutoRestart        bool `yaml:"auto_restart"`
	RestartDelay       int  `yaml:"restart_delay"`
	HealthCheckTimeout int  `yaml:"health_check_timeout"`
}

// UpdateHistory 更新历史
type UpdateHistory struct {
	StartTime   time.Time    `json:"start_time"`
	EndTime     time.Time    `json:"end_time"`
	Success     bool         `json:"success"`
	Error       string       `json:"error,omitempty"`
	PackageName string       `json:"package_name"`
	Version     string       `json:"version"`
	Files       []FileSpec   `json:"files"`
	Backups     []BackupInfo `json:"backups"`
	FilesCount  int          `json:"files_count"`
	HistoryFile string       `json:"history_file"` // 历史记录文件名
}

// BackupInfo 备份信息
type BackupInfo struct {
	OriginalPath string `json:"original_path"`
	BackupPath   string `json:"backup_path"`
}

// NewSystemUpdateHandler 创建系统更新处理器
func NewSystemUpdateHandler(updateConfigPath string) *SystemUpdateHandler {
	return &SystemUpdateHandler{
		updateConfigPath: updateConfigPath,
	}
}

// HandleUpdate 处理系统更新请求
func (h *SystemUpdateHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	// 验证请求方法
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析上传的文件
	file, header, err := r.FormFile("package")
	if err != nil {
		http.Error(w, "Missing package file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 验证文件格式
	if !strings.HasSuffix(header.Filename, ".tar.gz") {
		http.Error(w, "Package must be a .tar.gz file", http.StatusBadRequest)
		return
	}

	// 验证文件大小（500MB限制）
	const maxFileSize = 500 * 1024 * 1024
	if header.Size > maxFileSize {
		http.Error(w, "Package size exceeds 500MB limit", http.StatusBadRequest)
		return
	}

	logger.Infof("[系统更新] 开始处理更新包: %s", header.Filename)

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "centag-update-*")
	if err != nil {
		logger.Errorf("[系统更新] 创建临时目录失败: %v", err)
		http.Error(w, "Failed to create temp directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// 保存更新包
	packagePath := filepath.Join(tempDir, header.Filename)
	if err := saveFile(file, packagePath); err != nil {
		logger.Errorf("[系统更新] 保存更新包失败: %v", err)
		http.Error(w, "Failed to save package", http.StatusInternalServerError)
		return
	}

	// 计算 SHA256
	sha256sum, err := calculateSHA256(packagePath)
	if err != nil {
		logger.Errorf("[系统更新] 计算 SHA256 失败: %v", err)
		http.Error(w, "Failed to calculate SHA256", http.StatusInternalServerError)
		return
	}

	logger.Infof("[系统更新] 更新包 SHA256: %s", sha256sum[:8])

	// 解压更新包
	extractDir := filepath.Join(tempDir, "extracted")
	if err := extractTarGz(packagePath, extractDir); err != nil {
		logger.Errorf("[系统更新] 解压更新包失败: %v", err)
		http.Error(w, "Failed to extract package", http.StatusInternalServerError)
		return
	}

	// 读取更新配置
	configPath := filepath.Join(extractDir, "update_config.yml")
	config, err := readUpdateConfig(configPath)
	if err != nil {
		logger.Errorf("[系统更新] 读取更新配置失败: %v", err)
		http.Error(w, "Failed to read update config", http.StatusBadRequest)
		return
	}

	// 获取可执行文件所在目录
	execDir, err := getExecDir()
	if err != nil {
		logger.Errorf("[系统更新] 获取可执行文件目录失败: %v", err)
		http.Error(w, "Failed to get executable directory", http.StatusInternalServerError)
		return
	}

	// 创建更新历史
	history := &UpdateHistory{
		StartTime:   time.Now(),
		PackageName: header.Filename,
		Version:     config.Version,
		Files:       config.Files,
	}

	// 执行更新
	var backups []BackupInfo
	updateSucceeded := true
	for _, fileSpec := range config.Files {
		sourcePath, err := safeJoinUnderBase(extractDir, fileSpec.Source)
		if err != nil {
			logger.Errorf("[系统更新] 非法 source 路径: %v", err)
			history.Success = false
			history.Error = err.Error()
			updateSucceeded = false
			break
		}

		targetPath, err := safeJoinUnderBase(execDir, fileSpec.Target)
		if err != nil {
			logger.Errorf("[系统更新] 非法 target 路径: %v", err)
			history.Success = false
			history.Error = err.Error()
			updateSucceeded = false
			break
		}

		logger.Infof("[系统更新] 处理文件: %s -> %s", sourcePath, targetPath)

		// 备份原文件
		if fileSpec.Backup {
			backupPath := filepath.Join(execDir, "storage", "backups", filepath.Base(targetPath)+"."+time.Now().Format("20060102150405"))
			// 创建备份目录
			if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
				logger.Errorf("[系统更新] 创建备份目录失败: %v", err)
			}
			if _, err := os.Stat(targetPath); err == nil {
				var err error
				if fileSpec.Recursive {
					// 递归备份目录
					err = copyDir(targetPath, backupPath)
				} else {
					// 备份文件
					err = copyFile(targetPath, backupPath)
				}
				if err != nil {
					logger.Errorf("[系统更新] 备份失败: %v", err)
				} else {
					backups = append(backups, BackupInfo{
						OriginalPath: targetPath,
						BackupPath:   backupPath,
					})
					logger.Infof("[系统更新] 已备份: %s -> %s", targetPath, backupPath)
				}
			}
		}

		// 复制文件
		if fileSpec.Recursive {
			logger.Infof("[系统更新] 递归复制目录: %s -> %s", sourcePath, targetPath)
			if err := copyDir(sourcePath, targetPath); err != nil {
				logger.Errorf("[系统更新] 复制目录失败: %v", err)
				history.Success = false
				history.Error = err.Error()
				updateSucceeded = false
				break
			}
			logger.Infof("[系统更新] 目录复制成功: %s", targetPath)
		} else {
			logger.Infof("[系统更新] 复制文件: %s -> %s", sourcePath, targetPath)
			if err := copyFile(sourcePath, targetPath); err != nil {
				logger.Errorf("[系统更新] 复制文件失败: %v", err)
				history.Success = false
				history.Error = err.Error()
				updateSucceeded = false
				break
			}
			logger.Infof("[系统更新] 文件复制成功: %s", targetPath)
		}
	}

	// 仅在全量更新无错误时标记成功
	history.Success = updateSucceeded

	history.Backups = backups
	history.FilesCount = len(config.Files)
	history.EndTime = time.Now()

	// 执行初始化脚本
	if config.Behavior.AutoRestart && history.Success {
		for _, script := range config.InitScripts {
			scriptPath, err := safeJoinUnderBase(extractDir, script.Script)
			if err != nil {
				logger.Warnf("[系统更新] 跳过非法 init script 路径: %v", err)
				continue
			}
			if _, err := os.Stat(scriptPath); err == nil {
				logger.Infof("[系统更新] 执行脚本: %s", script.Script)
				// TODO: 执行脚本
			}
		}
	}

	// 保存更新历史
	historyDir := filepath.Join(execDir, "storage", "update-history")
	os.MkdirAll(historyDir, 0755)
	historyFileName := fmt.Sprintf("update-%d-%s.json", time.Now().Unix(), sha256sum[:8])
	historyPath := filepath.Join(historyDir, historyFileName)

	historyData, _ := json.MarshalIndent(history, "", "  ")
	os.WriteFile(historyPath, historyData, 0644)

	// 返回结果
	response := map[string]interface{}{
		"success": history.Success,
		"message": "Update completed",
	}

	if history.Success {
		logger.Info("[系统更新] 更新文件已就绪，准备重启服务...")

		// 通知守护进程进行热更新（写入标记文件）
		if config.Behavior.AutoRestart {
			// 写入热更新标记文件
			updateMarker := filepath.Join(execDir, "storage", "update_marker")
			markerData := map[string]string{
				"timestamp": time.Now().Format(time.RFC3339),
				"version":   config.Version,
				"history":   historyFileName,
			}
			markerJSON, _ := json.Marshal(markerData)
			os.WriteFile(updateMarker, markerJSON, 0644)
			logger.Infof("[系统更新] 已写入更新标记: %s", updateMarker)

			// 优雅停止 HTTP 服务器，等待当前请求处理完毕
			go func() {
				// 给 HTTP 服务器一些时间来处理正在进行的请求
				time.Sleep(2 * time.Second)

				// 写入停止标记，通知守护进程
				stopMarker := filepath.Join(execDir, "storage", "update_stop")
				os.WriteFile(stopMarker, []byte(time.Now().Format(time.RFC3339)), 0644)
				logger.Info("[系统更新] 已写入停止标记，守护进程将重启服务...")

				// 延迟退出，确保标记文件已写入
				time.Sleep(1 * time.Second)
				os.Exit(0)
			}()
		}
	} else {
		logger.Errorf("[系统更新] 更新失败: %s", history.Error)
		response["error"] = history.Error
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleUpdateHistory 获取更新历史
func (h *SystemUpdateHandler) HandleUpdateHistory(w http.ResponseWriter, r *http.Request) {
	execDir, err := getExecDir()
	if err != nil {
		http.Error(w, "Failed to get executable directory", http.StatusInternalServerError)
		return
	}

	historyDir := filepath.Join(execDir, "storage", "update-history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"history": []interface{}{},
				"total":   0,
			})
			return
		}
		http.Error(w, "Failed to read history directory", http.StatusInternalServerError)
		return
	}

	var histories []UpdateHistory
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(historyDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var history UpdateHistory
		if err := json.Unmarshal(data, &history); err != nil {
			continue
		}

		// 将历史文件名添加到记录中
		history.HistoryFile = entry.Name()
		histories = append(histories, history)
	}

	// 按时间倒序排列
	for i := 0; i < len(histories); i++ {
		for j := i + 1; j < len(histories); j++ {
			if histories[i].StartTime.Before(histories[j].StartTime) {
				histories[i], histories[j] = histories[j], histories[i]
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"history": histories,
		"total":   len(histories),
	})
}

// HandleRollback 处理版本回退请求
func (h *SystemUpdateHandler) HandleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	historyFile, err := resolveHistoryFileParam(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("解析请求失败: %v", err), http.StatusBadRequest)
		return
	}

	logger.Infof("[版本回退] 请求回退到: %s", historyFile)

	// 获取可执行文件所在目录
	execDir, err := getExecDir()
	if err != nil {
		logger.Errorf("[版本回退] 获取可执行文件目录失败: %v", err)
		http.Error(w, "Failed to get executable directory", http.StatusInternalServerError)
		return
	}

	// 读取历史记录
	historyPath := filepath.Join(execDir, "storage", "update-history", historyFile)
	historyData, err := os.ReadFile(historyPath)
	if err != nil {
		logger.Errorf("[版本回退] 读取历史记录失败: %v", err)
		http.Error(w, fmt.Sprintf("历史记录不存在: %v", err), http.StatusNotFound)
		return
	}

	var originalHistory UpdateHistory
	if err := json.Unmarshal(historyData, &originalHistory); err != nil {
		logger.Errorf("[版本回退] 解析历史记录失败: %v", err)
		http.Error(w, "解析历史记录失败", http.StatusInternalServerError)
		return
	}

	// 检查该记录是否更新成功
	if !originalHistory.Success {
		logger.Warnf("[版本回退] 无法回退到失败的更新版本")
		http.Error(w, "无法回退到失败的更新版本", http.StatusBadRequest)
		return
	}

	logger.Infof("[版本回退] 开始回退操作，版本: %s", originalHistory.Version)

	// 创建回退日志
	rollbackHistory := &UpdateHistory{
		StartTime:   time.Now(),
		PackageName: originalHistory.PackageName + " (回退)",
		Version:     originalHistory.Version,
		Files:       originalHistory.Files,
		Success:     false,
	}

	// 执行回退：恢复备份文件
	for i := len(originalHistory.Backups) - 1; i >= 0; i-- {
		backup := originalHistory.Backups[i]
		logger.Infof("[版本回退] 恢复: %s <- %s", backup.OriginalPath, backup.BackupPath)

		// 检查备份是文件还是目录
		fileInfo, err := os.Stat(backup.BackupPath)
		if err != nil {
			logger.Errorf("[版本回退] 检查备份类型失败: %v", err)
			rollbackHistory.Error = err.Error()
			rollbackHistory.EndTime = time.Now()

			// 保存回退历史
			saveUpdateHistory(execDir, rollbackHistory)
			http.Error(w, fmt.Sprintf("回退失败: %v", err), http.StatusInternalServerError)
			return
		}

		// 根据类型选择复制方式
		if fileInfo.IsDir() {
			// 复制目录
			if err := copyDir(backup.BackupPath, backup.OriginalPath); err != nil {
				logger.Errorf("[版本回退] 恢复目录失败: %v", err)
				rollbackHistory.Error = err.Error()
				rollbackHistory.EndTime = time.Now()

				// 保存回退历史
				saveUpdateHistory(execDir, rollbackHistory)
				http.Error(w, fmt.Sprintf("回退失败: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			// 复制文件
			if err := copyFile(backup.BackupPath, backup.OriginalPath); err != nil {
				logger.Errorf("[版本回退] 恢复文件失败: %v", err)
				rollbackHistory.Error = err.Error()
				rollbackHistory.EndTime = time.Now()

				// 保存回退历史
				saveUpdateHistory(execDir, rollbackHistory)
				http.Error(w, fmt.Sprintf("回退失败: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}

	rollbackHistory.Success = true
	rollbackHistory.FilesCount = len(originalHistory.Backups)
	rollbackHistory.EndTime = time.Now()

	// 保存回退历史
	saveUpdateHistory(execDir, rollbackHistory)

	logger.Info("[版本回退] 回退成功")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "回退成功",
	})
}

// HandleDelete 处理删除更新包请求
func (h *SystemUpdateHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	historyFile, err := resolveHistoryFileParam(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("解析请求失败: %v", err), http.StatusBadRequest)
		return
	}

	logger.Infof("[删除更新包] 请求删除: %s", historyFile)

	// 获取可执行文件所在目录
	execDir, err := getExecDir()
	if err != nil {
		logger.Errorf("[删除更新包] 获取可执行文件目录失败: %v", err)
		http.Error(w, "Failed to get executable directory", http.StatusInternalServerError)
		return
	}

	// 读取历史记录
	historyPath := filepath.Join(execDir, "storage", "update-history", historyFile)
	historyData, err := os.ReadFile(historyPath)
	if err != nil {
		logger.Errorf("[删除更新包] 读取历史记录失败: %v", err)
		http.Error(w, fmt.Sprintf("历史记录不存在: %v", err), http.StatusNotFound)
		return
	}

	var log UpdateHistory
	if err := json.Unmarshal(historyData, &log); err != nil {
		logger.Errorf("[删除更新包] 解析历史记录失败: %v", err)
		http.Error(w, "解析历史记录失败", http.StatusInternalServerError)
		return
	}

	// 删除备份文件（如果存在）
	for _, backup := range log.Backups {
		if _, err := os.Stat(backup.BackupPath); err == nil {
			if err := os.Remove(backup.BackupPath); err != nil {
				logger.Warnf("[删除更新包] 删除备份文件失败: %v", err)
			} else {
				logger.Infof("[删除更新包] 已删除备份: %s", backup.BackupPath)
			}
		}
	}

	// 删除历史记录
	if err := os.Remove(historyPath); err != nil {
		logger.Errorf("[删除更新包] 删除历史记录失败: %v", err)
		http.Error(w, fmt.Sprintf("删除历史记录失败: %v", err), http.StatusInternalServerError)
		return
	}

	logger.Infof("[删除更新包] 已删除历史记录: %s", historyPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "删除成功",
	})
}

// saveUpdateHistory 保存更新历史到文件
func saveUpdateHistory(execDir string, history *UpdateHistory) error {
	historyDir := filepath.Join(execDir, "storage", "update-history")
	os.MkdirAll(historyDir, 0755)

	historyData, _ := json.MarshalIndent(history, "", "  ")
	historyFileName := fmt.Sprintf("update-%d.json", time.Now().Unix())
	historyPath := filepath.Join(historyDir, historyFileName)

	return os.WriteFile(historyPath, historyData, 0644)
}

func resolveHistoryFileParam(r *http.Request) (string, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		var req struct {
			HistoryFile string `json:"history_file"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", err
		}
		historyFile := strings.TrimSpace(req.HistoryFile)
		if historyFile == "" {
			return "", fmt.Errorf("缺少历史记录文件名")
		}
		return historyFile, nil
	}

	if strings.Contains(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			return "", err
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return "", err
		}
	}

	historyFile := strings.TrimSpace(r.FormValue("history_file"))
	if historyFile == "" {
		return "", fmt.Errorf("缺少历史记录文件名")
	}
	return historyFile, nil
}

// 辅助函数
func saveFile(src io.Reader, dst string) error {
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, src)
	return err
}

func calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func extractTarGz(src, dst string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	// 检测并跳过顶层目录
	var prefix string
	var prefixFound bool

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// 首次遍历：确定顶层目录前缀
		if !prefixFound {
			parts := strings.SplitN(header.Name, "/", 2)
			if len(parts) > 1 && header.Typeflag == tar.TypeDir {
				prefix = parts[0] + "/"
				prefixFound = true
			}
			// 回到开始重新遍历
			if prefixFound {
				_, err := file.Seek(0, 0)
				if err != nil {
					return err
				}
				gzReader, err = gzip.NewReader(file)
				if err != nil {
					return err
				}
				defer gzReader.Close()
				tarReader = tar.NewReader(gzReader)
				continue
			}
		}

		// 跳过顶层目录
		cleanName := header.Name
		if prefixFound && strings.HasPrefix(header.Name, prefix) {
			cleanName = strings.TrimPrefix(header.Name, prefix)
		}

		// 跳过空路径
		if cleanName == "" || cleanName == "." {
			continue
		}

		targetPath, err := safeJoinUnderBase(dst, cleanName)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

func readUpdateConfig(path string) (*UpdateConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config UpdateConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func getExecDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.Dir(execPath), nil
}

func safeJoinUnderBase(base, relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return "", fmt.Errorf("路径不能为空")
	}

	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("不允许绝对路径: %s", relativePath)
	}

	joined := filepath.Join(base, relativePath)
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(absBase, absJoined)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("路径越界: %s", relativePath)
	}

	return absJoined, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 使用临时文件，确保原子性替换
	tempDst := dst + ".tmp." + time.Now().Format("20060102150405")
	dstFile, err := os.Create(tempDst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		os.Remove(tempDst)
		return err
	}
	dstFile.Close()

	// 获取源文件权限
	info, err := os.Stat(src)
	if err != nil {
		os.Remove(tempDst)
		return err
	}

	// 设置权限
	if err := os.Chmod(tempDst, info.Mode()); err != nil {
		os.Remove(tempDst)
		return err
	}

	// 原子性替换
	if err := os.Rename(tempDst, dst); err != nil {
		os.Remove(tempDst)
		return fmt.Errorf("原子性替换文件失败: %w", err)
	}

	return nil
}

func copyDir(src, dst string) error {
	// 预检查：源目录是否存在
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("源目录不存在: %s", src)
		}
		return fmt.Errorf("检查源目录失败: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("源路径不是目录: %s", src)
	}

	// 创建目标目录
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 读取源目录内容
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %w", err)
	}

	// 递归复制每个条目
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// 递归复制子目录
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// 复制文件
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
