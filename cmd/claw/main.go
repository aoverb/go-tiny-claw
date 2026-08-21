package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	ctxpkg "github.com/aoverb/go-tiny-claw/internal/context"
	"github.com/aoverb/go-tiny-claw/internal/engine"
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

	promptComposer := ctxpkg.NewPromptComposer(workDir, true)
	eng := engine.NewAgentEngine(llmProvider, registry, promptComposer, true)
	reporter := engine.NewTerminalReporter()

	sessionID := "task_web_server_01"
	sess := ctxpkg.GlobalSessionMgr.GetOrCreate(sessionID, workDir)

	prompt := `
    我需要你把本目录的代码提交并推送到远端。
    `

	sess.Append(schema.Message{Role: schema.RoleUser, Content: prompt})

	err := eng.Run(context.Background(), sess, reporter)
	if err != nil {
		log.Fatalf("引擎运行崩溃: %v", err)
	}
}
