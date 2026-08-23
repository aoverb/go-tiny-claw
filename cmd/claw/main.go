package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	ctxpkg "github.com/aoverb/go-tiny-claw/internal/context"
	"github.com/aoverb/go-tiny-claw/internal/engine"
	"github.com/aoverb/go-tiny-claw/internal/observability"
	"github.com/aoverb/go-tiny-claw/internal/provider"
	"github.com/aoverb/go-tiny-claw/internal/schema"
	"github.com/aoverb/go-tiny-claw/internal/tools"
)

func main() {
	if os.Getenv("ZHIPU_API_KEY") == "" {
		log.Fatal("请先导出 ZHIPU_API_KEY 环境变量")
	}

	workDir, _ := os.Getwd()
	workDir = filepath.Join(workDir, "")
	llmProvider := provider.NewZhipuOpenAIProvider("glm-4.5-air")

	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workDir))
	registry.Register(tools.NewWriteFileTool(workDir))
	registry.Register(tools.NewEditFileTool(workDir))
	registry.Register(tools.NewBashTool(workDir))

	readonlyRegistry := tools.NewRegistry()
	readonlyRegistry.Register(tools.NewReadFileTool(workDir))
	readonlyRegistry.Register(tools.NewBashTool(workDir))

	promptComposer := ctxpkg.NewPromptComposer(workDir, true)

	sessionID := "task_push_01"
	sess := ctxpkg.GlobalSessionMgr.GetOrCreate(sessionID, workDir)

	eng := engine.NewAgentEngine(observability.NewCostTracker(llmProvider, "glm-4.5-air", sess), registry, promptComposer, true)
	reporter := engine.NewTerminalReporter()
	registry.Register(tools.NewSubAgentTool(eng, readonlyRegistry, reporter))

	prompt := `
    帮我认真分析本次工作目录下代码库修改新增的内容，commit合适的信息，然后推送到远端仓库。
    `

	sess.Append(schema.Message{Role: schema.RoleUser, Content: prompt})

	err := eng.Run(context.Background(), sess, reporter)

	if err != nil {
		log.Fatalf("引擎运行崩溃: %v", err)
	}

	log.Printf("\n================ 财务报表 ================\n")
	log.Printf("会话 ID: %s\n", sess.ID)
	log.Printf("总消耗 Input Tokens: %d\n", sess.TotalPromptTokens)
	log.Printf("总消耗 Output Tokens: %d\n", sess.TotalCompletionTokens)
	log.Printf("总计费用 (CNY): ¥%.6f\n", sess.TotalCostCNY)
	log.Printf("==========================================\n")
}
