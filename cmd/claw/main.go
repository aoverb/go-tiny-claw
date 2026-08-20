package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aoverb/go-tiny-claw/internal/engine"
	"github.com/aoverb/go-tiny-claw/internal/feishu"
	"github.com/aoverb/go-tiny-claw/internal/provider"
	"github.com/aoverb/go-tiny-claw/internal/tools"
	"github.com/larksuite/oapi-sdk-go/v3/core/httpserverext"
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

	bot := feishu.NewFeishuBot(eng)
	handler := httpserverext.NewEventHandlerFunc(bot.GetEventDispatcher())

	http.HandleFunc("/webhook/event", handler)
	http.HandleFunc("/webhook/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Pong!")
	})
	port := ":48080"
	log.Printf("go-tiny-claw 飞书服务端已启动，端口%s", port)
	err := http.ListenAndServe(port, nil)

	if err != nil {
		log.Fatalf("引擎运行崩溃: %v", err)
	}
}
