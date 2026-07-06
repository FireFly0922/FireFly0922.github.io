package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Anthropic 是 Messages API 的真实实现（v2）。
//
// 刻意用 net/http 手写而非官方 SDK：一是本机 Go proxy 被墙、拉不动外部依赖，
// 二是设计文档 §6.1 的学习目标就是亲手把 tool-use 协议走一遍。
// 本包的 Message/Block/ToolDef 的 JSON tag 已对齐 Anthropic 的 content-block 形状，
// 因此可直接复用它们做序列化。
type Anthropic struct {
	APIKey    string // 官方：走 x-api-key 头
	AuthToken string // 第三方兼容网关（如 DeepSeek）：走 Authorization: Bearer，优先于 APIKey
	Model     string // 默认模型；Request.Model 非空时以后者为准
	BaseURL   string // 默认 https://api.anthropic.com，可指向任意 Anthropic 兼容端点
	HTTP      *http.Client
}

// NewAnthropic 构造一个带合理默认值的客户端。
func NewAnthropic(apiKey, model string) *Anthropic {
	return &Anthropic{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://api.anthropic.com",
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

const anthropicVersion = "2023-06-01"

type anthropicRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Tools     []ToolDef `json:"tools,omitempty"`
	Messages  []Message `json:"messages"`
}

type anthropicResponse struct {
	Content    []Block `json:"content"`
	StopReason string  `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Complete 发一次 Messages 请求，把结果映射回 provider 无关的 Response。
func (a *Anthropic) Complete(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = a.Model
	}
	maxTok := req.MaxTokens
	if maxTok <= 0 {
		maxTok = 1024
	}

	payload := anthropicRequest{
		Model:     model,
		MaxTokens: maxTok,
		System:    req.System,
		Tools:     req.Tools,
		Messages:  req.Messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("编码请求: %w", err)
	}

	base := a.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	if a.AuthToken != "" {
		httpReq.Header.Set("authorization", "Bearer "+a.AuthToken)
	} else {
		httpReq.Header.Set("x-api-key", a.APIKey)
	}
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	client := a.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("请求 Anthropic: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("Anthropic 返回 %d: %s", resp.StatusCode, string(raw))
	}

	var out anthropicResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, fmt.Errorf("解析响应: %w", err)
	}
	if out.Error != nil {
		return Response{}, fmt.Errorf("Anthropic 错误 %s: %s", out.Error.Type, out.Error.Message)
	}

	return Response{Blocks: out.Content, StopReason: StopReason(out.StopReason)}, nil
}

var _ Client = (*Anthropic)(nil)
