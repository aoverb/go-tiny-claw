package main

import (
	"context"
	"log"
	"os"

	"github.com/aoverb/go-tiny-claw/internal/engine"
	"github.com/aoverb/go-tiny-claw/internal/provider"
	"github.com/aoverb/go-tiny-claw/internal/tools"
)

func main() {
	if os.Getenv("ZHIPU_API_KEY") == "" {
		log.Fatal("请先导出 ZHIPU_API_KEY 环境变量")
	}

	workDir, _ := os.Getwd()

	llmProvider := provider.NewZhipuOpenAIProvider("glm-4.5-air")

	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workDir))
	// 挂载其他的极简工具
	registry.Register(tools.NewWriteFileTool(workDir))
	registry.Register(tools.NewBashTool(workDir))
	registry.Register(tools.NewEditFileTool(workDir))

	// 实例化引擎，开启 EnableThinking = true (开启慢思考，促使模型一次性统筹规划)
	eng := engine.NewAgentEngine(llmProvider, registry, workDir, true)

	reporter := engine.NewTerminalReporter()

	prompt := `
    帮我把代码用 git 提交一下，并push到远端。
    `

	err := eng.Run(context.Background(), prompt, reporter)
	if err != nil {
		log.Fatalf("引擎运行崩溃: %v", err)
	}
}
