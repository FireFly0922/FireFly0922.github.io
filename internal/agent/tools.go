package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"personalweb/internal/llm"
	"personalweb/internal/memory"
)

// tool 把「给 LLM 的定义」与「本地真正执行的函数」绑在一起。
type tool struct {
	def llm.ToolDef
	run func(ctx context.Context, input json.RawMessage) (string, error)
}

// toolset 是一批工具，按名字调度。
type toolset struct {
	tools map[string]tool
}

func newToolset(ts ...tool) *toolset {
	m := make(map[string]tool, len(ts))
	for _, t := range ts {
		m[t.def.Name] = t
	}
	return &toolset{tools: m}
}

// defs 返回所有工具定义，喂给 llm.Request.Tools。
func (s *toolset) defs() []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(s.tools))
	for _, t := range s.tools {
		defs = append(defs, t.def)
	}
	return defs
}

// dispatch 按名字执行工具，返回结果文本。
func (s *toolset) dispatch(ctx context.Context, name string, input json.RawMessage) (string, error) {
	t, ok := s.tools[name]
	if !ok {
		return "", fmt.Errorf("未知工具: %s", name)
	}
	return t.run(ctx, input)
}

// yesterdayReportTool 让模型能主动拉取昨天的日报作参照物。
// 参照物本身来自 MemoryProvider —— 今天是读文件，以后可换压缩/检索，工具定义不变。
func yesterdayReportTool(mem memory.MemoryProvider, today time.Time) tool {
	return tool{
		def: llm.ToolDef{
			Name:        "get_yesterday_report",
			Description: "获取昨天的学习日报（Markdown）作为对比参照。若昨天没有记录，会返回提示文本。",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		run: func(ctx context.Context, _ json.RawMessage) (string, error) {
			return mem.Context(ctx, today)
		},
	}
}
