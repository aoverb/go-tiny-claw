package eval

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	ctxpkg "github.com/aoverb/go-tiny-claw/internal/context"
	"github.com/aoverb/go-tiny-claw/internal/engine"
	"github.com/aoverb/go-tiny-claw/internal/observability"
	"github.com/aoverb/go-tiny-claw/internal/provider"
	"github.com/aoverb/go-tiny-claw/internal/schema"
	"github.com/aoverb/go-tiny-claw/internal/tools"
)

type Testcase struct {
	ID             string
	Name           string
	SetupScript    string
	TaskPrompt     string
	ValidateScript string
	MaxTurns       int
}

type TestResult struct {
	TestcaseID   string
	Passed       bool
	TotalCostCNY float64
	ErrorMsg     string
	DurationMs   int64
}

type BenchmarkRunner struct {
	modelName string
}

func NewBenchmarkRunner(modelName string) *BenchmarkRunner {
	return &BenchmarkRunner{
		modelName: modelName,
	}
}

func (r *BenchmarkRunner) runSingleTest(ctx context.Context, testcase Testcase) TestResult {
	workDir, _ := os.Getwd()
	workDir += fmt.Sprintf("/workspace/%s_%d", testcase.ID, time.Now().Unix())
	_ = os.MkdirAll(workDir, 0755)

	if testcase.SetupScript != "" {
		cmd := exec.Command("bash", "-c", testcase.SetupScript)
		cmd.Dir = workDir
		if err := cmd.Run(); err != nil {
			return TestResult{
				TestcaseID: testcase.ID,
				Passed:     false,
				ErrorMsg:   "配置脚本执行失败",
			}
		}
	}

	composer := ctxpkg.NewPromptComposer(workDir, false)

	provider := provider.NewZhipuOpenAIProvider("glm-4.5-air")
	sess := ctxpkg.NewSession(testcase.ID, workDir)
	sess.Append(schema.Message{
		Role:    schema.RoleUser,
		Content: testcase.TaskPrompt,
	})
	tracker := observability.NewCostTracker(provider, "glm-4.5-air", sess)

	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workDir))
	registry.Register(tools.NewWriteFileTool(workDir))
	registry.Register(tools.NewEditFileTool(workDir))
	registry.Register(tools.NewBashTool(workDir))

	eng := engine.NewAgentEngine(tracker, registry, composer, false)
	startTime := time.Now()
	eng.Run(ctx, sess, nil)
	cmd := exec.Command("bash", "-c", testcase.ValidateScript)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return TestResult{
			TestcaseID:   testcase.ID,
			Passed:       false,
			TotalCostCNY: sess.TotalCostCNY,
			DurationMs:   duration,
			ErrorMsg:     fmt.Sprintf("验证脚本执行失败: %s", string(out)),
		}
	}

	return TestResult{
		TestcaseID:   testcase.ID,
		Passed:       true,
		TotalCostCNY: sess.TotalCostCNY,
		DurationMs:   duration,
	}
}

func (r *BenchmarkRunner) RunSuite(ctx context.Context, testcases []Testcase) {
	log.Println("==================================================")
	log.Printf("🚀 启动自动化 Harness Benchmark 评估... | 模型: %s\n", r.modelName)
	log.Println("==================================================")

	var results []TestResult
	passedCount := 0
	totalCost := 0.0

	for _, tc := range testcases {
		log.Printf("\n>>> ⏳ 正在执行用例 [%s]: %s\n", tc.ID, tc.Name)

		res := r.runSingleTest(ctx, tc)
		results = append(results, res)

		if res.Passed {
			passedCount++
			log.Printf(">>> ✅ 用例 [%s] 测试通过! | 耗时: %dms | 花费: $%.6f\n", tc.ID, res.DurationMs, res.TotalCostCNY)
		} else {
			log.Printf(">>> ❌ 用例 [%s] 测试失败! | 错误: %s\n", tc.ID, res.ErrorMsg)
		}
		totalCost += res.TotalCostCNY
	}

	// 打印终极报表
	log.Println("\n================ 🏆 跑分终极报告 ================")
	log.Printf("总用例数: %d | 成功数: %d | 成功率: %.2f%%\n", len(testcases), passedCount, float64(passedCount)/float64(len(testcases))*100)
	log.Printf("总消耗成本: $%.6f\n", totalCost)
	log.Println("==================================================")
}
