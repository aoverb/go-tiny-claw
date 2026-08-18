package provider

import (
	"context"

	"github.com/aoverb/go-tiny-claw/internal/schema"
)

type LLMProvider interface {
	Generate(ctx context.Context, message []schema.Message, avaliableTools []schema.ToolDefinition) (*schema.Message, error)
}
