// Package llm 定义 provider 无关的对话 + tool-use 契约。
//
// 类型刻意贴近 Anthropic Messages API 的形状（content blocks、tool_use / tool_result、
// stop_reason），这样 agent 主循环手写一次就能同时适配 mock 与真实实现，
// v2 接 Anthropic 时 agent 一行不用改。
package llm

import (
	"context"
	"encoding/json"
)

// Role 是一条消息的角色。
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// 内容块类型。
const (
	BlockText       = "text"
	BlockToolUse    = "tool_use"
	BlockToolResult = "tool_result"
)

// Block 是一条消息里的一个内容块：文本、工具调用、或工具结果。
type Block struct {
	Type string `json:"type"`

	// Type == BlockText
	Text string `json:"text,omitempty"`

	// Type == BlockToolUse
	ID    string          `json:"id,omitempty"`    // 工具调用 id
	Name  string          `json:"name,omitempty"`  // 工具名
	Input json.RawMessage `json:"input,omitempty"` // 工具入参（JSON）

	// Type == BlockToolResult
	ToolUseID string `json:"tool_use_id,omitempty"` // 对应的 tool_use.ID
	Content   string `json:"content,omitempty"`     // 工具返回文本
	IsError   bool   `json:"is_error,omitempty"`    // 工具是否出错

	// Type == "thinking" / "redacted_thinking"（扩展思考模型，如 DeepSeek reasoning）。
	// tool-use 多轮时必须把这些块原样回传，否则端点报 missing field `thinking`。
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
	Data      string `json:"data,omitempty"`
}

// TextBlock 便捷构造。
func TextBlock(text string) Block { return Block{Type: BlockText, Text: text} }

// ToolResultBlock 便捷构造。
func ToolResultBlock(toolUseID, content string, isErr bool) Block {
	return Block{Type: BlockToolResult, ToolUseID: toolUseID, Content: content, IsError: isErr}
}

// Message 是一轮对话消息。
type Message struct {
	Role   Role    `json:"role"`
	Blocks []Block `json:"content"`
}

// ToolDef 是给 LLM 的工具定义（JSON Schema 描述入参）。
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// Request 是一次 Messages 调用的入参。
type Request struct {
	System    string
	Messages  []Message
	Tools     []ToolDef
	Model     string
	MaxTokens int
}

// StopReason 表示模型为何停下。
type StopReason string

const (
	StopEndTurn StopReason = "end_turn" // 正常收尾
	StopToolUse StopReason = "tool_use" // 请求调用工具，需回填 tool_result 再调
	StopMaxTok  StopReason = "max_tokens"
)

// Response 是一次 Messages 调用的返回。
type Response struct {
	Blocks     []Block
	StopReason StopReason
}

// Text 拼接返回里所有文本块。
func (r Response) Text() string {
	var s string
	for _, b := range r.Blocks {
		if b.Type == BlockText {
			s += b.Text
		}
	}
	return s
}

// Client 是 provider 无关的 LLM 客户端。mock（v1）与 anthropic（v2）都实现它。
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
}
