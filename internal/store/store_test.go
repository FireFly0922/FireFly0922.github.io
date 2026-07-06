package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"personalweb/internal/contract"
)

func TestSaveWritesThreeArtifacts(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	day := time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC)

	items := []contract.Item{
		{ID: "g1", Source: contract.SourceGit, Title: "c1"},
		{ID: "g2", Source: contract.SourceGit, Title: "c2"},
		{ID: "n1", Source: contract.SourceObsidian, Title: "note"},
	}
	if _, err := st.Save(day, items, "# 日报\n变化 X"); err != nil {
		t.Fatal(err)
	}

	// days/*.json 存在且内容正确
	dayJSON := filepath.Join(dir, "data", "days", "2025-07-05.json")
	var dd contract.DayData
	readJSON(t, dayJSON, &dd)
	if len(dd.Commits) != 2 || len(dd.Notes) != 1 {
		t.Fatalf("day json 分组错误: %+v", dd)
	}

	// reports/*.md 存在
	if b, err := os.ReadFile(filepath.Join(dir, "reports", "2025-07-05.md")); err != nil || string(b) != "# 日报\n变化 X" {
		t.Fatalf("report md 错误: %q err=%v", string(b), err)
	}

	// index.json 计数与 level 正确
	var idx []contract.IndexEntry
	readJSON(t, filepath.Join(dir, "data", "index.json"), &idx)
	if len(idx) != 1 || idx[0].Counts.Commits != 2 || idx[0].Counts.Notes != 1 {
		t.Fatalf("index 计数错误: %+v", idx)
	}
	if idx[0].Level != contract.LevelFor(3) {
		t.Fatalf("index level=%d want %d", idx[0].Level, contract.LevelFor(3))
	}
}

func TestSaveUpsertsIndexByDate(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)

	d1 := time.Date(2025, 7, 4, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC)

	// 同一天写两次，应覆盖而非追加。
	_, _ = st.Save(d1, []contract.Item{{ID: "a", Source: contract.SourceGit}}, "r1")
	_, _ = st.Save(d2, []contract.Item{{ID: "b", Source: contract.SourceGit}}, "r2")
	_, _ = st.Save(d2, []contract.Item{
		{ID: "b", Source: contract.SourceGit},
		{ID: "c", Source: contract.SourceGit},
	}, "r2-updated")

	var idx []contract.IndexEntry
	readJSON(t, filepath.Join(dir, "data", "index.json"), &idx)

	if len(idx) != 2 {
		t.Fatalf("应有 2 天，实得 %d: %+v", len(idx), idx)
	}
	// 按日期升序
	if idx[0].Date != "2025-07-04" || idx[1].Date != "2025-07-05" {
		t.Fatalf("未按日期升序: %+v", idx)
	}
	// 2025-07-05 被覆盖为 2 条
	if idx[1].Counts.Commits != 2 {
		t.Fatalf("upsert 覆盖失败: %+v", idx[1])
	}
}

func TestRebuildArticlesDedupsNotes(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	d1 := time.Date(2025, 7, 4, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC)

	// 同一篇笔记（id 相同）两天各上传一次，内容更新；另有一篇不同笔记 + 一条 git（不该进博客）。
	_, _ = st.Save(d1, []contract.Item{
		{ID: "obs-a.md", Source: contract.SourceObsidian, Title: "A", Detail: "旧摘要", Content: "旧正文", Tags: []string{"go"}},
	}, "r1")
	_, _ = st.Save(d2, []contract.Item{
		{ID: "obs-a.md", Source: contract.SourceObsidian, Title: "A", Detail: "新摘要", Content: "新正文", Tags: []string{"go"}},
		{ID: "obs-b.md", Source: contract.SourceObsidian, Title: "B", Detail: "b", Content: "b 正文"},
		{ID: "git-x", Source: contract.SourceGit, Title: "commit", Detail: "d"},
	}, "r2")

	var arts []contract.Article
	readJSON(t, filepath.Join(dir, "data", "articles.json"), &arts)

	if len(arts) != 2 {
		t.Fatalf("应有 2 篇笔记文章（git 不算），实得 %d: %+v", len(arts), arts)
	}
	// 日期倒序：b(0705) 与 a(0705) 都在，a 取最新版
	byID := map[string]contract.Article{}
	for _, a := range arts {
		byID[a.ID] = a
	}
	a := byID["obs-a.md"]
	if a.Content != "新正文" || a.Snippet != "新摘要" || a.Date != "2025-07-05" {
		t.Fatalf("笔记 a 未取最新版: %+v", a)
	}
	if _, ok := byID["git-x"]; ok {
		t.Fatal("git 条目不应进博客")
	}
}

func TestAddMusingUpserts(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)

	if err := st.AddMusing(contract.Musing{ID: "2025-07-04-a", Title: "A", Category: "书评", Date: "2025-07-04", Content: "旧"}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddMusing(contract.Musing{ID: "2025-07-05-b", Title: "B", Category: "乐评", Date: "2025-07-05", Content: "b"}); err != nil {
		t.Fatal(err)
	}
	// 同 ID 再发 → 覆盖，不新增
	if err := st.AddMusing(contract.Musing{ID: "2025-07-04-a", Title: "A", Category: "书评", Date: "2025-07-04", Content: "新"}); err != nil {
		t.Fatal(err)
	}

	var ms []contract.Musing
	readJSON(t, filepath.Join(dir, "data", "musings.json"), &ms)
	if len(ms) != 2 {
		t.Fatalf("应有 2 条随笔，实得 %d: %+v", len(ms), ms)
	}
	// 日期倒序：b(0705) 在前
	if ms[0].ID != "2025-07-05-b" {
		t.Fatalf("未按日期倒序: %+v", ms)
	}
	// a 被覆盖为「新」
	for _, m := range ms {
		if m.ID == "2025-07-04-a" && m.Content != "新" {
			t.Fatalf("upsert 覆盖失败: %+v", m)
		}
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s: %v", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("解析 %s: %v", path, err)
	}
}
