package engine

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/aoverb/go-tiny-claw/internal/schema"
)

var (
	RETRY_LIMIT = 3
)

type DeadendDetector struct {
	errCount                 int
	consecutiveFailuresCount map[string]int
	consecutiveFailures      map[string]schema.ToolCall
}

func NewDeadEndDetector() *DeadendDetector {
	return &DeadendDetector{
		errCount:                 0,
		consecutiveFailuresCount: make(map[string]int),
		consecutiveFailures:      make(map[string]schema.ToolCall),
	}
}

func (d *DeadendDetector) SetNewTurn() {
	d.errCount = 0
}

func (d *DeadendDetector) NotifyFailedCall(t schema.ToolCall) {
	d.errCount++

	fingerPrint := d.generateFingerPrints(t)
	d.consecutiveFailures[fingerPrint] = t
	d.consecutiveFailuresCount[fingerPrint]++
}

func (d *DeadendDetector) generateFingerPrints(t schema.ToolCall) string {
	m := md5.New()
	m.Write([]byte(t.Name))
	m.Write(t.Arguments)
	return hex.EncodeToString(m.Sum(nil))
}

func (d *DeadendDetector) getExceededRecord() string {
	var recordBuilder strings.Builder
	for fingerPrint, failureCount := range d.consecutiveFailuresCount {
		if failureCount < RETRY_LIMIT {
			continue
		}
		recordBuilder.WriteString(fmt.Sprintf("连续 %d 次使用相同的参数调用了 '%s' 工具，\n", failureCount, d.consecutiveFailures[fingerPrint].Name))
	}
	return recordBuilder.String()
}

func (d *DeadendDetector) Summarize() string {
	if d.errCount == 0 {
		d.consecutiveFailuresCount = make(map[string]int)
		d.consecutiveFailures = make(map[string]schema.ToolCall)
		return ""
	}

	exceededRecord := d.getExceededRecord()
	if exceededRecord == "" {
		return ""
	}

	log.Println("[Deadend Detector] ⚠️ 触发死循环干预！注入强力修正指令。")
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString("[SYSTEM DEADEND DETECTOR 警告] 你似乎陷入了死循环。你刚刚：\n")
	summaryBuilder.WriteString(exceededRecord)
	summaryBuilder.WriteString(`并且都失败了。
		请立即停止这种无效的重试！你的注意力被当前的报错过度吸引了。
		你需要：
		1. 停止猜测参数。跳出当前的局部思维。
		2. 彻底改变你的策略。
		3. 如果你确实无法通过系统工具解决当前问题，请直接结束任务并向用户说明你需要什么人工帮助，而不是继续盲目消耗 API 资源尝试。`)
	return summaryBuilder.String()
}
