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

	engine := engine.NewAgentEngine(zhipuOpenAIProvider, r, workDir, false)
	err := engine.Run(context.Background(), "请调用工具读取一下当前工作区目录下 hello.txt 文件的内容，并用一句话向我总结它说了什么。")
	if err != nil {
		log.Fatal(err)
	}
}
