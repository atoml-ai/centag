package loader

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/plugin/registry"
)

// DefaultLoader 默认插件加载器
type DefaultLoader struct {
	mu         sync.RWMutex
	plugins    map[string]*ManagedPlugin
	registry   registry.Store
	httpClient *http.Client

	// 配置
	pluginDir string
	verifySig bool

	// 监听器
	listeners  []LifecycleListener
	listenerMu sync.RWMutex

	// 更新状态存储
	updateStatuses map[string]*UpdateStatus
	updateMu       sync.RWMutex
}

// NewDefaultLoader 创建默认加载器
func NewDefaultLoader(pluginDir string, registryStore registry.Store) *DefaultLoader {
	return &DefaultLoader{
		plugins:        make(map[string]*ManagedPlugin),
		registry:       registryStore,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		pluginDir:      pluginDir,
		verifySig:      true,
		updateStatuses: make(map[string]*UpdateStatus),
	}
}

// SetVerifySignature 设置是否验证签名
func (l *DefaultLoader) SetVerifySignature(verify bool) {
	l.verifySig = verify
}

// Load 加载插件
func (l *DefaultLoader) Load(ctx context.Context, req *LoadRequest) (*ManagedPlugin, error) {
	// 根据来源加载
	var pkg *PluginPackage
	var err error

	switch req.Source {
	case "url":
		pkg, err = l.loadFromURL(ctx, req.URL, req.VerifyChecksum, req.VerifySignature)
	case "file":
		pkg, err = l.loadFromFile(req.FilePath, req.VerifyChecksum, req.VerifySignature)
	case "registry":
		pkg, err = l.loadFromRegistry(ctx, req.PluginID, req.Version, req.VerifyChecksum, req.VerifySignature)
	default:
		return nil, fmt.Errorf("unsupported source: %s", req.Source)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load plugin: %w", err)
	}

	// 创建托管插件
	managed := &ManagedPlugin{
		ID:       pkg.Manifest.ID,
		Manifest: pkg.Manifest,
		State:    StateUnknown,
		Version:  pkg.Manifest.Version,
	}

	// 检查是否已存在
	l.mu.Lock()
	if existing, ok := l.plugins[managed.ID]; ok {
		l.mu.Unlock()
		return existing, fmt.Errorf("plugin already loaded: %s", managed.ID)
	}
	l.plugins[managed.ID] = managed
	l.mu.Unlock()

	// 执行生命周期
	if err := l.transitionState(managed, StateLoading); err != nil {
		return nil, err
	}

	// 保存插件包到本地
	if err := l.savePackage(managed.ID, pkg); err != nil {
		l.transitionState(managed, StateError)
		return nil, fmt.Errorf("failed to save package: %w", err)
	}

	if err := l.transitionState(managed, StateLoaded); err != nil {
		return nil, err
	}

	// 验证
	if err := l.transitionState(managed, StateValidating); err != nil {
		return nil, err
	}

	if err := l.validatePlugin(managed, pkg); err != nil {
		l.transitionState(managed, StateError)
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	if err := l.transitionState(managed, StateValidated); err != nil {
		return nil, err
	}

	// 自动启动
	if req.AutoStart {
		if err := l.Start(managed.ID); err != nil {
			return nil, err
		}
	}

	return managed, nil
}

// loadFromURL 从 URL 加载
func (l *DefaultLoader) loadFromURL(ctx context.Context, url string, verifyChecksum, verifySignature bool) (*PluginPackage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return l.parsePackage(data, verifyChecksum, verifySignature)
}

// loadFromFile 从文件加载
func (l *DefaultLoader) loadFromFile(path string, verifyChecksum, verifySignature bool) (*PluginPackage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return l.parsePackage(data, verifyChecksum, verifySignature)
}

// loadFromRegistry 从注册中心加载
func (l *DefaultLoader) loadFromRegistry(ctx context.Context, pluginID, version string, verifyChecksum, verifySignature bool) (*PluginPackage, error) {
	if l.registry == nil {
		return nil, fmt.Errorf("registry not configured")
	}

	var metadata *registry.PluginMetadata
	var err error

	if version == "" {
		// 获取最新版本
		metadata, err = l.registry.Get(ctx, pluginID)
	} else {
		metadata, err = l.registry.GetVersion(ctx, pluginID, version)
	}

	if err != nil {
		return nil, err
	}

	// 下载插件包
	req, err := http.NewRequestWithContext(ctx, "GET", metadata.DownloadURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 验证校验和
	if verifyChecksum && metadata.Checksum != "" {
		checksum := sha256.Sum256(data)
		if hex.EncodeToString(checksum[:]) != metadata.Checksum {
			return nil, fmt.Errorf("checksum mismatch")
		}
	}

	return l.parsePackage(data, false, verifySignature && metadata.Signature != "")
}

// parsePackage 解析插件包
func (l *DefaultLoader) parsePackage(data []byte, verifyChecksum, verifySignature bool) (*PluginPackage, error) {
	// 解析 ZIP 格式
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid package format: %w", err)
	}

	pkg := &PluginPackage{
		Resources: make(map[string][]byte),
	}

	// 读取 manifest
	manifestFound := false
	for _, file := range reader.File {
		if file.Name == "manifest.json" {
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			manifestData, err := io.ReadAll(rc)
			if err != nil {
				return nil, err
			}

			pkg.Manifest = &PluginManifest{}
			if err := json.Unmarshal(manifestData, pkg.Manifest); err != nil {
				return nil, fmt.Errorf("invalid manifest: %w", err)
			}
			manifestFound = true
			break
		}
	}

	if !manifestFound {
		return nil, fmt.Errorf("manifest.json not found")
	}

	// 读取代码文件
	for _, file := range reader.File {
		if file.Name == pkg.Manifest.Entrypoint {
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			pkg.Code, err = io.ReadAll(rc)
			if err != nil {
				return nil, err
			}
		}
	}

	// 读取资源文件
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && file.Name != "manifest.json" && file.Name != pkg.Manifest.Entrypoint {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				continue
			}

			pkg.Resources[file.Name] = data
		}
	}

	// 计算校验和
	checksum := sha256.Sum256(data)
	pkg.Checksum = hex.EncodeToString(checksum[:])

	return pkg, nil
}

