package llm

import (
	"context"
	"encoding/json"
)

// Mock 是 v1 的空跑 LLM：不联网、不花钱，但真实地驱动一次 tool-use 往返，
// 用来验证 agent 主循环的「工具定义 → 调用 → 结果回填 → 停止条件」是否正确。
//
// 行为：
//   - 首轮（消息里还没有 tool_result）→ 返回一个 tool_use，请求调用 get_yesterday_report，
//     stop_reason = tool_use。
//   - 收到 tool_result 后 → 返回固定的对比日报文本，stop_reason = end_turn。
type Mock struct {
	// ToolName 指定首轮要调用的工具名（默认 get_yesterday_report）。
	ToolName string
	// Report 是收尾时返回的日报文本（默认见 defaultMockReport）。
	Report string
}

const defaultMockReport = `## 今日 vs 昨天

**新开**
- 打通了 daily-agent 的最小闭环（collect → 勾选 → 总结 → 上传）。

**继续推进**
- Go 并发模型笔记：对比了 WaitGroup 与 errgroup 的取舍。

**搁置**
- 暂无。

> （mock 生成，用于 v1 空跑验证）`

func (m Mock) Complete(_ context.Context, req Request) (Response, error) {
	if hasToolResult(req.Messages) {
		report := m.Report
		if report == "" {
			report = defaultMockReport
		}
		return Response{
			Blocks:     []Block{TextBlock(report)},
			StopReason: StopEndTurn,
		}, nil
	}

	name := m.ToolName
	if name == "" {
		name = "get_yesterday_report"
	}
	return Response{
		Blocks: []Block{{
			Type:  BlockToolUse,
			ID:    "mock-tool-1",
			Name:  name,
			Input: json.RawMessage(`{}`),
		}},
		StopReason: StopToolUse,
	}, nil
}

// hasToolResult 判断对话里是否已经回填过工具结果。
func hasToolResult(msgs []Message) bool {
	for _, msg := range msgs {
		for _, b := range msg.Blocks {
			if b.Type == BlockToolResult {
				return true
			}
		}
	}
	return false
}

var _ Client = Mock{}
