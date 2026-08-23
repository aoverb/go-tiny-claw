package engine

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	ctxpkg "github.com/aoverb/go-tiny-claw/internal/context"
	"github.com/aoverb/go-tiny-claw/internal/provider"
	"github.com/aoverb/go-tiny-claw/internal/schema"
	"github.com/aoverb/go-tiny-claw/internal/tools"
)

type AgentEngine struct {
	provider        provider.LLMProvider
	registry        tools.Registry
	promptComposer  *ctxpkg.PromptComposer
	compactor       ctxpkg.Compactor
	deadEndDetector *DeadendDetector
	EnableThinking  bool
}

func NewAgentEngine(p provider.LLMProvider, r tools.Registry, promptComposer *ctxpkg.PromptComposer, enableThinking bool) *AgentEngine {
	return &AgentEngine{
		provider:        p,
		registry:        r,
		promptComposer:  promptComposer,
		compactor:       *ctxpkg.NewCompactor(100000, 20),
		deadEndDetector: NewDeadEndDetector(),
		EnableThinking:  enableThinking,
	}
}

func (e *AgentEngine) Run(ctx context.Context, session *ctxpkg.Session, reporter Reporter) error {
	log.Printf("[Engine] 引擎启动，会话[%s]，工作区：%s\n", session.ID, session.WorkDir)
	log.Printf("[Engine] 慢思考模式：%v\n", e.EnableThinking)

	systemMsg := e.promptComposer.Build()

	turnCount := 0

	// =============Main Loop!!!===============
	for {
		turnCount++
		log.Printf("============= [Turn %d] 开始 ============\n", turnCount)

		availableTools := e.registry.GetAvailableTools()
		workingMemory := session.GetWorkingMemory(20)

		var contextHistory []schema.Message
		contextHistory = append(contextHistory, systemMsg)
		contextHistory = append(contextHistory, workingMemory...)

		compactedContext := e.compactor.Compact(contextHistory)

		if e.EnableThinking {
			log.Println("[Engine][Phase 1] 慢思考 （Thinking）...")
			if reporter != nil {
				reporter.OnThinking(ctx)
			}
			ThinkingMsg, err := e.provider.Generate(ctx, compactedContext, nil)
			if err != nil {
				return fmt.Errorf("思考过程中发生失败：%w", err)
			}
			if ThinkingMsg.Content != "" {
				fmt.Printf("[内部思考Trace]：%s\n", ThinkingMsg.Content)
				sanitizedThinking := sanitizeThinkingTrace(ThinkingMsg.Content)
				compactedContext = append(compactedContext, schema.Message{
					Role: schema.RoleUser,
					Content: "【内部思考参考】以下是你在慢思考阶段输出的推理草稿，仅供你制定行动计划时参考。\n\n" +
						"【严重警告】草稿中出现的任何 bash(...)、write_file(...)、<tool_call> 等工具调用字样，都只是虚构的文字草稿，系统从未执行、也永远不会执行它们，它们不代表任何真实的工具调用或执行结果。\n" +
						"因此，在本次行动阶段，你必须把草稿中提到的每一条待办操作，逐一通过真实的工具调用（bash、write_file、edit_file、read_file 等）重新发起执行。严禁假设草稿中的操作已经完成，严禁编造执行结果，严禁复述草稿中的伪工具调用标签。\n\n" +
						"草稿内容如下：\n" + sanitizedThinking,
				})
			}
		}

		log.Println("[Engine] 正在行动 （Acting）...")
		responseMsg, err := e.provider.Generate(ctx, compactedContext, availableTools)
		if err != nil {
			return fmt.Errorf("行动生成失败：%w", err)
		}

		session.Append(*responseMsg)
		compactedContext = append(compactedContext, *responseMsg)

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
		e.deadEndDetector.SetNewTurn()

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
					e.deadEndDetector.NotifyFailedCall(t)
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
		session.Append(observationMsgSlice...)
		deadEndNotifyMsg := e.deadEndDetector.Summarize()
		if deadEndNotifyMsg != "" {
			session.Append(schema.Message{
				Role:    schema.RoleUser,
				Content: deadEndNotifyMsg,
			})
		}
	}

	return nil
}