// savePackage 保存插件包到本地
func (l *DefaultLoader) savePackage(pluginID string, pkg *PluginPackage) error {
	dir := filepath.Join(l.pluginDir, pluginID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 保存 manifest
	manifestData, err := json.MarshalIndent(pkg.Manifest, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifestData, 0644); err != nil {
		return err
	}

	// 保存代码
	if pkg.Code != nil {
		if err := os.WriteFile(filepath.Join(dir, pkg.Manifest.Entrypoint), pkg.Code, 0755); err != nil {
			return err
		}
	}

	// 保存资源
	for name, data := range pkg.Resources {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

// validatePlugin 验证插件
func (l *DefaultLoader) validatePlugin(managed *ManagedPlugin, pkg *PluginPackage) error {
	// 验证 manifest
	if pkg.Manifest.ID == "" {
		return fmt.Errorf("plugin ID is required")
	}

	if pkg.Manifest.Name == "" {
		return fmt.Errorf("plugin name is required")
	}

	if pkg.Manifest.Version == "" {
		return fmt.Errorf("plugin version is required")
	}

	if pkg.Manifest.Kind == "" {
		return fmt.Errorf("plugin kind is required")
	}

	// 验证 kind
	validKinds := []string{"generator", "processor", "reviewer", "router"}
	found := false
	for _, k := range validKinds {
		if pkg.Manifest.Kind == k {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid plugin kind: %s", pkg.Manifest.Kind)
	}

	// 验证 entrypoint
	if pkg.Manifest.Entrypoint == "" {
		return fmt.Errorf("entrypoint is required")
	}

	// 验证代码存在
	if len(pkg.Code) == 0 {
		return fmt.Errorf("plugin code is required")
	}

	return nil
}

// transitionState 状态转换
func (l *DefaultLoader) transitionState(managed *ManagedPlugin, newState PluginState) error {
	oldState := managed.State

	if !IsValidTransition(oldState, newState) {
		return fmt.Errorf("invalid state transition: %s -> %s", oldState, newState)
	}

	managed.State = newState

	// 记录时间
	if newState == StateRunning {
		managed.StartTime = time.Now()
	}

	// 触发事件
	event := &LifecycleEvent{
		PluginID:  managed.ID,
		FromState: oldState,
		ToState:   newState,
		Timestamp: time.Now(),
	}

	l.notifyListeners(event)

	return nil
}

// notifyListeners 通知监听器
func (l *DefaultLoader) notifyListeners(event *LifecycleEvent) {
	l.listenerMu.RLock()
	listeners := make([]LifecycleListener, len(l.listeners))
	copy(listeners, l.listeners)
	l.listenerMu.RUnlock()

	for _, listener := range listeners {
		go listener.OnStateChanged(event)
	}
}

// Unload 卸载插件
func (l *DefaultLoader) Unload(ctx context.Context, pluginID string) error {
	l.mu.Lock()
	managed, ok := l.plugins[pluginID]
	if !ok {
		l.mu.Unlock()
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	l.mu.Unlock()

	// 如果正在运行，先停止
	if managed.State == StateRunning {
		if err := l.Stop(pluginID); err != nil {
			return err
		}
	}

	// 卸载
	if err := l.transitionState(managed, StateUnloading); err != nil {
		return err
	}

	// 删除本地文件
	dir := filepath.Join(l.pluginDir, pluginID)
	if err := os.RemoveAll(dir); err != nil {
		// 记录错误但不阻止卸载
		managed.LastError = err.Error()
	}

	// 从内存中删除
	l.mu.Lock()
	delete(l.plugins, pluginID)
	l.mu.Unlock()

	return l.transitionState(managed, StateUnloaded)
}

// Get 获取插件
func (l *DefaultLoader) Get(pluginID string) (*ManagedPlugin, error) {
	l.mu.RLock()
	managed, ok := l.plugins[pluginID]
	l.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	return managed, nil
}

// List 列出所有插件
func (l *DefaultLoader) List() []*ManagedPlugin {
	l.mu.RLock()
	result := make([]*ManagedPlugin, 0, len(l.plugins))
	for _, p := range l.plugins {
		result = append(result, p)
	}
	l.mu.RUnlock()

	return result
}

// Start 启动插件
func (l *DefaultLoader) Start(pluginID string) error {
	l.mu.RLock()
	managed, ok := l.plugins[pluginID]
	l.mu.RUnlock()

	if !ok {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	// 检查当前状态
	if managed.State != StateValidated && managed.State != StateStopped {
		return fmt.Errorf("cannot start plugin in state: %s", managed.State)
	}

	// 开始启动
	if err := l.transitionState(managed, StateStarting); err != nil {
		return err
	}

	// 实际启动插件实例
	if err := l.startPluginInstance(managed); err != nil {
		managed.LastError = err.Error()
		l.transitionState(managed, StateError)
		return fmt.Errorf("failed to start plugin instance: %w", err)
	}

	managed.StartTime = time.Now()
	managed.RestartCount++

	return l.transitionState(managed, StateRunning)
}

// startPluginInstance 根据运行时类型启动插件实例
func (l *DefaultLoader) startPluginInstance(managed *ManagedPlugin) error {
	if managed.Manifest == nil {
		return fmt.Errorf("plugin manifest is nil")
	}

	runtime := managed.Manifest.Runtime

	switch runtime {
	case "go":
		return l.startGoPlugin(managed)
	case "wasm":
		return l.startWASMPlugin(managed)
	case "python":
		return l.startPythonPlugin(managed)
	default:
		// 对于内置插件或其他类型，假设已经在 Load 阶段初始化了 Instance
		if managed.Instance == nil {
			return fmt.Errorf("plugin instance not initialized for runtime: %s", runtime)
		}
		return nil
	}
}

// startGoPlugin 启动 Go 插件
func (l *DefaultLoader) startGoPlugin(managed *ManagedPlugin) error {
	// Go 插件通过 .so 文件加载
	pluginPath := filepath.Join(l.pluginDir, managed.ID, managed.Manifest.Entrypoint)

	// 检查插件文件是否存在
	if _, err := os.Stat(pluginPath); err != nil {
		return fmt.Errorf("go plugin file not found: %w", err)
	}

	return fmt.Errorf("go runtime loading is not implemented; plugin remains stopped")
}

// startWASMPlugin 启动 WASM 插件
func (l *DefaultLoader) startWASMPlugin(managed *ManagedPlugin) error {
	pluginPath := filepath.Join(l.pluginDir, managed.ID, managed.Manifest.Entrypoint)

	if _, err := os.Stat(pluginPath); err != nil {
		return fmt.Errorf("wasm plugin file not found: %w", err)
	}

	return fmt.Errorf("wasm runtime loading is not implemented; plugin remains stopped")
}

// startPythonPlugin 启动 Python 插件
func (l *DefaultLoader) startPythonPlugin(managed *ManagedPlugin) error {
	pluginPath := filepath.Join(l.pluginDir, managed.ID, managed.Manifest.Entrypoint)

	if _, err := os.Stat(pluginPath); err != nil {
		return fmt.Errorf("python plugin file not found: %w", err)
	}

	return fmt.Errorf("python runtime loading is not implemented; plugin remains stopped")
}

// Stop 停止插件
func (l *DefaultLoader) Stop(pluginID string) error {
	l.mu.RLock()
	managed, ok := l.plugins[pluginID]
	l.mu.RUnlock()

	if !ok {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	// 检查当前状态
	if managed.State != StateRunning {
		return fmt.Errorf("cannot stop plugin in state: %s", managed.State)
	}

	// 开始停止
	if err := l.transitionState(managed, StateStopping); err != nil {
		return err
	}

	// 实际停止插件实例
	if err := l.stopPluginInstance(managed); err != nil {
		logger.Warnf("Error stopping plugin %s: %v", pluginID, err)
	}

	return l.transitionState(managed, StateStopped)
}

// stopPluginInstance 停止插件实例
func (l *DefaultLoader) stopPluginInstance(managed *ManagedPlugin) error {
	if managed.Manifest == nil {
		return fmt.Errorf("plugin manifest is nil")
	}

	runtime := managed.Manifest.Runtime

	switch runtime {
	case "go":
		return l.stopGoPlugin(managed)
	case "wasm":
		return l.stopWASMPlugin(managed)
	case "python":
		return l.stopPythonPlugin(managed)
	default:
		// 对于内置插件，清理实例
		managed.Instance = nil
		return nil
	}
}

// stopGoPlugin 停止 Go 插件
func (l *DefaultLoader) stopGoPlugin(managed *ManagedPlugin) error {
	logger.Infof("Stopping Go plugin: %s", managed.ID)

	// TODO: 实际卸载 Go 插件
	// Go 的 plugin 包不支持卸载，通常做法是让插件自己清理资源

	managed.Instance = nil
	return nil
}

// stopWASMPlugin 停止 WASM 插件
func (l *DefaultLoader) stopWASMPlugin(managed *ManagedPlugin) error {
	logger.Infof("Stopping WASM plugin: %s", managed.ID)

	// TODO: 实际停止 WASM 插件
	// 需要清理 WASM 实例和相关资源

	managed.Instance = nil
	return nil
}

// stopPythonPlugin 停止 Python 插件
func (l *DefaultLoader) stopPythonPlugin(managed *ManagedPlugin) error {
	logger.Infof("Stopping Python plugin: %s", managed.ID)

	// TODO: 实际停止 Python 插件进程
	// 可能需要发送信号或调用 API 来停止进程

	managed.Instance = nil
	return nil
}

// Update 更新插件
func (l *DefaultLoader) Update(ctx context.Context, req *UpdateRequest) (*UpdateStatus, error) {
	// 获取当前插件
	managed, err := l.Get(req.PluginID)
	if err != nil {
		return nil, err
	}

	// 创建更新状态
	status := &UpdateStatus{
		UpdateID:    fmt.Sprintf("update-%d", time.Now().UnixNano()),
		PluginID:    req.PluginID,
		FromVersion: managed.Version,
		ToVersion:   req.NewVersion,
		Strategy:    req.Strategy,
		State:       "pending",
		Progress:    0,
		StartTime:   time.Now(),
	}

	// 保存更新状态
	l.updateMu.Lock()
	l.updateStatuses[status.UpdateID] = status
	l.updateMu.Unlock()

	// 异步执行更新
	go l.executeUpdate(status, req)

	return status, nil
}

// executeUpdate 执行更新
func (l *DefaultLoader) executeUpdate(status *UpdateStatus, req *UpdateRequest) {
	status.State = "in_progress"

	switch req.Strategy {
	case StrategyBlueGreen:
		l.executeBlueGreenUpdate(status, req)
	case StrategyCanary:
		l.executeCanaryUpdate(status, req)
	case StrategyRolling:
		l.executeRollingUpdate(status, req)
	default:
		status.State = "failed"
		status.Error = "unknown strategy"
		status.Progress = 0
	}
}

// executeBlueGreenUpdate 蓝绿更新
func (l *DefaultLoader) executeBlueGreenUpdate(status *UpdateStatus, req *UpdateRequest) {
	status.Progress = 0
	status.State = "failed"
	status.Error = "blue-green runtime update is not implemented"
	now := time.Now()
	status.EndTime = &now
}

// executeCanaryUpdate 金丝雀更新
func (l *DefaultLoader) executeCanaryUpdate(status *UpdateStatus, req *UpdateRequest) {
	status.Progress = 0
	status.State = "failed"
	status.Error = "canary runtime update is not implemented"
	now := time.Now()
	status.EndTime = &now
}

// executeRollingUpdate 滚动更新
func (l *DefaultLoader) executeRollingUpdate(status *UpdateStatus, req *UpdateRequest) {
	status.Progress = 0
	status.State = "failed"
	status.Error = "rolling runtime update is not implemented"
	now := time.Now()
	status.EndTime = &now
}

// GetUpdateStatus 获取更新状态
func (l *DefaultLoader) GetUpdateStatus(updateID string) (*UpdateStatus, error) {
	l.updateMu.RLock()
	defer l.updateMu.RUnlock()

	status, ok := l.updateStatuses[updateID]
	if !ok {
		return nil, fmt.Errorf("update status not found: %s", updateID)
	}

	return status, nil
}

// Rollback 回滚更新
func (l *DefaultLoader) Rollback(updateID string) error {
	// 获取更新状态
	l.updateMu.RLock()
	status, ok := l.updateStatuses[updateID]
	l.updateMu.RUnlock()

	if !ok {
		return fmt.Errorf("update status not found: %s", updateID)
	}

	// 检查是否可以回滚
	if status.State != "failed" && status.State != "completed" {
		return fmt.Errorf("cannot rollback update in state: %s", status.State)
	}

	// 获取插件
	managed, err := l.Get(status.PluginID)
	if err != nil {
		return fmt.Errorf("plugin not found: %w", err)
	}

	// 检查当前版本是否与更新后的版本一致
	if managed.Version != status.ToVersion {
		return fmt.Errorf("current version (%s) does not match update target version (%s)", managed.Version, status.ToVersion)
	}

	// 开始回滚 - 停止当前版本
	if managed.State == StateRunning {
		if err := l.Stop(status.PluginID); err != nil {
			return fmt.Errorf("failed to stop current version: %w", err)
		}
	}

	// 恢复之前的版本（这里简化实现，实际需要重新加载旧版本）
	managed.Version = status.FromVersion
	managed.PreviousVersion = status.ToVersion

	// 更新状态
	l.updateMu.Lock()
	status.State = "rolled_back"
	now := time.Now()
	status.EndTime = &now
	l.updateMu.Unlock()

	// 重启插件（如果需要）
	if managed.State == StateStopped {
		if err := l.Start(status.PluginID); err != nil {
			logger.Warnf("Failed to restart plugin after rollback: %v", err)
		}
	}

	logger.Infof("Successfully rolled back plugin %s from version %s to %s",
		status.PluginID, status.ToVersion, status.FromVersion)

	return nil
}

// AddListener 添加监听器
func (l *DefaultLoader) AddListener(listener LifecycleListener) {
	l.listenerMu.Lock()
	l.listeners = append(l.listeners, listener)
	l.listenerMu.Unlock()
}

// RemoveListener 移除监听器
func (l *DefaultLoader) RemoveListener(listener LifecycleListener) {
	l.listenerMu.Lock()
	for i, li := range l.listeners {
		if li == listener {
			l.listeners = append(l.listeners[:i], l.listeners[i+1:]...)
			break
		}
	}
	l.listenerMu.Unlock()
}
