package engine

import (
	"context"
	"fmt"
	"log"
	"sync"

	ctxpkg "github.com/aoverb/go-tiny-claw/internal/context"
	"github.com/aoverb/go-tiny-claw/internal/provider"
	"github.com/aoverb/go-tiny-claw/internal/schema"
	"github.com/aoverb/go-tiny-claw/internal/tools"
)

type AgentEngine struct {
	provider provider.LLMProvider
	registry tools.Registry

	workDir        string
	EnableThinking bool

	promptComposer ctxpkg.PromptComposer
}

func NewAgentEngine(p provider.LLMProvider, r tools.Registry, workDir string, enableThinking bool) *AgentEngine {
	return &AgentEngine{
		provider:       p,
		registry:       r,
		workDir:        workDir,
		EnableThinking: enableThinking,
		promptComposer: *ctxpkg.NewPromptComposer(workDir),
	}
}

func (e *AgentEngine) Run(ctx context.Context, userPrompt string, reporter Reporter) error {
	log.Printf("[Engine] 引擎启动，工作区：%s\n", e.workDir)
	log.Printf("[Engine] 慢思考模式：%v\n", e.EnableThinking)

	systemPrompt := e.promptComposer.Build()
	contextHistory := []schema.Message{
		systemPrompt,
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
			if reporter != nil {
				reporter.OnThinking(ctx)
			}
			ThinkingMsg, err := e.provider.Generate(ctx, contextHistory, nil)
			if err != nil {
				return fmt.Errorf("思考过程中发生失败：%w", err)
			}
			if ThinkingMsg.Content != "" {
				fmt.Printf("[内部思考Trace]：%s\n", ThinkingMsg.Content)
			}
		}

		log.Println("[Engine] 正在行动 （Acting）...")
		responseMsg, err := e.provider.Generate(ctx, contextHistory, availableTools)
		if err != nil {
			return fmt.Errorf("行动生成失败：%w", err)
		}

		contextHistory = append(contextHistory, *responseMsg)

		if responseMsg.Content != "" && reporter != nil {
			reporter.OnMessage(ctx, responseMsg.Content)
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
			go func(idx int, t schema.ToolCall) {
				defer wg.Done()
				if reporter != nil {
					reporter.OnToolCall(ctx, t.Name, string(t.Arguments))
				}
				log.Printf(" -> 执行工具：%s，参数: %s\n", t.Name, string(t.Arguments))

				result := e.registry.Execute(ctx, t)
				if reporter != nil {
					displayOutput := result.Output
					if len(displayOutput) > 200 {
						displayOutput = displayOutput[:200] + "... (已截断)"
					}
					reporter.OnToolCallResult(ctx, t.Name, displayOutput, result.IsError)
				}

				if result.IsError {
					log.Printf(" -> 工具执行报错：%s\n", result.Output)
				} else {
					log.Printf(" -> 工具执行成功（返回 %d 字节）\n", len(result.Output))
				}

				observationMsgSlice[idx] = schema.Message{
					Role:       schema.RoleUser,
					Content:    result.Output,
					ToolCallID: t.ID,
				}

			}(idx, toolcall)
		}
		wg.Wait()
		contextHistory = append(contextHistory, observationMsgSlice...)
	}

	return nil
}
