package pipeline

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const defaultRemotePluginMaxBody = 4 << 20 // 4 MiB

type RemoteNodePlugin struct {
	baseURL    string
	httpClient *http.Client
	descriptor NodePluginDescriptor
	// streamSupported 标记该插件是否支持 /stream 端点
	streamSupported bool
	// 熔断相关字段 - 使用原子操作和互斥锁保护并发安全
	failureCount int32     // 原子操作保护
	lastFailure  time.Time // 受 mu 保护
	circuitOpen  int32     // 原子操作保护：0=关闭, 1=打开
	// mu 保护 lastFailure 和其他非原子字段的并发访问
	mu sync.RWMutex
	// 健康状态 - 受 mu 保护
	healthStatus       string             // "healthy", "unhealthy", "unknown" - 受 mu 保护
	lastHealthCheck    time.Time          // 受 mu 保护
	healthCheckCancel  context.CancelFunc // 健康检查取消函数 - 受 mu 保护
	healthCheckRunning int32              // 原子操作：0=停止, 1=运行中
	// 并发控制
	semaphore chan struct{}
	// 哈希锁定配置
	hashConfig ManifestHashConfig
}

func NewRemoteNodePlugin(baseURL string) NodePlugin {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &RemoteNodePlugin{
		baseURL: normalized,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		semaphore:    make(chan struct{}, 10), // 默认最大并发10
		healthStatus: "unknown",
		descriptor: NodePluginDescriptor{
			Name:                normalized,
			Implementation:      normalized,
			Kind:                "remote.node",
			Version:             "unknown",
			Description:         "远程流水线节点插件",
			Permissions:         []string{"network.outbound"},
			SupportsStream:      false,
			Concurrent:          true,
			APIVersion:          PipelinePluginSchemaVersion,
			MinProxyclawVersion: PipelinePluginSchemaVersion,
			Remote: &RemoteNodePluginSpec{
				BaseURL:     normalized,
				ManifestURL: normalized + "/.well-known/centag-node-plugin.json",
			},
		},
	}
}

func (p *RemoteNodePlugin) Descriptor() NodePluginDescriptor {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if descriptor, err := p.fetchDescriptor(ctx); err == nil {
		p.mu.Lock()
		p.descriptor = descriptor
		p.streamSupported = descriptor.SupportsStream
		p.mu.Unlock()
	}
	p.mu.RLock()
	desc := p.descriptor
	p.mu.RUnlock()
	return desc
}

func (p *RemoteNodePlugin) ValidateConfig(config NodeConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.httpClient.Timeout)
	defer cancel()
	reqBody := map[string]interface{}{
		"schema_version": PipelinePluginSchemaVersion,
		"config":         config,
	}
	resp, err := p.postJSON(ctx, "/validate", reqBody)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultRemotePluginMaxBody))
	if err != nil {
		return fmt.Errorf("remote plugin validate: read body failed: %w", err)
	}
	if resp.StatusCode >= 500 {
		var vr NodeValidateResponse
		if parseErr := json.Unmarshal(body, &vr); parseErr == nil && vr.Message != "" {
			return fmt.Errorf("%s: %s", vr.Code, vr.Message)
		}
		return fmt.Errorf("remote plugin validate failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	if resp.StatusCode >= 400 {
		return nil
	}
	var vr NodeValidateResponse
	if err := json.Unmarshal(body, &vr); err != nil {
		return nil
	}
	if vr.Valid {
		return nil
	}
	if len(vr.Errors) > 0 {
		errMsgs := make([]string, 0, len(vr.Errors))
		for _, e := range vr.Errors {
			if e.Code != "" && e.Message != "" {
				errMsgs = append(errMsgs, e.Code+": "+e.Message)
			} else if e.Message != "" {
				errMsgs = append(errMsgs, e.Message)
			}
		}
		if len(errMsgs) > 0 {
			return fmt.Errorf("validation failed: %s", strings.Join(errMsgs, "; "))
		}
	}
	if vr.Message != "" {
		return fmt.Errorf("%s: %s", vr.Code, vr.Message)
	}
	return nil
}

