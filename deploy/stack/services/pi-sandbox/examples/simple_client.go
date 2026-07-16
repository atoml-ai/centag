//go:build ignore

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// 简化的 RPC 客户端示例
func main() {
	fmt.Println("=== Pi RPC 客户端示例 ===")
	fmt.Println()
	
	// 检查 API 密钥
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	
	if apiKey == "" {
		log.Println("警告: 未设置 API 密钥，某些功能可能无法使用")
	}
	
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// 处理信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n正在退出...")
		cancel()
	}()
	
	// 示例 1: 基本 RPC 调用
	fmt.Println("示例 1: 基本 RPC 调用")
	fmt.Println(strings.Repeat("-", 40))
	
	// 这里我们模拟 RPC 调用
	// 实际使用时，需要启动 Pi 容器并通过 stdin/stdout 通信
	
	fmt.Println("发送命令: {\"type\": \"prompt\", \"message\": \"Hello\"}")
	fmt.Println("预期响应: {\"type\": \"response\", \"command\": \"prompt\", \"success\": true}")
	fmt.Println()
	
	// 示例 2: 流式输出处理
	fmt.Println("示例 2: 流式输出处理")
	fmt.Println(strings.Repeat("-", 40))
	
	events := []map[string]interface{}{
		{"type": "agent_start"},
		{"type": "message_start", "message": map[string]interface{}{"role": "assistant"}},
		{"type": "message_update", "assistantMessageEvent": map[string]interface{}{
			"type": "text_delta", "delta": "Hello",
		}},
		{"type": "message_update", "assistantMessageEvent": map[string]interface{}{
			"type": "text_delta", "delta": " world!",
		}},
		{"type": "message_end", "message": map[string]interface{}{"role": "assistant"}},
		{"type": "agent_end"},
	}
	
	fmt.Println("模拟流式输出:")
	for _, event := range events {
		eventJSON, _ := json.MarshalIndent(event, "", "  ")
		fmt.Printf("事件: %s\n", event["type"])
		
		if event["type"] == "message_update" {
			if assistantEvent, ok := event["assistantMessageEvent"].(map[string]interface{}); ok {
				if delta, ok := assistantEvent["delta"].(string); ok {
					fmt.Printf("  文本: %s", delta)
				}
			}
		}
		
		_ = eventJSON
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("\n")
	
	// 示例 3: 安全检查
	fmt.Println("示例 3: 安全检查")
	fmt.Println(strings.Repeat("-", 40))
	
	testCommands := []struct {
		command string
		safe    bool
	}{
		{"ls -la", true},
		{"cat file.txt", true},
		{"rm -rf /", false},
		{"sudo apt-get update", false},
		{"chmod 777 file", false},
	}
	
	for _, test := range testCommands {
		status := "✅ 安全"
		if !test.safe {
			status = "❌ 危险"
		}
		fmt.Printf("命令: %-25s %s\n", test.command, status)
	}
	fmt.Println()
	
	// 示例 4: 会话管理
	fmt.Println("示例 4: 会话管理")
	fmt.Println(strings.Repeat("-", 40))
	
	sessions := []map[string]interface{}{
		{"id": "session-1", "created": time.Now().Add(-10 * time.Minute), "status": "active"},
		{"id": "session-2", "created": time.Now().Add(-5 * time.Minute), "status": "active"},
		{"id": "session-3", "created": time.Now(), "status": "idle"},
	}
	
	fmt.Println("活动会话:")
	for _, session := range sessions {
		fmt.Printf("  - %s (创建: %s, 状态: %s)\n",
			session["id"],
			session["created"].(time.Time).Format("15:04:05"),
			session["status"])
	}
	fmt.Println()
	
	// 示例 5: 交互式命令行
	fmt.Println("示例 5: 交互式命令行 (输入 'quit' 退出)")
	fmt.Println(strings.Repeat("-", 40))
	
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		
		if input == "quit" || input == "exit" {
			break
		}
		
		if input == "" {
			fmt.Print("> ")
			continue
		}
		
		// 处理命令
		if strings.HasPrefix(input, "/") {
			parts := strings.Fields(input)
			cmd := parts[0]
			
			switch cmd {
			case "/help":
				fmt.Println("可用命令:")
				fmt.Println("  /help    - 显示帮助")
				fmt.Println("  /status  - 显示状态")
				fmt.Println("  /sessions - 列出会话")
				fmt.Println("  quit     - 退出")
				
			case "/status":
				fmt.Println("状态: 运行中")
				fmt.Println("会话数:", len(sessions))
				
			case "/sessions":
				fmt.Println("活动会话:")
				for _, session := range sessions {
					fmt.Printf("  - %s\n", session["id"])
				}
				
			default:
				fmt.Printf("未知命令: %s (输入 /help 查看帮助)\n", cmd)
			}
		} else {
			// 模拟发送提示
			fmt.Printf("发送提示: %s\n", input)
			fmt.Println("响应: 这是一个模拟响应。在实际使用中，这里会显示 Pi 的真实响应。")
		}
		
		fmt.Print("> ")
	}
	
	fmt.Println("\n再见!")
}
