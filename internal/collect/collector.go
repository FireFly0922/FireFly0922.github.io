// Package collect 定义三源读取器的统一接口，并发采集当天痕迹。
package collect

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"personalweb/internal/contract"
)

// Collector 是一个源的读取器。每个源（git / obsidian / zotero）一个实现。
type Collector interface {
	Name() contract.Source
	Collect(ctx context.Context, day time.Time) ([]contract.Item, error)
}

// CollectAll 并发跑所有读取器，合并结果。
//
// 用 errgroup（设计文档 §6.1 点名的「主场」）：任一源出错即取消其余，返回首个错误；
// 每个源写各自的结果槽位，避免共享切片的锁。
func CollectAll(ctx context.Context, day time.Time, collectors ...Collector) ([]contract.Item, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := make([][]contract.Item, len(collectors))

	for i, c := range collectors {
		g.Go(func() error {
			items, err := c.Collect(ctx, day)
			if err != nil {
				return fmt.Errorf("collect %s: %w", c.Name(), err)
			}
			results[i] = items
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	var all []contract.Item
	for _, r := range results {
		all = append(all, r...)
	}
	contract.SortItems(all) // 抹平并发乱序，输出确定
	return all, nil
}
