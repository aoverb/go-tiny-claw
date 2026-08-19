package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aoverb/go-tiny-claw/internal/schema"
)

type Registry interface {
	Register(tool BaseTool)
	GetAvailableTools() []schema.ToolDefinition

	Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult
}

type BaseTool interface {
	Name() string
	Definition() schema.ToolDefinition
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}
type RegistryImpl struct {
	tools map[string]BaseTool
}

func NewRegistry() Registry {
	return &RegistryImpl{
		tools: make(map[string]BaseTool),
	}
}

func (r *RegistryImpl) Register(tool BaseTool) {
	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		log.Printf("[warning] 工具'%s'已存在，将被覆盖", name)
		return
	}
	r.tools[name] = tool
	log.Printf("[Registry] 成功挂载工具'%s'", name)
}

func (r *RegistryImpl) GetAvailableTools() []schema.ToolDefinition {
	defs := []schema.ToolDefinition{}

	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}

	return defs
}

func (r *RegistryImpl) Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult {
	name := call.Name
	if _, exists := r.tools[name]; !exists {
		log.Printf("[error] 需要调用的工具'%s'不存在", name)
		return schema.ToolResult{
			ToolCallID: call.ID,
			IsError:    true,
			Output:     fmt.Sprintf("[error] 需要调用的工具'%s'不存在", name),
		}
	}
	output, err := r.tools[name].Execute(ctx, call.Arguments)
	if err != nil {
		return schema.ToolResult{
			ToolCallID: call.ID,
			IsError:    true,
			Output:     fmt.Sprintf("调用工具 %s 时发生错误：%w", name, err),
		}
	}
	return schema.ToolResult{
		ToolCallID: call.ID,
		IsError:    false,
		Output:     fmt.Sprintf("%w", output),
	}
}
