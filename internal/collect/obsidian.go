package collect

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"personalweb/internal/contract"
)

// Obsidian 遍历 vault，挑出今天 mtime 改过的 .md 笔记（设计文档 v2）。
// 不做版本管理，只取「今天动过哪些笔记」这一层。
type Obsidian struct {
	Vault string // Obsidian vault 根目录
}

func (Obsidian) Name() contract.Source { return contract.SourceObsidian }

func (o Obsidian) Collect(_ context.Context, day time.Time) ([]contract.Item, error) {
	if o.Vault == "" {
		return nil, nil // 未配置 vault：安静跳过
	}
	y, m, d := day.Date()

	var items []contract.Item
	err := filepath.WalkDir(o.Vault, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // 单个路径出错就跳过，不中断整棵树
		}
		name := entry.Name()
		if entry.IsDir() {
			// 跳过 Obsidian 自身配置、回收站与隐藏目录。
			if path != o.Vault && (name == ".obsidian" || name == ".trash" || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(name), ".md") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		my, mm, md := info.ModTime().Date()
		if my != y || mm != m || md != d {
			return nil // 不是今天动的
		}

		rel, relErr := filepath.Rel(o.Vault, path)
		if relErr != nil {
			rel = name
		}
		rel = filepath.ToSlash(rel)
		items = append(items, contract.Item{
			ID:      "obs-" + rel,
			Source:  contract.SourceObsidian,
			Title:   strings.TrimSuffix(name, filepath.Ext(name)),
			Detail:  firstLine(path),
			Content: fullContent(path),
			Tags:    noteTags(rel),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// firstLine 取笔记里第一段有意义的文字作摘要：
// 跳过 YAML frontmatter 与空行，去掉标题的 # 前缀，截断到 120 字符。
func firstLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	inFront := false
	for i := 0; sc.Scan() && i < 200; i++ {
		line := strings.TrimSpace(sc.Text())
		if i == 0 && line == "---" { // frontmatter 起始
			inFront = true
			continue
		}
		if inFront {
			if line == "---" {
				inFront = false
			}
			continue
		}
		if line == "" {
			continue
		}
		line = strings.TrimLeft(line, "# ")
		if line == "" {
			continue
		}
		return truncate(line, 120)
	}
	return ""
}

// fullContent 读整篇笔记正文，供详情页展示。超大文件截断到 20000 rune 防止快照膨胀。
func fullContent(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return truncate(string(b), 20000)
}

// noteTags 用笔记所在的一级子目录作标签（便于分类）。rel 已是斜杠分隔。
func noteTags(rel string) []string {
	if i := strings.Index(rel, "/"); i > 0 {
		return []string{rel[:i]}
	}
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var _ Collector = Obsidian{}
