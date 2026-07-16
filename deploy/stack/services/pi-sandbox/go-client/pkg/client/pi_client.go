package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

// Command 表示发送给 Pi 的 RPC 命令
type Command struct {
	Type    string      `json:"type"`
	ID      string      `json:"id,omitempty"`
	Message string      `json:"message,omitempty"`
	Images  []Image     `json:"images,omitempty"`
	
	// 可选字段
	Provider          string `json:"provider,omitempty"`
	ModelID           string `json:"modelId,omitempty"`
	Level             string `json:"level,omitempty"`
	Mode              string `json:"mode,omitempty"`
	Enabled           bool   `json:"enabled,omitempty"`
	CustomInstructions string `json:"customInstructions,omitempty"`
	SessionPath       string `json:"sessionPath,omitempty"`
	ParentSession     string `json:"parentSession,omitempty"`
	EntryID           string `json:"entryId,omitempty"`
	Name              string `json:"name,omitempty"`
	OutputPath        string `json:"outputPath,omitempty"`
	Command           string `json:"command,omitempty"`
	StreamingBehavior string `json:"streamingBehavior,omitempty"`
}

// Image 表示图片附件
type Image struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

// Response 表示 Pi 的响应
type Response struct {
	Type    string          `json:"type"`
	Command string          `json:"command"`
	Success bool            `json:"success"`
	ID      string          `json:"id,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// Event 表示 Pi 的事件
type EventType string

const (
	EventAgentStart          EventType = "agent_start"
	EventAgentEnd            EventType = "agent_end"
	EventTurnStart           EventType = "turn_start"
	EventTurnEnd             EventType = "turn_end"
	EventMessageStart        EventType = "message_start"
	EventMessageUpdate       EventType = "message_update"
	EventMessageEnd          EventType = "message_end"
	EventToolExecutionStart  EventType = "tool_execution_start"
	EventToolExecutionUpdate EventType = "tool_execution_update"
	EventToolExecutionEnd    EventType = "tool_execution_end"
	EventQueueUpdate         EventType = "queue_update"
	EventCompactionStart     EventType = "compaction_start"
	EventCompactionEnd       EventType = "compaction_end"
	EventAutoRetryStart      EventType = "auto_retry_start"
	EventAutoRetryEnd        EventType = "auto_retry_end"
	EventExtensionError      EventType = "extension_error"
	EventExtensionUIRequest  EventType = "extension_ui_request"
)

// Event 表示 Pi 的事件
type Event struct {
	Type                 EventType           `json:"type"`
	Messages             []json.RawMessage   `json:"messages,omitempty"`
	Message              json.RawMessage     `json:"message,omitempty"`
	ToolResults          []json.RawMessage   `json:"toolResults,omitempty"`
	ToolCallID           string              `json:"toolCallId,omitempty"`
	ToolName             string              `json:"toolName,omitempty"`
	Args                 json.RawMessage     `json:"args,omitempty"`
	Result               json.RawMessage     `json:"result,omitempty"`
	IsError              bool                `json:"isError,omitempty"`
	PartialResult        json.RawMessage     `json:"partialResult,omitempty"`
	AssistantMessageEvent json.RawMessage    `json:"assistantMessageEvent,omitempty"`
	Steering             []string            `json:"steering,omitempty"`
	FollowUp             []string            `json:"followUp,omitempty"`
	Reason               string              `json:"reason,omitempty"`
	Aborted              bool                `json:"aborted,omitempty"`
	WillRetry            bool                `json:"willRetry,omitempty"`
	ErrorMessage         string              `json:"errorMessage,omitempty"`
	Attempt              int                 `json:"attempt,omitempty"`
	MaxAttempts          int                 `json:"maxAttempts,omitempty"`
	DelayMs              int                 `json:"delayMs,omitempty"`
	Success              bool                `json:"success,omitempty"`
	FinalError           string              `json:"finalError,omitempty"`
	ExtensionPath        string              `json:"extensionPath,omitempty"`
	Event                string              `json:"event,omitempty"`
	Error                string              `json:"error,omitempty"`
	
	// Extension UI（文案与流式事件的 message 共用 JSON 键，由 Message json.RawMessage 承载）
	ID      string   `json:"id,omitempty"`
	Method  string   `json:"method,omitempty"`
	Title   string   `json:"title,omitempty"`
	Options []string `json:"options,omitempty"`
	Timeout int      `json:"timeout,omitempty"`
}

// PiClient 是 Pi RPC 客户端
type PiClient struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	events   chan Event
	errors   chan error
	done     chan struct{}
	closed   bool
	
	// 事件处理器
	onEvent func(Event)
	
	// Extension UI 处理器
	onExtensionUI func(Event) (interface{}, error)
}

// NewPiClient 创建新的 Pi 客户端
func NewPiClient(workDir string, args ...string) (*PiClient, error) {
	cmdArgs := []string{"--mode", "rpc"}
	cmdArgs = append(cmdArgs, args...)
	
	cmd := exec.Command("pi", cmdArgs...)
	cmd.Dir = workDir
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	client := &PiClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		events: make(chan Event, 100),
		errors: make(chan error, 10),
		done:   make(chan struct{}),
	}
	
	return client, nil
}

// Connect 启动 Pi 进程并开始读取事件
func (c *PiClient) Connect(ctx context.Context) error {
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pi: %w", err)
	}
	
	// 启动事件读取 goroutine
	go c.readEvents(ctx)
	go c.readStderr(ctx)
	
	// 等待进程退出
	go func() {
		err := c.cmd.Wait()
		if err != nil {
			c.errors <- fmt.Errorf("pi process exited with error: %w", err)
		}
		close(c.done)
	}()
	
	return nil
}

// readEvents 从 stdout 读取事件
func (c *PiClient) readEvents(ctx context.Context) {
	scanner := bufio.NewScanner(c.stdout)
	
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		
		// 尝试解析为响应或事件
		var base struct {
			Type string `json:"type"`
		}
		
		if err := json.Unmarshal(line, &base); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}
		
		if base.Type == "response" {
			var resp Response
			if err := json.Unmarshal(line, &resp); err != nil {
				log.Printf("Failed to parse response: %v", err)
			}
			// 响应可以通过其他方式处理
			continue
		}
		
		// 解析为事件
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			log.Printf("Failed to parse event: %v", err)
			continue
		}
		
		// 调用事件处理器
		if c.onEvent != nil {
			c.onEvent(event)
		}
		
		// 处理 Extension UI 请求
		if event.Type == EventExtensionUIRequest && c.onExtensionUI != nil {
			go c.handleExtensionUI(event)
		}
		
		// 发送到事件通道
		select {
		case c.events <- event:
		default:
			log.Println("Event channel full, dropping event")
		}
	}
	
	if err := scanner.Err(); err != nil {
		c.errors <- fmt.Errorf("scanner error: %w", err)
	}
}

// readStderr 读取 stderr 输出
func (c *PiClient) readStderr(ctx context.Context) {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		log.Printf("[Pi stderr] %s", scanner.Text())
	}
}

// handleExtensionUI 处理 Extension UI 请求
func (c *PiClient) handleExtensionUI(event Event) {
	result, err := c.onExtensionUI(event)
	
	resp := map[string]interface{}{
		"type": "extension_ui_response",
		"id":   event.ID,
	}
	
	if err != nil {
		resp["cancelled"] = true
	} else {
		switch v := result.(type) {
		case bool:
			resp["confirmed"] = v
		case string:
			resp["value"] = v
		default:
			resp["value"] = v
		}
	}
	
	c.Send(resp)
}

// Send 发送命令
func (c *PiClient) Send(cmd interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return fmt.Errorf("client is closed")
	}
	
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}
	
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return fmt.Errorf("failed to write command: %w", err)
	}
	
	return nil
}

// Prompt 发送提示消息
func (c *PiClient) Prompt(message string, images ...Image) error {
	cmd := Command{
		Type:    "prompt",
		Message: message,
		Images:  images,
	}
	return c.Send(cmd)
}

// PromptWithOptions 发送带选项的提示消息
func (c *PiClient) PromptWithOptions(message string, streamingBehavior string) error {
	cmd := Command{
		Type:              "prompt",
		Message:           message,
		StreamingBehavior: streamingBehavior,
	}
	return c.Send(cmd)
}

// Steer 发送引导消息
func (c *PiClient) Steer(message string) error {
	cmd := Command{
		Type:    "steer",
		Message: message,
	}
	return c.Send(cmd)
}

// FollowUp 发送后续消息
func (c *PiClient) FollowUp(message string) error {
	cmd := Command{
		Type:    "follow_up",
		Message: message,
	}
	return c.Send(cmd)
}

// Abort 中止当前操作
func (c *PiClient) Abort() error {
	return c.Send(Command{Type: "abort"})
}

// NewSession 创建新会话
func (c *PiClient) NewSession(parentSession string) error {
	cmd := Command{
		Type:          "new_session",
		ParentSession: parentSession,
	}
	return c.Send(cmd)
}

// GetState 获取当前状态
func (c *PiClient) GetState() error {
	return c.Send(Command{Type: "get_state"})
}

// GetMessages 获取所有消息
func (c *PiClient) GetMessages() error {
	return c.Send(Command{Type: "get_messages"})
}

// SetModel 设置模型
func (c *PiClient) SetModel(provider, modelID string) error {
	cmd := Command{
		Type:     "set_model",
		Provider: provider,
		ModelID:  modelID,
	}
	return c.Send(cmd)
}

// SetThinkingLevel 设置思考级别
func (c *PiClient) SetThinkingLevel(level string) error {
	cmd := Command{
		Type:  "set_thinking_level",
		Level: level,
	}
	return c.Send(cmd)
}

// Compact 压缩上下文
func (c *PiClient) Compact(customInstructions string) error {
	cmd := Command{
		Type:                "compact",
		CustomInstructions: customInstructions,
	}
	return c.Send(cmd)
}

// Bash 执行命令
func (c *PiClient) Bash(command string) error {
	cmd := Command{
		Type:    "bash",
		Command: command,
	}
	return c.Send(cmd)
}

// AbortBash 中止命令执行
func (c *PiClient) AbortBash() error {
	return c.Send(Command{Type: "abort_bash"})
}

// GetSessionStats 获取会话统计
func (c *PiClient) GetSessionStats() error {
	return c.Send(Command{Type: "get_session_stats"})
}

// ExportHTML 导出为 HTML
func (c *PiClient) ExportHTML(outputPath string) error {
	cmd := Command{
		Type:       "export_html",
		OutputPath: outputPath,
	}
	return c.Send(cmd)
}

// SwitchSession 切换会话
func (c *PiClient) SwitchSession(sessionPath string) error {
	cmd := Command{
		Type:        "switch_session",
		SessionPath: sessionPath,
	}
	return c.Send(cmd)
}

// Fork 分叉会话
func (c *PiClient) Fork(entryID string) error {
	cmd := Command{
		Type:    "fork",
		EntryID: entryID,
	}
	return c.Send(cmd)
}

// Clone 克隆会话
func (c *PiClient) Clone() error {
	return c.Send(Command{Type: "clone"})
}

// SetSessionName 设置会话名称
func (c *PiClient) SetSessionName(name string) error {
	cmd := Command{
		Type: "set_session_name",
		Name: name,
	}
	return c.Send(cmd)
}

// GetCommands 获取可用命令
func (c *PiClient) GetCommands() error {
	return c.Send(Command{Type: "get_commands"})
}

// SetAutoCompaction 设置自动压缩
func (c *PiClient) SetAutoCompaction(enabled bool) error {
	cmd := Command{
		Type:    "set_auto_compaction",
		Enabled: enabled,
	}
	return c.Send(cmd)
}

// SetAutoRetry 设置自动重试
func (c *PiClient) SetAutoRetry(enabled bool) error {
	cmd := Command{
		Type:    "set_auto_retry",
		Enabled: enabled,
	}
	return c.Send(cmd)
}

// AbortRetry 中止重试
func (c *PiClient) AbortRetry() error {
	return c.Send(Command{Type: "abort_retry"})
}

// SetSteeringMode 设置引导模式
func (c *PiClient) SetSteeringMode(mode string) error {
	cmd := Command{
		Type: "set_steering_mode",
		Mode: mode,
	}
	return c.Send(cmd)
}

// SetFollowUpMode 设置后续消息模式
func (c *PiClient) SetFollowUpMode(mode string) error {
	cmd := Command{
		Type: "set_follow_up_mode",
		Mode: mode,
	}
	return c.Send(cmd)
}

// Events 返回事件通道
func (c *PiClient) Events() <-chan Event {
	return c.events
}

// Errors 返回错误通道
func (c *PiClient) Errors() <-chan error {
	return c.errors
}

// SetEventHandler 设置事件处理器
func (c *PiClient) SetEventHandler(handler func(Event)) {
	c.onEvent = handler
}

// SetExtensionUIHandler 设置 Extension UI 处理器
func (c *PiClient) SetExtensionUIHandler(handler func(Event) (interface{}, error)) {
	c.onExtensionUI = handler
}

// Close 关闭客户端
func (c *PiClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return nil
	}
	
	c.closed = true
	
	// 发送退出命令
	c.Send(Command{Type: "quit"})
	
	// 等待进程退出或超时
	select {
	case <-c.done:
		// 正常退出
	case <-time.After(5 * time.Second):
		// 超时，强制终止
		if c.cmd.Process != nil {
			c.cmd.Process.Kill()
		}
	}
	
	close(c.events)
	close(c.errors)
	
	return nil
}

// Wait 等待进程退出
func (c *PiClient) Wait() <-chan struct{} {
	return c.done
}
