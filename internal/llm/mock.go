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

const defaultMockReport = `（把几份记录交给流形摊开，我沿着今天留下的水痕逐项看过去）

FireFly，今天最明显的新变化，是 daily-agent 已经打通 collect、勾选、总结到上传的最小闭环；这条路线不再只是纸面上的计划了。

Go 并发模型的笔记也还在向前流动：今天继续比较了 WaitGroup 与 errgroup 的取舍，算是沿着昨天的方向又往深处走了一步。

暂时没有从记录里看到明确搁置的新项目，所以我不会替你凭空补上一项。哼哼，账面很干净，对吧？

今天的变化我都收好了。忙完记得喝点水，明天再让我看看这些小小的涟漪会流向哪里。`

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
