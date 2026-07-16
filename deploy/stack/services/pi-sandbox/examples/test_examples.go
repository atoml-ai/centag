//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pi-sandbox/go-client/pkg/client"
)

func main() {
	fmt.Println("=== Pi Client Test Examples ===\n")
	
	// 示例 1: 基本对话
	fmt.Println("Example 1: Basic Conversation")
	basicConversation()
	
	fmt.Println("\n" + "=".repeat(50) + "\n")
	
	// 示例 2: 流式输出
	fmt.Println("Example 2: Streaming Output")
	streamingExample()
	
	fmt.Println("\n" + "=".repeat(50) + "\n")
	
	// 示例 3: 多会话管理
	fmt.Println("Example 3: Multi-Session Management")
	multiSessionExample()
	
	fmt.Println("\n" + "=".repeat(50) + "\n")
	
	// 示例 4: 工作流执行
	fmt.Println("Example 4: Workflow Execution")
	workflowExample()
}

// 基本对话示例
func basicConversation() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	// 创建客户端
	piClient, err := client.NewPiClient(".", "--no-session")
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return
	}
	defer piClient.Close()
	
	// 连接
	if err := piClient.Connect(ctx); err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}
	
	// 设置事件处理器
	done := make(chan struct{})
	piClient.SetEventHandler(func(event client.Event) {
		switch event.Type {
		case client.EventMessageUpdate:
			// 这里简化处理，实际应该解析 JSON
			fmt.Print(".")
		case client.EventAgentEnd:
			fmt.Println("\nResponse received!")
			close(done)
		}
	})
	
	// 发送提示
	fmt.Println("Sending prompt: 'What is 2+2?'")
	if err := piClient.Prompt("What is 2+2?"); err != nil {
		log.Printf("Failed to send prompt: %v", err)
		return
	}
	
	// 等待完成
	select {
	case <-done:
		fmt.Println("Conversation completed successfully")
	case <-ctx.Done():
		fmt.Println("Timeout waiting for response")
	}
}

// 流式输出示例
func streamingExample() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	piClient, err := client.NewPiClient(".", "--no-session")
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return
	}
	defer piClient.Close()
	
	if err := piClient.Connect(ctx); err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}
	
	// 设置流式输出处理器
	piClient.SetEventHandler(func(event client.Event) {
		if event.Type == client.EventMessageUpdate && event.AssistantMessageEvent != nil {
			// 简化：直接打印事件类型
			fmt.Print("█")
		}
	})
	
	fmt.Println("Streaming response for: 'Count from 1 to 5'")
	if err := piClient.Prompt("Count from 1 to 5, one number per line"); err != nil {
		log.Printf("Failed to send prompt: %v", err)
		return
	}
	
	// 等待一段时间
	time.Sleep(10 * time.Second)
	fmt.Println("\nStreaming example completed")
}

// 多会话管理示例
func multiSessionExample() {
	config := client.DefaultManagerConfig()
	config.MaxSessions = 3
	
	manager := client.NewAgentManager(".", config)
	defer manager.CloseAll()
	
	// 创建多个会话
	sessionIDs := []string{"session-1", "session-2", "session-3"}
	
	for _, id := range sessionIDs {
		session, err := manager.CreateSession(id, "--no-session")
		if err != nil {
			log.Printf("Failed to create session %s: %v", id, err)
			continue
		}
		fmt.Printf("Created session: %s\n", session.ID)
	}
	
	// 列出会话
	sessions := manager.ListSessions()
	fmt.Printf("\nActive sessions: %d\n", len(sessions))
	for _, s := range sessions {
		fmt.Printf("  - %s (created: %s)\n", s.ID, s.CreatedAt.Format(time.RFC3339))
	}
	
	// 关闭一个会话
	fmt.Println("\nClosing session-2...")
	manager.CloseSession("session-2")
	
	// 再次列出
	sessions = manager.ListSessions()
	fmt.Printf("Active sessions after close: %d\n", len(sessions))
}

// 工作流执行示例
func workflowExample() {
	config := client.DefaultManagerConfig()
	manager := client.NewAgentManager(".", config)
	defer manager.CloseAll()
	
	// 创建会话
	session, err := manager.CreateSession("workflow-session", "--no-session")
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		return
	}
	
	// 定义工作流
	workflow := &client.Workflow{
		Name:        "Code Review",
		Description: "Review code for issues",
		Steps: []client.WorkflowStep{
			{
				Name:     "List Files",
				Prompt:   "List all .go files in the current directory",
				WaitNext: true,
				Timeout:  30,
			},
			{
				Name:     "Analyze Code",
				Prompt:   "Analyze the code structure and identify potential issues",
				WaitNext: true,
				Timeout:  60,
			},
			{
				Name:     "Generate Report",
				Prompt:   "Generate a summary report of findings",
				WaitNext: true,
				Timeout:  30,
			},
		},
	}
	
	fmt.Printf("Executing workflow: %s\n", workflow.Name)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	if err := manager.ExecuteWorkflow(ctx, workflow, session.ID); err != nil {
		log.Printf("Workflow failed: %v", err)
		return
	}
	
	fmt.Println("Workflow completed successfully!")
}

// 辅助函数：重复字符串
func repeat(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
