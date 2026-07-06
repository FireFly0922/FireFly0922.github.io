package collect

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestObsidianCollectsTodayOnly(t *testing.T) {
	vault := t.TempDir()
	ctx := context.Background()
	now := time.Now()

	// 今天动过的笔记（新建即 mtime=now）。
	writeFile(t, filepath.Join(vault, "today.md"), "---\ntags: x\n---\n# 今日主题\n正文一句话。")
	// 子目录里的今日笔记（用于验证一级目录成为 tag）。
	sub := filepath.Join(vault, "项目")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "nested.md"), "内容")

	// 两天前的旧笔记，不该被采到。
	old := filepath.Join(vault, "old.md")
	writeFile(t, old, "旧内容")
	twoDaysAgo := now.AddDate(0, 0, -2)
	if err := os.Chtimes(old, twoDaysAgo, twoDaysAgo); err != nil {
		t.Fatal(err)
	}

	// .obsidian 配置目录应被跳过。
	cfg := filepath.Join(vault, ".obsidian")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cfg, "workspace.md"), "should be ignored")

	items, err := Obsidian{Vault: vault}.Collect(ctx, now)
	if err != nil {
		t.Fatal(err)
	}

	titles := map[string]bool{}
	for _, it := range items {
		titles[it.Title] = true
	}
	if titles["old"] {
		t.Error("采到了两天前的旧笔记")
	}
	if !titles["today"] || !titles["nested"] {
		t.Fatalf("未采到今天的笔记，实得: %v", titles)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条今日笔记，实得 %d: %+v", len(items), items)
	}

	// 校验摘要跳过了 frontmatter 与 # 前缀，且 Content 保留了笔记全文。
	for _, it := range items {
		if it.Title == "today" {
			if it.Detail != "今日主题" {
				t.Errorf("摘要应为标题文字，实得 %q", it.Detail)
			}
			if !strings.Contains(it.Content, "正文一句话") {
				t.Errorf("Content 应含笔记全文，实得 %q", it.Content)
			}
		}
		if it.Title == "nested" {
			if len(it.Tags) != 1 || it.Tags[0] != "项目" {
				t.Errorf("子目录应成为 tag，实得 %v", it.Tags)
			}
		}
	}
}

func TestObsidianEmptyVault(t *testing.T) {
	items, err := Obsidian{Vault: ""}.Collect(context.Background(), time.Now())
	if err != nil || items != nil {
		t.Fatalf("空 vault 应安静返回 nil,nil，实得 %v, %v", items, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
