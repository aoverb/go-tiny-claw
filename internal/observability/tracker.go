package observability

import (
	"context"
	"log"
	"time"

	ctxpkg "github.com/aoverb/go-tiny-claw/internal/context"
	"github.com/aoverb/go-tiny-claw/internal/provider"
	"github.com/aoverb/go-tiny-claw/internal/schema"
)

var PricingModel = map[string]struct {
	InputPrice  float64
	OutputPrice float64
}{
	"glm-4.5-air": {InputPrice: 0.15, OutputPrice: 0.15}, // 这里假定的大模型价格(每百万Token，tk)
}

type CostTracker struct {
	nextProvider provider.LLMProvider
	session      *ctxpkg.Session
	modelName    string
}

func NewCostTracker(nextProvider provider.LLMProvider, modelName string, session *ctxpkg.Session) *CostTracker {
	return &CostTracker{
		nextProvider: nextProvider,
		session:      session,
		modelName:    modelName,
	}
}

func (c *CostTracker) Generate(ctx context.Context, message []schema.Message, avaliableTools []schema.ToolDefinition) (*schema.Message, error) {
	startTime := time.Now()
	msg, err := c.nextProvider.Generate(ctx, message, avaliableTools)
	latency := time.Since(startTime)

	if err != nil {
		log.Printf("[Tracker] ❌ API 调用失败，耗时: %v\n", latency)
		return msg, err
	}

	if msg.Usage == nil {
		log.Printf("[Tracker] ⚠️ API 调用完成，但未返回 Usage 数据 | 耗时: %v\n", latency)
		return msg, err
	}

	completionTokens := msg.Usage.CompletionTokens
	promptTokens := msg.Usage.PromptTokens

	costCNY := float64(0)
	if rate, exist := PricingModel[c.modelName]; exist {
		costCNY += (float64(rate.InputPrice)*float64(promptTokens) + float64(rate.OutputPrice)*float64(completionTokens)) / 1000000.0
	}

	log.Printf("[Tracker] 📊 API 调用完成 | 耗时: %v | 输入: %d tk | 输出: %d tk | 花费: ¥%.6f\n",
		latency, promptTokens, completionTokens, costCNY)

	// 6. 将账单累加到当前的 Session 中，供人类后续随时查询
	if c.session != nil {
		c.session.RecordUsage(promptTokens, completionTokens, costCNY)
		log.Printf("[Tracker] 💰 当前会话 (%s) 累计花费: ¥%.6f\n", c.session.ID, c.session.TotalCostCNY)
	}
	return msg, err
}