func (p *RemoteNodePlugin) Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("remote node execution request cannot be nil")
	}

	// 检查熔断状态 - 使用原子操作读取
	if atomic.LoadInt32(&p.circuitOpen) == 1 {
		p.mu.RLock()
		lastFailure := p.lastFailure
		p.mu.RUnlock()

		if time.Since(lastFailure) > 30*time.Second {
			// 冷却时间已过，尝试恢复
			atomic.StoreInt32(&p.circuitOpen, 0)
			atomic.StoreInt32(&p.failureCount, 0)
		} else {
			return nil, fmt.Errorf("circuit breaker open, plugin temporarily unavailable")
		}
	}

	// 并发限制：获取信号量
	p.semaphore <- struct{}{}
	defer func() { <-p.semaphore }()

	// 如果插件支持流式，调用 /stream 端点
	p.mu.RLock()
	streamSupported := p.streamSupported
	p.mu.RUnlock()
	if streamSupported {
		return p.executeStream(ctx, req)
	}

	resp, err := p.postJSON(ctx, "/execute", req)
	if err != nil {
		p.recordFailure()
		return nil, err
	}
	defer resp.Body.Close()
	body := io.LimitReader(resp.Body, defaultRemotePluginMaxBody)
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(body)
		p.recordFailure()
		return nil, fmt.Errorf("remote plugin execute failed: status=%d body=%s", resp.StatusCode, string(data))
	}

	var pluginResp NodeExecutionResponse
	if err := json.NewDecoder(body).Decode(&pluginResp); err != nil {
		p.recordFailure()
		return nil, fmt.Errorf("decode remote plugin response failed: %w", err)
	}
	if pluginResp.Output == nil {
		p.recordFailure()
		return nil, fmt.Errorf("remote plugin returned empty output")
	}

	// 成功执行，重置失败计数
	p.resetFailure()
	return &pluginResp, nil
}

// recordFailure 记录失败 - 使用原子操作保证并发安全
func (p *RemoteNodePlugin) recordFailure() {
	newCount := atomic.AddInt32(&p.failureCount, 1)
	p.mu.Lock()
	p.lastFailure = time.Now()
	p.mu.Unlock()

	if newCount >= 5 {
		atomic.StoreInt32(&p.circuitOpen, 1)
		p.mu.Lock()
		pluginName := p.descriptor.Name
		if p.descriptor.Implementation != "" {
			pluginName = p.descriptor.Implementation
		}
		p.mu.Unlock()
		log.Printf("[CIRCUIT_BREAKER] Circuit opened for plugin %s due to %d consecutive failures", pluginName, newCount)
		p.updateHealthStatus("unhealthy")
	}
}

// resetFailure 重置失败计数 - 使用原子操作保证并发安全
func (p *RemoteNodePlugin) resetFailure() {
	oldCount := atomic.LoadInt32(&p.failureCount)
	if oldCount > 0 {
		atomic.StoreInt32(&p.failureCount, 0)
		wasOpen := atomic.SwapInt32(&p.circuitOpen, 0) == 1
		if wasOpen {
			p.mu.Lock()
			pluginName := p.descriptor.Name
			if p.descriptor.Implementation != "" {
				pluginName = p.descriptor.Implementation
			}
			p.mu.Unlock()
			log.Printf("[CIRCUIT_BREAKER] Circuit closed for plugin %s after successful recovery", pluginName)
		}
		p.updateHealthStatus("healthy")
	}
}