func (e *AgentEngine) RunSub(ctx context.Context, textPrompt string, registry tools.Registry, reporterAny any) (string, error) {
	reporter := reporterAny.(Reporter)
	log.Printf("[SubAgent] 子Agent启动\n")

	// 【核心优化】：子智能体极其容易偷懒。我们必须在 System Prompt 中严厉警告它必须使用工具！
	contextHistory := []schema.Message{
		{
			Role: schema.RoleSystem,
			Content: `你是一个专门负责深度探索的探路者 (Explorer Subagent)。
你的任务是根据主架构师的指令，在当前工作区内仔细阅读代码、查阅日志，搜集足够的信息。

【核心纪律】
1. 你必须、且只能依靠内置工具（如 bash 的 find/grep，或 read_file）去寻找答案。绝对不允许凭空捏造或猜测！
2. 如果你没有找到确切的答案，你必须继续使用工具深入搜索。
3. 当且仅当你找到了确切的线索后，停止调用工具，直接输出一段纯文本作为你的终极汇报。主架构师会根据你的汇报来做下一步决策。`,
		},
		{
			Role:    schema.RoleUser,
			Content: textPrompt,
		},
	}

	turnCount := 0
	maxTurns := 10
	// =============Main Loop!!!===============
	for {
		if turnCount > maxTurns {
			return "", fmt.Errorf("子智能体探索过于深入，超过 %d 轮被强制召回，请主 Agent 给它更明确的指令", maxTurns)
		}
		turnCount++
		log.Printf("============= [Turn %d] 开始 ============\n", turnCount)

		availableTools := registry.GetAvailableTools()

		compactedContext := e.compactor.Compact(contextHistory)

		if e.EnableThinking {
			log.Println("[SubAgent][Phase 1] 慢思考 （Thinking）...")
			if reporter != nil {
				reporter.OnThinking(ctx)
			}
			ThinkingMsg, err := e.provider.Generate(ctx, compactedContext, nil)
			if err != nil {
				return "", fmt.Errorf("思考过程中发生失败：%w", err)
			}
			if ThinkingMsg.Content != "" {
				fmt.Printf("[SubAgent 内部思考Trace]：%s\n", ThinkingMsg.Content)
				sanitizedThinking := sanitizeThinkingTrace(ThinkingMsg.Content)
				compactedContext = append(compactedContext, schema.Message{
					Role: schema.RoleUser,
					Content: "【内部思考参考】以下是你在慢思考阶段输出的推理草稿，仅供你制定行动计划时参考。\n\n" +
						"【严重警告】草稿中出现的任何 bash(...)、write_file(...)、<tool_call> 等工具调用字样，都只是虚构的文字草稿，系统从未执行、也永远不会执行它们，它们不代表任何真实的工具调用或执行结果。\n" +
						"因此，在本次行动阶段，你必须把草稿中提到的每一条待办操作，逐一通过真实的工具调用（bash、write_file、edit_file、read_file 等）重新发起执行。严禁假设草稿中的操作已经完成，严禁编造执行结果，严禁复述草稿中的伪工具调用标签。\n\n" +
						"草稿内容如下：\n" + sanitizedThinking,
				})
			}
		}

		log.Println("[SubAgent] 正在行动 （Acting）...")
		responseMsg, err := e.provider.Generate(ctx, compactedContext, availableTools)
		if err != nil {
			return "", fmt.Errorf("行动生成失败：%w", err)
		}

		compactedContext = append(compactedContext, *responseMsg)

		if responseMsg.Content != "" && reporter != nil {
			reporter.OnMessage(ctx, responseMsg.Content)
			fmt.Printf("模型回复：%s\n", responseMsg.Content)
		}

		if len(responseMsg.ToolCalls) == 0 {
			log.Println("[SubAgent] 任务完成，退出循环。")
			return responseMsg.Content, nil
		}

		log.Printf("[SubAgent] 模型请求调用 %d 个工具...\n", len(responseMsg.ToolCalls))

		var wg sync.WaitGroup
		e.deadEndDetector.SetNewTurn()

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
					e.deadEndDetector.NotifyFailedCall(t)
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
		deadEndNotifyMsg := e.deadEndDetector.Summarize()
		if deadEndNotifyMsg != "" {
			contextHistory = append(contextHistory, schema.Message{
				Role:    schema.RoleUser,
				Content: deadEndNotifyMsg,
			})
		}
	}
}

// sanitizeThinkingTrace 清理慢思考阶段输出中可能出现的伪工具调用标签。
// 模型在“无工具可用的思考阶段”有时仍会虚构 <tool_call>、<arg_key> 等标签，
// 若这些标签原样进入行动阶段的上下文，会被模型误认为已经发生的真实工具调用，
// 进而产生“工具调用幻觉”（例如直接编造 git status 的执行结果）。
// 这里将这些伪标签替换为醒目的中文占位符，从源头打断这种错误联想。
func sanitizeThinkingTrace(trace string) string {
	replacer := strings.NewReplacer(
		"<tool_call>", "【伪工具调用标签】",
		"</tool_call>", "【伪工具调用结束】",
		"<arg_key>", "【伪参数名】",
		"</arg_key>", "【伪参数名结束】",
		"<arg_value>", "【伪参数值】",
		"</arg_value>", "【伪参数值结束】",
	)
	return replacer.Replace(trace)
}
