package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultAgentEngine 默认Agent引擎实现
type DefaultAgentEngine struct {
	// httpClient HTTP客户端
	httpClient *http.Client

	// proxyURL Centag服务地址
	proxyURL string

	// tools 可用工具列表
	tools []ToolDefinition

	// toolExecutor 工具执行器（由外部注入）
	toolExecutor AgentToolExecutor

	// mu 互斥锁
	mu sync.RWMutex

	// requests 正在执行的请求
	requests map[string]context.CancelFunc
}

// NewDefaultAgentEngine 创建默认Agent引擎
func NewDefaultAgentEngine(proxyURL string) *DefaultAgentEngine {
	return &DefaultAgentEngine{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		proxyURL: proxyURL,
		tools:    make([]ToolDefinition, 0),
		requests: make(map[string]context.CancelFunc),
	}
}

// SetToolExecutor 设置工具执行器
func (e *DefaultAgentEngine) SetToolExecutor(executor AgentToolExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.toolExecutor = executor
}

// Run 执行Agent循环
func (e *DefaultAgentEngine) Run(ctx context.Context, req *AgentRequest) (<-chan AgentEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// 创建事件通道
	eventChan := make(chan AgentEvent, 100)

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(ctx)

	// 注册请求
	e.mu.Lock()
	e.requests[req.RequestID] = cancel
	e.mu.Unlock()

	// 发送开始事件
	e.sendEvent(eventChan, AgentEvent{
		Type:      EventAgentStart,
		RequestID: req.RequestID,
		Timestamp: time.Now(),
	})

	// 异步执行Agent循环
	go func() {
		defer close(eventChan)
		defer func() {
			e.mu.Lock()
			delete(e.requests, req.RequestID)
			e.mu.Unlock()
		}()

		e.runAgentLoop(ctx, req, eventChan)
	}()

	return eventChan, nil
}

// Cancel 取消执行
func (e *DefaultAgentEngine) Cancel(requestID string) error {
	e.mu.RLock()
	cancel, exists := e.requests[requestID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("request not found: %s", requestID)
	}

	cancel()
	return nil
}

// GetTools 获取可用工具列表
func (e *DefaultAgentEngine) GetTools() []ToolDefinition {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tools := make([]ToolDefinition, len(e.tools))
	copy(tools, e.tools)
	return tools
}

// RegisterTool 动态注册工具
func (e *DefaultAgentEngine) RegisterTool(tool ToolDefinition) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查是否已存在
	for _, t := range e.tools {
		if t.Name == tool.Name {
			return fmt.Errorf("tool already registered: %s", tool.Name)
		}
	}

	e.tools = append(e.tools, tool)
	return nil
}

