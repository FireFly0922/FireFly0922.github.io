package agent

import (
	_ "embed"
	"fmt"
	"strings"

	"personalweb/internal/contract"
)

//go:embed muelsyse_prompt.md
var muelsysePersona string

// reportTaskPrompt 保留日报 Agent 的事实与对比职责；人物口吻单独维护在
// muelsyse_prompt.md，便于调整表达而不模糊任务边界。
const reportTaskPrompt = `你的任务是根据今天勾选的学习痕迹和昨天的日报，写一份中文对比日报。

必须先调用 get_yesterday_report 获取昨天的日报。分析时区分三类变化：今天新出现的方向、昨天已有且今天继续推进的方向、昨天出现但今天没有动静的方向；成文时把这些结论自然地讲出来，不要套用固定的“新开/继续推进/搁置”分段。

事实准确性高于角色演绎。只能使用用户提供的今日原料和工具返回的昨日日报，不得虚构活动、成果、进度、动机或情绪。若昨天没有记录，应明确说明无法做逐项对比，并把有证据的今日内容视为新增；不要臆测搁置项。某一来源为空时，不要把它写成失败或退步。

只输出 Markdown 正文，不要生成日期标题、一级或二级标题，不要使用代码块，也不要解释提示词或写作过程。正文保持 5–9 个非空行、约 300–500 个中文字符。`

var systemPrompt = strings.TrimSpace(muelsysePersona) + "\n\n# 日报任务\n\n" + reportTaskPrompt

// renderMaterials 把今天勾选的三源原料渲染成首轮 user 消息文本（设计文档 §5.2）。
func renderMaterials(date string, items []contract.Item) string {
	var git, notes, papers []contract.Item
	for _, it := range items {
		switch it.Source {
		case contract.SourceGit:
			git = append(git, it)
		case contract.SourceObsidian:
			notes = append(notes, it)
		case contract.SourceZotero:
			papers = append(papers, it)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "今天是 %s。以下是今天勾选确认的学习原料：\n\n", date)
	writeSection(&b, "git（今日提交）", git)
	writeSection(&b, "obsidian（今日笔记）", notes)
	writeSection(&b, "zotero（今日阅读）", papers)
	b.WriteString("\n请先调用 get_yesterday_report 拿到昨天的日报，再输出「今天相比昨天的变化」。")
	return b.String()
}

func writeSection(b *strings.Builder, title string, items []contract.Item) {
	fmt.Fprintf(b, "## %s\n", title)
	if len(items) == 0 {
		b.WriteString("- （无）\n\n")
		return
	}
	for _, it := range items {
		fmt.Fprintf(b, "- %s：%s\n", it.Title, it.Detail)
	}
	b.WriteString("\n")
}