// executeStream 处理流式响应（SSE）
func (p *RemoteNodePlugin) executeStream(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error) {
	resp, err := p.postJSON(ctx, "/stream", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body := io.LimitReader(resp.Body, defaultRemotePluginMaxBody)
		data, _ := io.ReadAll(body)
		return nil, fmt.Errorf("remote plugin stream failed: status=%d body=%s", resp.StatusCode, string(data))
	}

	// 解析 SSE 流
	var finalOutput *NodeOutput
	var finalErr error
	eventChan := make(chan SSEEvent)
	errorChan := make(chan error, 1)

	// 启动 SSE 解析协程
	go func() {
		defer close(eventChan)
		scanner := bufio.NewScanner(resp.Body)
		// 设置更大的缓冲区以处理长行
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		var currentEvent SSEEvent
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				// 空行表示事件结束
				if currentEvent.Event != "" || currentEvent.Data != "" {
					eventChan <- currentEvent
					currentEvent = SSEEvent{}
				}
				continue
			}

			// 解析 SSE 行
			if strings.HasPrefix(line, "event:") {
				currentEvent.Event = strings.TrimSpace(line[6:])
			} else if strings.HasPrefix(line, "data:") {
				currentEvent.Data = strings.TrimSpace(line[5:])
			} else if strings.HasPrefix(line, "id:") {
				currentEvent.ID = strings.TrimSpace(line[3:])
			} else if strings.HasPrefix(line, "retry:") {
				_ = strings.TrimSpace(line[5:]) // 忽略 retry 字段
			}
		}

		if err := scanner.Err(); err != nil {
			errorChan <- err
		}
	}()

	// 收集最终结果
	var fullContent strings.Builder
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// 流结束
				goto DONE
			}

			// 处理事件
			switch event.Event {
			case "message", "":
				// 解析数据
				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(event.Data), &chunk); err == nil {
					if content, ok := chunk["content"].(string); ok {
						fullContent.WriteString(content)
					}
				}
			case "done":
				// 流完成
				goto DONE
			case "error":
				var errData map[string]interface{}
				if err := json.Unmarshal([]byte(event.Data), &errData); err == nil {
					if msg, ok := errData["message"].(string); ok {
						finalErr = fmt.Errorf("stream error: %s", msg)
					}
				}
				goto DONE
			}
		case err := <-errorChan:
			finalErr = err
			goto DONE
		case <-ctx.Done():
			finalErr = ctx.Err()
			goto DONE
		}
	}

DONE:
	if finalErr != nil {
		return nil, finalErr
	}

	finalOutput = &NodeOutput{
		Content: fullContent.String(),
		Metadata: map[string]interface{}{
			"streaming": true,
		},
	}

	return &NodeExecutionResponse{
		Output: finalOutput,
	}, nil
}

// SSEEvent SSE 事件
type SSEEvent struct {
	Event string
	Data  string
	ID    string
}

// verifyManifestSignature 验证 manifest 的签名或哈希
// 支持两种方式：
// 1. 哈希锁定：计算 manifest 的 SHA-256 哈希，与期望的哈希比较
// 2. 签名验证：使用公钥验证 manifest 的签名（未来扩展）
func (p *RemoteNodePlugin) verifyManifestSignature(expectedHash string) error {
	if expectedHash == "" {
		// 未配置哈希，跳过验证
		return nil
	}

	// 获取 manifest
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/.well-known/centag-node-plugin.json", nil)
	if err != nil {
		return fmt.Errorf("create manifest request failed: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch manifest failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("manifest returned status %d", resp.StatusCode)
	}

	// 读取 manifest 内容
	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultRemotePluginMaxBody))
	if err != nil {
		return fmt.Errorf("read manifest failed: %w", err)
	}

	// 计算 SHA-256 哈希
	hash := sha256.Sum256(body)
	actualHash := hex.EncodeToString(hash[:])

	// 比较哈希（不区分大小写）
	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("manifest hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// startHealthCheck 启动定期健康检查 - 内部方法，支持 context 取消
func (p *RemoteNodePlugin) startHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	p.performHealthCheck()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.performHealthCheck()
		}
	}
}

// StartHealthCheck 启动定期健康检查协程 - 公开方法
func (p *RemoteNodePlugin) StartHealthCheck() {
	if !atomic.CompareAndSwapInt32(&p.healthCheckRunning, 0, 1) {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.healthCheckCancel = cancel
	p.mu.Unlock()

	go p.startHealthCheck(ctx)
}

// StopHealthCheck 停止健康检查协程 - 公开方法
func (p *RemoteNodePlugin) StopHealthCheck() {
	if !atomic.CompareAndSwapInt32(&p.healthCheckRunning, 1, 0) {
		return
	}

	p.mu.Lock()
	if p.healthCheckCancel != nil {
		p.healthCheckCancel()
		p.healthCheckCancel = nil
	}
	p.mu.Unlock()
}

// IsHealthCheckRunning 检查健康检查是否正在运行
func (p *RemoteNodePlugin) IsHealthCheckRunning() bool {
	return atomic.LoadInt32(&p.healthCheckRunning) == 1
}

// performHealthCheck 执行健康检查
func (p *RemoteNodePlugin) performHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 尝试访问健康检查端点，如果不存在则检查manifest端点
	healthURL := p.baseURL + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		p.updateHealthStatus("unhealthy")
		return
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		// 如果/health不存在，尝试检查manifest端点作为兜底
		p.checkManifestHealth()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		p.updateHealthStatus("healthy")
	} else {
		p.updateHealthStatus("unhealthy")
	}
}

