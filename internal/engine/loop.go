package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/aoverb/go-tiny-claw/internal/provider"
	"github.com/aoverb/go-tiny-claw/internal/schema"
	"github.com/aoverb/go-tiny-claw/internal/tools"
)

type AgentEngine struct {
	provider provider.LLMProvider
	registry tools.Registry

	workDir string
}

func NewAgentEngine(p provider.LLMProvider, r tools.Registry, workDir string) *AgentEngine {
	return &AgentEngine{
		provider: p,
		registry: r,
		workDir:  workDir,
	}
}

func (e *AgentEngine) Run(ctx context.Context, userPrompt string) error {
	log.Printf("[Engine] 引擎启动，工作区：%s\n")
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

		log.Println("[Engine] 正在思考 （Reasoning）...")
		responseMsg, err := e.provider.Generate(ctx, contextHistory, availableTools)
		if err != nil {
			return fmt.Errorf("模型生成失败：%w", err)
		}

		contextHistory = append(contextHistory, *responseMsg)

		if responseMsg.Content != "" {
			fmt.Printf("模型：%s\n", responseMsg.Content)
		}

		if len(responseMsg.ToolCalls) == 0 {
			log.Println("[Engine] 任务完成，退出循环。")
			break
		}

		log.Printf("[Engine] 模型请求调用 %d 个工具...\n", len(responseMsg.ToolCalls))

		for _, toolcall := range responseMsg.ToolCalls {
			log.Printf(" -> 执行工具：%s，参数: %s\n", toolcall.Name, string(toolcall.Arguments))

			result := e.registry.Execute(ctx, toolcall)

			if result.IsError {
				log.Printf(" -> 工具执行报错：%s\n", result.Output)
			} else {
				log.Printf(" -> 工具执行成功（返回 %d 字节）\n", len(result.Output))
			}

			observationMsg := schema.Message{
				Role:       schema.RoleUser,
				Content:    result.Output,
				ToolCallID: toolcall.ID,
			}
			contextHistory = append(contextHistory, observationMsg)
		}
	}

	return nil
}
