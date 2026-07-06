package collect

import (
	"context"
	"time"

	"personalweb/internal/contract"
)

// 本文件是 v1 的假数据读取器：不碰真实数据源，只返回与 day 绑定的确定假数据，
// 用来先打通「采集 → 勾选 → 总结 → 上传」整条链路。
// v2 会用 git.go / obsidian.go / zotero.go 里的真实实现替换它们。

// StubGit 模拟 git 今日提交。
type StubGit struct{}

func (StubGit) Name() contract.Source { return contract.SourceGit }

func (StubGit) Collect(_ context.Context, day time.Time) ([]contract.Item, error) {
	d := day.Format("2006-01-02")
	return []contract.Item{
		{
			ID:     "git-" + d + "-1",
			Source: contract.SourceGit,
			Title:  "feat: 打通 daily-agent 最小闭环",
			Detail: "新增 collect/agent/store/server 骨架，整条链路空跑通过。",
			Tags:   []string{"go", "agent"},
		},
		{
			ID:     "git-" + d + "-2",
			Source: contract.SourceGit,
			Title:  "chore: 定死数据契约 contract.go",
			Detail: "Item / DayData / IndexEntry 三类型落地。",
			Tags:   []string{"go"},
		},
	}, nil
}

// StubObsidian 模拟今日改动的 Obsidian 笔记。
type StubObsidian struct{}

func (StubObsidian) Name() contract.Source { return contract.SourceObsidian }

func (StubObsidian) Collect(_ context.Context, day time.Time) ([]contract.Item, error) {
	d := day.Format("2006-01-02")
	return []contract.Item{
		{
			ID:     "obs-" + d + "-1",
			Source: contract.SourceObsidian,
			Title:  "Go 并发模型笔记",
			Detail: "整理 goroutine / channel / WaitGroup 与 errgroup 的取舍。",
			Tags:   []string{"go", "concurrency"},
		},
	}, nil
}

// StubZotero 模拟今日阅读的文献。
type StubZotero struct{}

func (StubZotero) Name() contract.Source { return contract.SourceZotero }

func (StubZotero) Collect(_ context.Context, day time.Time) ([]contract.Item, error) {
	d := day.Format("2006-01-02")
	return []contract.Item{
		{
			ID:     "zot-" + d + "-1",
			Source: contract.SourceZotero,
			Title:  "Building Effective Agents (Anthropic)",
			Detail: "读到 tool-use 循环与何时才需要 memory 检索一节。",
			Tags:   []string{"agent", "llm"},
		},
	}, nil
}

// DefaultStubs 返回 v1 的三个假数据读取器。
func DefaultStubs() []Collector {
	return []Collector{StubGit{}, StubObsidian{}, StubZotero{}}
}

// 确保三个 stub 实现了 Collector 接口（编译期断言）。
var (
	_ Collector = StubGit{}
	_ Collector = StubObsidian{}
	_ Collector = StubZotero{}
)
