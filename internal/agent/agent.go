// Package agent 手写 tool-use 主循环 —— 设计文档 §6.1 的核心学习目标：
// 不套 agent 框架，自己实现「工具定义 → 调用 → 结果回填 → 多轮 → 停止条件」。
package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"personalweb/internal/contract"
	"personalweb/internal/llm"
	"personalweb/internal/memory"
)

// Agent 编排一次「今天 vs 昨天」的对比日报生成。
type Agent struct {
	LLM       llm.Client
	Memory    memory.MemoryProvider
	Model     string
	MaxTokens int
	MaxSteps  int // tool-use 往返上限，防止死循环
}

// New 用合理默认值构造 Agent。
func New(client llm.Client, mem memory.MemoryProvider, model string) *Agent {
	return &Agent{
		LLM:       client,
		Memory:    mem,
		Model:     model,
		MaxTokens: 1024,
		MaxSteps:  6,
	}
}

// Report 对今天勾选确认的原料跑一遍 agent 循环，产出对比日报 markdown。
func (a *Agent) Report(ctx context.Context, today time.Time, items []contract.Item) (string, error) {
	tools := newToolset(yesterdayReportTool(a.Memory, today))

	// 首轮：把今天的原料交给模型，并提示它去拉昨天的日报。
	messages := []llm.Message{{
		Role:   llm.RoleUser,
		Blocks: []llm.Block{llm.TextBlock(renderMaterials(today.Format("2006-01-02"), items))},
	}}

	for step := 0; step < a.MaxSteps; step++ {
		resp, err := a.LLM.Complete(ctx, llm.Request{
			System:    systemPrompt,
			Messages:  messages,
			Tools:     tools.defs(),
			Model:     a.Model,
			MaxTokens: a.MaxTokens,
		})
		if err != nil {
			return "", fmt.Errorf("llm 调用失败(step %d): %w", step, err)
		}

		// 把助手这轮的输出（可能含 tool_use）追加进对话历史。
		messages = append(messages, llm.Message{Role: llm.RoleAssistant, Blocks: resp.Blocks})

		// 没有请求工具 → 收尾，返回日报文本。
		if resp.StopReason != llm.StopToolUse {
			text := resp.Text()
			if text == "" {
				return "", errors.New("模型收尾但没有输出日报文本")
			}
			return text, nil
		}

		// 执行本轮所有 tool_use，收集 tool_result 回填给模型。
		results := make([]llm.Block, 0, len(resp.Blocks))
		for _, b := range resp.Blocks {
			if b.Type != llm.BlockToolUse {
				continue
			}
			out, runErr := tools.dispatch(ctx, b.Name, b.Input)
			if runErr != nil {
				results = append(results, llm.ToolResultBlock(b.ID, "工具执行出错: "+runErr.Error(), true))
				continue
			}
			results = append(results, llm.ToolResultBlock(b.ID, out, false))
		}
		messages = append(messages, llm.Message{Role: llm.RoleUser, Blocks: results})
	}

	return "", fmt.Errorf("超过最大 tool-use 轮数 %d 仍未收尾", a.MaxSteps)
}
