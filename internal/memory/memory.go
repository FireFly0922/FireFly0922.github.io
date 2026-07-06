// Package memory 抽象 agent 的「参照物提供者」。
//
// 设计文档 §5：「今天 vs 昨天」的核心是 diff 不是 summary，天然需要昨天作参照。
// 第一版参照物不从向量库回忆，而是从磁盘读 —— 就是每天 push 上去的 reports/*.md。
// 用接口预留升级位：以后换 RollingSummary（压缩）/ VectorRetrieval（检索），
// agent 主循环一行不用改。
package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// MemoryProvider 提供「昨天/历史」参照物。
type MemoryProvider interface {
	Context(ctx context.Context, today time.Time) (string, error)
}

// YesterdayReport 是第 2 阶段的最简实现：只读昨天的日报。
type YesterdayReport struct {
	Dir string // reports 目录，如 site/reports
}

// Context 读取 today 前一天的 reports/YYYY-MM-DD.md。
// 第一天（昨天没有记录）要优雅处理，不能报错。
func (y YesterdayReport) Context(_ context.Context, today time.Time) (string, error) {
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	path := filepath.Join(y.Dir, yesterday+".md")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "（昨天没有记录）", nil
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// 编译期断言：YesterdayReport 实现了 MemoryProvider。
var _ MemoryProvider = YesterdayReport{}
