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

	zhipuOpenAIProvider := provider.NewZhipuOpenAIProvider("glm-4.5-air")

	r := tools.NewRegistry()
	r.Register(tools.NewReadFileTool(workDir))
	r.Register(tools.NewWriteFileTool(workDir))
	r.Register(tools.NewBashTool(workDir))

	engine := engine.NewAgentEngine(zhipuOpenAIProvider, r, workDir, false)
	prompt := `
    请帮我执行以下操作：
    1. 用 bash 查看一下我当前电脑的 Go 版本。
    2. 帮我写一个简单的 helloworld.go 文件，输出 "Hello, go-tiny-claw!"。
    3. 用 bash 编译并运行这个 go 文件，确认它能正常工作。
    `
	err := engine.Run(context.Background(), prompt)
	if err != nil {
		log.Fatal(err)
	}
}