// checkManifestHealth 通过检查manifest端点来判断健康状态
func (p *RemoteNodePlugin) checkManifestHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/.well-known/centag-node-plugin.json", nil)
	if err != nil {
		p.updateHealthStatus("unhealthy")
		return
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.updateHealthStatus("unhealthy")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		p.updateHealthStatus("healthy")
	} else {
		p.updateHealthStatus("unhealthy")
	}
}

// updateHealthStatus 更新健康状态并记录指标
func (p *RemoteNodePlugin) updateHealthStatus(status string) {
	p.mu.Lock()
	oldStatus := p.healthStatus
	p.healthStatus = status
	p.lastHealthCheck = time.Now()
	descriptorImpl := p.descriptor.Implementation
	p.mu.Unlock()

	if GlobalPluginMetrics != nil {
		GlobalPluginMetrics.RecordHealthCheck(descriptorImpl, status, oldStatus)
	}
}

// GetHealthStatus 获取当前健康状态
func (p *RemoteNodePlugin) GetHealthStatus() (status string, lastCheck time.Time) {
	p.mu.RLock()
	status = p.healthStatus
	lastCheck = p.lastHealthCheck
	p.mu.RUnlock()
	return status, lastCheck
}

// IsCircuitOpen 获取熔断状态 - 使用原子操作
func (p *RemoteNodePlugin) IsCircuitOpen() bool {
	return atomic.LoadInt32(&p.circuitOpen) == 1
}

// GetFailureCount 获取失败次数 - 使用原子操作
func (p *RemoteNodePlugin) GetFailureCount() int {
	return int(atomic.LoadInt32(&p.failureCount))
}

func (p *RemoteNodePlugin) setLastFailureForTest(ts time.Time) {
	p.mu.Lock()
	p.lastFailure = ts
	p.mu.Unlock()
}

// ManifestHashConfig manifest 哈希配置
type ManifestHashConfig struct {
	Enabled    bool   `json:"enabled"`
	Hash       string `json:"hash"`        // 期望的 SHA-256 哈希值
	AutoUpdate bool   `json:"auto_update"` // 是否自动更新期望哈希
}

func (p *RemoteNodePlugin) fetchDescriptor(ctx context.Context) (NodePluginDescriptor, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/.well-known/centag-node-plugin.json", nil)
	if err != nil {
		return NodePluginDescriptor{}, err
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return NodePluginDescriptor{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return NodePluginDescriptor{}, fmt.Errorf("manifest returned status %d", resp.StatusCode)
	}
	var descriptor NodePluginDescriptor
	if err := json.NewDecoder(io.LimitReader(resp.Body, defaultRemotePluginMaxBody)).Decode(&descriptor); err != nil {
		return NodePluginDescriptor{}, err
	}

	// 校验必需字段
	if err := validateManifest(&descriptor); err != nil {
		return NodePluginDescriptor{}, fmt.Errorf("invalid manifest: %w", err)
	}

	// 验证 manifest 哈希（如果配置了）
	if descriptor.ExpectedHash != "" {
		if err := p.verifyManifestSignature(descriptor.ExpectedHash); err != nil {
			return NodePluginDescriptor{}, fmt.Errorf("manifest hash verification failed: %w", err)
		}
	}

	if descriptor.Implementation == "" {
		descriptor.Implementation = p.baseURL
	}
	if descriptor.Remote == nil {
		descriptor.Remote = &RemoteNodePluginSpec{
			BaseURL:     p.baseURL,
			ManifestURL: p.baseURL + "/.well-known/centag-node-plugin.json",
		}
	}
	return descriptor, nil
}

// validateManifest 校验 manifest 必需字段
func validateManifest(descriptor *NodePluginDescriptor) error {
	if descriptor == nil {
		return fmt.Errorf("descriptor is nil")
	}
	if descriptor.Implementation == "" {
		return fmt.Errorf("missing required field: implementation")
	}
	if descriptor.Kind == "" {
		return fmt.Errorf("missing required field: kind")
	}
	if descriptor.Version == "" {
		return fmt.Errorf("missing required field: version")
	}
	return nil
}

func (p *RemoteNodePlugin) postJSON(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	return p.httpClient.Do(httpReq)
}
