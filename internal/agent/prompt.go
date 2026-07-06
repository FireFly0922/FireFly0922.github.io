package agent

import (
	"fmt"
	"strings"

	"personalweb/internal/contract"
)

// systemPrompt 对应设计文档 §5.2：要 diff 不要 summary。
const systemPrompt = `你是我的学习记录助手。你的任务是对比昨天和今天的学习痕迹，写一份简洁的中文日报。
不要泛泛总结今天做了什么，而要指出「变化」：
- 新开：今天新出现、昨天没有的方向。
- 继续推进：昨天已在做、今天有进展的。
- 搁置：昨天在做、今天没有动静的。

你可以调用 get_yesterday_report 工具拿到昨天的日报作为参照。
输出 Markdown，控制在 200 字以内，朴实准确，不要客套。`

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
