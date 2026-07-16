package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pi-sandbox/go-client/pkg/client"
)

func main() {
	// 命令行参数
	workDir := flag.String("dir", ".", "Working directory")
	model := flag.String("model", "", "Model to use (e.g., claude-sonnet-4-20250514)")
	provider := flag.String("provider", "anthropic", "Provider to use")
	thinking := flag.String("thinking", "medium", "Thinking level")
	noSession := flag.Bool("no-session", false, "Disable session persistence")
	flag.Parse()
	
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// 处理信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()
	
	// 构建参数
	args := []string{}
	if *noSession {
		args = append(args, "--no-session")
	}
	if *model != "" {
		args = append(args, "--model", *model)
	} else if *provider != "" {
		args = append(args, "--provider", *provider)
	}
	
	// 创建客户端
	piClient, err := client.NewPiClient(*workDir, args...)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer piClient.Close()
	
	// 设置事件处理器
	piClient.SetEventHandler(func(event client.Event) {
		switch event.Type {
		case client.EventMessageUpdate:
			// 流式输出文本
			if event.AssistantMessageEvent != nil {
				var delta struct {
					Type  string `json:"type"`
					Delta string `json:"delta"`
				}
				if err := json.Unmarshal(event.AssistantMessageEvent, &delta); err == nil {
					if delta.Type == "text_delta" {
						fmt.Print(delta.Delta)
					}
				}
			}
			
		case client.EventMessageEnd:
			fmt.Println() // 换行
			
		case client.EventToolExecutionStart:
			fmt.Printf("\n[Tool: %s]\n", event.ToolName)
			
		case client.EventToolExecutionEnd:
			if event.IsError {
				fmt.Printf("[Tool Error]\n")
			}
			
		case client.EventAgentEnd:
			fmt.Println("\n[Done]")
		}
	})
	
	// 连接
	if err := piClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// 设置思考级别
	if *thinking != "" {
		piClient.SetThinkingLevel(*thinking)
	}
	
	fmt.Println("Pi CLI Client")
	fmt.Println("Type your message and press Enter. Type 'quit' to exit.")
	fmt.Println("Commands: /model, /thinking, /compact, /state, /messages")
	fmt.Println(strings.Repeat("-", 50))
	
	// 读取输入
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		
		if !scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		
		// 处理退出
		if input == "quit" || input == "exit" {
			break
		}
		
		// 处理命令
		if strings.HasPrefix(input, "/") {
			parts := strings.Fields(input)
			cmd := parts[0]
			
			switch cmd {
			case "/model":
				if len(parts) > 1 {
					piClient.SetModel(*provider, parts[1])
					fmt.Printf("Model set to: %s\n", parts[1])
				} else {
					fmt.Println("Usage: /model <model-id>")
				}
				continue
				
			case "/thinking":
				if len(parts) > 1 {
					piClient.SetThinkingLevel(parts[1])
					fmt.Printf("Thinking level set to: %s\n", parts[1])
				} else {
					fmt.Println("Usage: /thinking <off|minimal|low|medium|high>")
				}
				continue
				
			case "/compact":
				instructions := ""
				if len(parts) > 1 {
					instructions = strings.Join(parts[1:], " ")
				}
				piClient.Compact(instructions)
				fmt.Println("Compaction started...")
				continue
				
			case "/state":
				piClient.GetState()
				continue
				
			case "/messages":
				piClient.GetMessages()
				continue
				
			case "/help":
				fmt.Println("Commands:")
				fmt.Println("  /model <id>      - Set model")
				fmt.Println("  /thinking <level> - Set thinking level")
				fmt.Println("  /compact [instructions] - Compact context")
				fmt.Println("  /state           - Show state")
				fmt.Println("  /messages        - Show messages")
				fmt.Println("  /help            - Show this help")
				fmt.Println("  quit/exit        - Exit")
				continue
			}
		}
		
		// 发送提示
		if err := piClient.Prompt(input); err != nil {
			log.Printf("Failed to send prompt: %v", err)
		}
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
	
	fmt.Println("Goodbye!")
}
