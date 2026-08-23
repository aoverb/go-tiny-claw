package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aoverb/go-tiny-claw/internal/schema"
)

type SubAgentRunner interface {
	RunSub(ctx context.Context, textPrompt string, registry Registry, reporterAny any) (string, error)
}

type SubAgentTool struct {
	subAgentRunner SubAgentRunner
	reporter       any
	registry       Registry
}

func NewSubAgentTool(subAgentRunner SubAgentRunner, registry Registry, reporter any) *SubAgentTool {
	return &SubAgentTool{
		subAgentRunner: subAgentRunner,
		reporter:       reporter,
		registry:       registry,
	}
}

func (t *SubAgentTool) Name() string {
	return "spawn_subagent"
}

func (t *SubAgentTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        t.Name(),
		Description: "派出一个专门用于深度探索（Exploration）的子智能体。当你需要阅读大量代码、跨文件查找逻辑时请调用此工具。它在探索完毕后，会给你返回一份极度精炼的摘要报告。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_prompt": map[string]any{
					"type":        "string",
					"description": "给子智能体下达的明确指令。",
				},
			},
			"required": []string{"task_prompt"},
		},
	}
}

type subagentArgs struct {
	TaskPrompt string `json:"task_prompt"`
}

func (t *SubAgentTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var input subagentArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("参数解析失败：%w", err)
	}

	prompt := input.TaskPrompt

	return t.subAgentRunner.RunSub(ctx, prompt, t.registry, t.reporter)
}
