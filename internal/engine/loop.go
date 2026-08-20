package engine

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/aoverb/go-tiny-claw/internal/provider"
	"github.com/aoverb/go-tiny-claw/internal/schema"
	"github.com/aoverb/go-tiny-claw/internal/tools"
)

type AgentEngine struct {
	provider provider.LLMProvider
	registry tools.Registry

	workDir        string
	EnableThinking bool
}

func NewAgentEngine(p provider.LLMProvider, r tools.Registry, workDir string, enableThinking bool) *AgentEngine {
	return &AgentEngine{
		provider:       p,
		registry:       r,
		workDir:        workDir,
		EnableThinking: enableThinking,
	}
}

func (e *AgentEngine) Run(ctx context.Context, userPrompt string) error {
	log.Printf("[Engine] 引擎启动，工作区：%s\n", e.workDir)
	log.Printf("[Engine] 慢思考模式：%v\n", e.EnableThinking)
	contextHistory := []schema.Message{
		{
			Role:    schema.RoleSystem,
			Content: "You are go-tiny-claw, an expert coding assistant. You have full accesss to tools in the workspace.",
		},
		{
			Role:    schema.RoleUser,
			Content: userPrompt,
		},
	}
	turnCount := 0

	// =============Main Loop!!!===============
	for {
		turnCount++
		log.Printf("============= [Turn %d] 开始 ============\n", turnCount)

		availableTools := e.registry.GetAvailableTools()

		if e.EnableThinking {
			log.Println("[Engine][Phase 1] 慢思考 （Thinking）...")
			ThinkingMsg, err := e.provider.Generate(ctx, contextHistory, nil)
			if err != nil {
				return fmt.Errorf("思考过程中发生失败：%w", err)
			}
			if ThinkingMsg.Content != "" {
				fmt.Printf("[内部思考Trace]：%s\n", ThinkingMsg.Content)
				contextHistory = append(contextHistory, *ThinkingMsg)
			}
		}

		log.Println("[Engine] 正在行动 （Acting）...")
		responseMsg, err := e.provider.Generate(ctx, contextHistory, availableTools)
		if err != nil {
			return fmt.Errorf("行动生成失败：%w", err)
		}

		contextHistory = append(contextHistory, *responseMsg)

		if responseMsg.Content != "" {
			fmt.Printf("模型回复：%s\n", responseMsg.Content)
		}

		if len(responseMsg.ToolCalls) == 0 {
			log.Println("[Engine] 任务完成，退出循环。")
			break
		}

		log.Printf("[Engine] 模型请求调用 %d 个工具...\n", len(responseMsg.ToolCalls))

		var wg sync.WaitGroup
		observationMsgSlice := make([]schema.Message, len(responseMsg.ToolCalls))
		for idx, toolcall := range responseMsg.ToolCalls {
			wg.Add(1)
			go func(idx int, toolCall schema.ToolCall) {
				defer wg.Done()
				log.Printf(" -> 执行工具：%s，参数: %s\n", toolcall.Name, string(toolcall.Arguments))

				result := e.registry.Execute(ctx, toolcall)

				if result.IsError {
					log.Printf(" -> 工具执行报错：%s\n", result.Output)
				} else {
					log.Printf(" -> 工具执行成功（返回 %d 字节）\n", len(result.Output))
				}

				observationMsgSlice[idx] = schema.Message{
					Role:       schema.RoleUser,
					Content:    result.Output,
					ToolCallID: toolcall.ID,
				}

			}(idx, toolcall)
		}
		wg.Wait()
		contextHistory = append(contextHistory, observationMsgSlice...)
	}

	return nil
}
