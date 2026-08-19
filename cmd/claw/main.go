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
	r.Register(tools.NewEditFileTool(workDir))

	engine := engine.NewAgentEngine(zhipuOpenAIProvider, r, workDir, false)
	prompt := `
    我当前目录下有一个 server.go 文件。
    请帮我把里面 "TODO: 增加鉴权逻辑" 下面的那个 if 语句，整个替换为：
    if user == nil {
        fmt.Println("Forbidden!")
        return
    }
    `
	err := engine.Run(context.Background(), prompt)
	if err != nil {
		log.Fatal(err)
	}
}
