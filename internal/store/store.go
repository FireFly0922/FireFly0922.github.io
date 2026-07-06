// Package store 把当天勾选确认的数据 + 日报写成站点数据快照。
//
// 设计文档 §4「一份数据，两个消费者」：
//   - site/data/days/YYYY-MM-DD.json  当天原始数据（喂日报页）
//   - site/reports/YYYY-MM-DD.md      日报正文（人可读）
//   - site/data/index.json            按天汇总（喂热力图 / 仪表盘）
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"personalweb/internal/contract"
)

// Store 负责把快照写进 site 目录。
type Store struct {
	SiteDir string // 站点根目录，如 ./site
}

func New(siteDir string) *Store { return &Store{SiteDir: siteDir} }

func (s *Store) daysDir() string      { return filepath.Join(s.SiteDir, "data", "days") }
func (s *Store) ReportsDir() string   { return filepath.Join(s.SiteDir, "reports") }
func (s *Store) indexPath() string    { return filepath.Join(s.SiteDir, "data", "index.json") }
func (s *Store) articlesPath() string { return filepath.Join(s.SiteDir, "data", "articles.json") }
func (s *Store) musingsPath() string  { return filepath.Join(s.SiteDir, "data", "musings.json") }

// Save 写当天快照的三处产物，并把 index.json upsert 更新。
// 返回写好的 DayData 供上层展示。
func (s *Store) Save(day time.Time, items []contract.Item, report string) (contract.DayData, error) {
	date := day.Format("2006-01-02")
	dayData := contract.BuildDayData(date, items, report)

	if err := os.MkdirAll(s.daysDir(), 0o755); err != nil {
		return dayData, fmt.Errorf("建 days 目录: %w", err)
	}
	if err := os.MkdirAll(s.ReportsDir(), 0o755); err != nil {
		return dayData, fmt.Errorf("建 reports 目录: %w", err)
	}

	// 1) days/YYYY-MM-DD.json
	if err := writeJSON(filepath.Join(s.daysDir(), date+".json"), dayData); err != nil {
		return dayData, fmt.Errorf("写 day json: %w", err)
	}

	// 2) reports/YYYY-MM-DD.md
	reportPath := filepath.Join(s.ReportsDir(), date+".md")
	if err := os.WriteFile(reportPath, []byte(report), 0o644); err != nil {
		return dayData, fmt.Errorf("写 report md: %w", err)
	}

	// 3) index.json（读-改-写，按日期 upsert）
	if err := s.upsertIndex(dayData.IndexEntryOf()); err != nil {
		return dayData, fmt.Errorf("更新 index: %w", err)
	}

	// 4) articles.json（全量重建：跨天聚合笔记，喂博客页）
	if err := s.rebuildArticles(); err != nil {
		return dayData, fmt.Errorf("重建 articles: %w", err)
	}

	return dayData, nil
}

// rebuildArticles 扫描所有 days/*.json，把 obsidian 笔记按 ID 去重（保留 date 最大的最新版），
// 按日期倒序写进 site/data/articles.json。全量重建最稳，天然处理重复上传与更正。
func (s *Store) rebuildArticles() error {
	ents, err := os.ReadDir(s.daysDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	latest := map[string]contract.Article{} // note id → 最新文章
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.daysDir(), e.Name()))
		if err != nil {
			return err
		}
		var day contract.DayData
		if err := json.Unmarshal(b, &day); err != nil {
			return fmt.Errorf("解析 %s: %w", e.Name(), err)
		}
		for _, note := range day.Notes {
			if prev, ok := latest[note.ID]; ok && prev.Date >= day.Date {
				continue // 已有更新或同样新的版本
			}
			latest[note.ID] = contract.Article{
				ID:      note.ID,
				Title:   note.Title,
				Tags:    note.Tags,
				Date:    day.Date,
				Snippet: note.Detail,
				Content: note.Content,
			}
		}
	}

	articles := make([]contract.Article, 0, len(latest))
	for _, a := range latest {
		articles = append(articles, a)
	}
	sort.Slice(articles, func(i, j int) bool {
		if articles[i].Date != articles[j].Date {
			return articles[i].Date > articles[j].Date // 日期倒序
		}
		return articles[i].ID < articles[j].ID // 同日期按 id 稳定排序
	})

	return writeJSON(s.articlesPath(), articles)
}

// upsertIndex 把一条 IndexEntry 插入/覆盖进 index.json，并按日期升序保存。
func (s *Store) upsertIndex(entry contract.IndexEntry) error {
	entries, err := s.LoadIndex()
	if err != nil {
		return err
	}

	replaced := false
	for i := range entries {
		if entries[i].Date == entry.Date {
			entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Date < entries[j].Date })

	return writeJSON(s.indexPath(), entries)
}

// AddMusing 把一条随笔按 ID upsert 进 musings.json，并按日期倒序保存。
func (s *Store) AddMusing(m contract.Musing) error {
	if err := os.MkdirAll(filepath.Dir(s.musingsPath()), 0o755); err != nil {
		return err
	}
	musings, err := s.LoadMusings()
	if err != nil {
		return err
	}

	replaced := false
	for i := range musings {
		if musings[i].ID == m.ID {
			musings[i] = m
			replaced = true
			break
		}
	}
	if !replaced {
		musings = append(musings, m)
	}
	sort.Slice(musings, func(i, j int) bool {
		if musings[i].Date != musings[j].Date {
			return musings[i].Date > musings[j].Date // 日期倒序
		}
		return musings[i].ID < musings[j].ID
	})

	return writeJSON(s.musingsPath(), musings)
}

// LoadMusings 读 musings.json；文件不存在时返回空切片而非报错。
func (s *Store) LoadMusings() ([]contract.Musing, error) {
	b, err := os.ReadFile(s.musingsPath())
	if os.IsNotExist(err) {
		return []contract.Musing{}, nil
	}
	if err != nil {
		return nil, err
	}
	var musings []contract.Musing
	if err := json.Unmarshal(b, &musings); err != nil {
		return nil, err
	}
	return musings, nil
}

// LoadIndex 读 index.json；文件不存在时返回空切片而非报错。
func (s *Store) LoadIndex() ([]contract.IndexEntry, error) {
	b, err := os.ReadFile(s.indexPath())
	if os.IsNotExist(err) {
		return []contract.IndexEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []contract.IndexEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// writeJSON 以缩进 JSON 落盘。
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
