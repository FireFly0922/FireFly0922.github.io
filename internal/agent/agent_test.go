package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"personalweb/internal/contract"
	"personalweb/internal/llm"
	"personalweb/internal/memory"
)

// fakeMemory 返回固定参照物，便于断言它被回填进对话。
type fakeMemory struct{ ctx string }

func (f fakeMemory) Context(context.Context, time.Time) (string, error) { return f.ctx, nil }

// recordingLLM 包住内层 client，记录每次 Complete 的入参。
type recordingLLM struct {
	inner llm.Client
	calls []llm.Request
}

func (r *recordingLLM) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	r.calls = append(r.calls, req)
	return r.inner.Complete(ctx, req)
}

func TestAgentToolUseLoop(t *testing.T) {
	rec := &recordingLLM{inner: llm.Mock{}}
	mem := fakeMemory{ctx: "MEMORY-SENTINEL-昨天日报"}
	ag := New(rec, mem, "test-model")

	today := time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC)
	items := []contract.Item{{ID: "g1", Source: contract.SourceGit, Title: "t", Detail: "d"}}

	report, err := ag.Report(context.Background(), today, items)
	if err != nil {
		t.Fatal(err)
	}

	// 1) 恰好两轮：首轮请求工具，回填后收尾。
	if len(rec.calls) != 2 {
		t.Fatalf("期望 2 次 LLM 调用，实得 %d", len(rec.calls))
	}

	// 2) 每轮都带上了工具定义。
	if len(rec.calls[0].Tools) == 0 || rec.calls[0].Tools[0].Name != "get_yesterday_report" {
		t.Fatalf("首轮未携带 get_yesterday_report 工具定义: %+v", rec.calls[0].Tools)
	}

	// 3) 第二轮的对话里应含参照物被回填的 tool_result。
	if !containsToolResult(rec.calls[1].Messages, "MEMORY-SENTINEL-昨天日报") {
		t.Fatalf("第二轮未回填 memory 参照物到 tool_result")
	}

	// 4) 最终输出即 mock 日报。
	if !strings.Contains(report, "今日 vs 昨天") {
		t.Fatalf("最终日报不符预期: %q", report)
	}
}

func containsToolResult(msgs []llm.Message, want string) bool {
	for _, m := range msgs {
		for _, b := range m.Blocks {
			if b.Type == llm.BlockToolResult && strings.Contains(b.Content, want) {
				return true
			}
		}
	}
	return false
}

// 编译期确认 fakeMemory 满足接口。
var _ memory.MemoryProvider = fakeMemory{}