// UnregisterTool 注销工具
func (e *DefaultAgentEngine) UnregisterTool(toolName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, t := range e.tools {
		if t.Name == toolName {
			e.tools = append(e.tools[:i], e.tools[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("tool not found: %s", toolName)
}

// runAgentLoop 运行Agent循环
func (e *DefaultAgentEngine) runAgentLoop(ctx context.Context, req *AgentRequest, eventChan chan<- AgentEvent) {
	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 20 // 默认最大20轮
	}

	messages := make([]Message, len(req.Messages))
	copy(messages, req.Messages)

	for turn := 0; turn < maxTurns; turn++ {
		// 检查上下文是否已取消
		if ctx.Err() != nil {
			e.sendEvent(eventChan, AgentEvent{
				Type:      EventError,
				RequestID: req.RequestID,
				Timestamp: time.Now(),
				Data: AgentEventData{
					Error: &AgentError{
						Code:    "CANCELLED",
						Message: "request cancelled",
					},
				},
			})
			return
		}

		// 发送进度事件
		e.sendEvent(eventChan, AgentEvent{
			Type:      EventProgress,
			RequestID: req.RequestID,
			Timestamp: time.Now(),
			Data: AgentEventData{
				Progress: &AgentProgressInfo{
					CurrentTurn: turn + 1,
					MaxTurns:    maxTurns,
					Status:      "calling LLM",
				},
			},
		})

		// 调用LLM
		llmResponse, err := e.callLLM(ctx, req, messages)
		if err != nil {
			e.sendEvent(eventChan, AgentEvent{
				Type:      EventError,
				RequestID: req.RequestID,
				Timestamp: time.Now(),
				Data: AgentEventData{
					Error: &AgentError{
						Code:    "LLM_ERROR",
						Message: err.Error(),
					},
				},
			})
			return
		}

		// 处理LLM响应
		if llmResponse.Choices == nil || len(llmResponse.Choices) == 0 {
			e.sendEvent(eventChan, AgentEvent{
				Type:      EventError,
				RequestID: req.RequestID,
				Timestamp: time.Now(),
				Data: AgentEventData{
					Error: &AgentError{
						Code:    "EMPTY_RESPONSE",
						Message: "LLM returned empty response",
					},
				},
			})
			return
		}

		choice := llmResponse.Choices[0]
		message := choice.Message

		// 添加assistant消息到历史
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   message.Content,
			ToolCalls: message.ToolCalls,
		})

		// 发送消息更新事件
		if message.Content != "" {
			e.sendEvent(eventChan, AgentEvent{
				Type:      EventMessageUpdate,
				RequestID: req.RequestID,
				Timestamp: time.Now(),
				Data: AgentEventData{
					Message: &Message{
						Role:    "assistant",
						Content: message.Content,
					},
				},
			})
		}

		// 检查是否有工具调用
		if len(message.ToolCalls) == 0 {
			// 没有工具调用，Agent循环结束
			e.sendEvent(eventChan, AgentEvent{
				Type:      EventAgentEnd,
				RequestID: req.RequestID,
				Timestamp: time.Now(),
			})
			return
		}

		// 处理工具调用
		for _, toolCall := range message.ToolCalls {
			e.sendEvent(eventChan, AgentEvent{
				Type:      EventToolStart,
				RequestID: req.RequestID,
				Timestamp: time.Now(),
				Data: AgentEventData{
					ToolCall: &toolCall,
				},
			})

			if e.toolExecutor == nil {
				e.sendEvent(eventChan, AgentEvent{
					Type:      EventError,
					RequestID: req.RequestID,
					Timestamp: time.Now(),
					Data: AgentEventData{
						Error: &AgentError{
							Code:    "NO_TOOL_EXECUTOR",
							Message: fmt.Sprintf("no tool executor configured, cannot execute tool: %s", toolCall.Function.Name),
						},
					},
				})
				return
			}

			content, isError, execErr := e.toolExecutor.Execute(ctx, toolCall.ID, toolCall.Function.Name, toolCall.Function.Arguments)
			if execErr != nil {
				e.sendEvent(eventChan, AgentEvent{
					Type:      EventError,
					RequestID: req.RequestID,
					Timestamp: time.Now(),
					Data: AgentEventData{
						Error: &AgentError{
							Code:    "TOOL_EXECUTION_ERROR",
							Message: fmt.Sprintf("tool %s execution failed: %v", toolCall.Function.Name, execErr),
						},
					},
				})
				return
			}

			toolResult := ToolResult{
				ToolCallID: toolCall.ID,
				Content:    content,
				IsError:    isError,
			}

			e.sendEvent(eventChan, AgentEvent{
				Type:      EventToolEnd,
				RequestID: req.RequestID,
				Timestamp: time.Now(),
				Data: AgentEventData{
					ToolCall:   &toolCall,
					ToolResult: &toolResult,
				},
			})

			messages = append(messages, Message{
				Role:       "tool",
				ToolCallID: toolCall.ID,
				Name:       toolCall.Function.Name,
				Content:    content,
			})
		}
	}
}

// callLLM 调用LLM
func (e *DefaultAgentEngine) callLLM(ctx context.Context, req *AgentRequest, messages []Message) (*LLMResponse, error) {
	// 构建请求
	llmReq := &LLMRequest{
		Model:    req.Model,
		Messages: messages,
		Tools:    e.convertToolsToLLMFormat(req.Tools),
	}

	if req.ToolChoice != "" {
		llmReq.ToolChoice = req.ToolChoice
	}

	// 序列化请求
	reqBody, err := json.Marshal(llmReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 构建HTTP请求
	url := fmt.Sprintf("%s/v1/chat/completions", e.proxyURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var llmResp LLMResponse
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &llmResp, nil
}

// convertToolsToLLMFormat 转换工具格式为LLM格式
func (e *DefaultAgentEngine) convertToolsToLLMFormat(tools []ToolDefinition) []LLMTool {
	if len(tools) == 0 {
		return nil
	}

	llmTools := make([]LLMTool, len(tools))
	for i, tool := range tools {
		llmTools[i] = LLMTool{
			Type: "function",
			Function: LLMFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
	}
	return llmTools
}

// sendEvent 发送事件
func (e *DefaultAgentEngine) sendEvent(eventChan chan<- AgentEvent, event AgentEvent) {
	select {
	case eventChan <- event:
	default:
		// 通道已满，丢弃事件
	}
}

// LLMRequest LLM请求
type LLMRequest struct {
	Model     string      `json:"model"`
	Messages  []Message   `json:"messages"`
	Tools     []LLMTool   `json:"tools,omitempty"`
	ToolChoice interface{} `json:"tool_choice,omitempty"`
}

// LLMResponse LLM响应
type LLMResponse struct {
	Choices []LLMChoice `json:"choices"`
}

// LLMChoice LLM响应选项
type LLMChoice struct {
	Message LLMMessage `json:"message"`
}

// LLMMessage LLM消息
type LLMMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content,omitempty"`
	ToolCalls []ToolCallInfo `json:"tool_calls,omitempty"`
}

// LLMTool LLM工具定义
type LLMTool struct {
	Type     string       `json:"type"`
	Function LLMFunction `json:"function"`
}

// LLMFunction LLM函数定义
type LLMFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}