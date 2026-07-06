// Package contract 定义本地半与网页半之间唯一的数据契约。
// 契约一旦定死，采集端（Go）与前端（静态站）即可并行开工。
// 对应设计文档 §4：site/data/days/*.json、site/data/index.json、site/reports/*.md。
package contract

import "sort"

// Source 标识一条痕迹来自哪个源。
type Source string

const (
	SourceGit      Source = "git"
	SourceObsidian Source = "obsidian"
	SourceZotero   Source = "zotero"
)

// Item 是采集到的一条待勾选条目（三源统一形状）。
type Item struct {
	ID      string   `json:"id"`                // 稳定 id：勾选、去重用
	Source  Source   `json:"source"`            // git / obsidian / zotero
	Title   string   `json:"title"`             // commit message / 笔记标题 / 文献题名
	Detail  string   `json:"detail"`            // diff 摘要 / 笔记摘要 / 阅读片段
	Content string   `json:"content,omitempty"` // 全文：笔记正文 / 完整提交信息 / 文献详情（供详情页展示）
	URL     string   `json:"url,omitempty"`     // 外链：git 提交跳 GitHub（有远端时），详情页优先展示
	Tags    []string `json:"tags,omitempty"`    // 可选标签
}

// DayData 是写进 site/data/days/YYYY-MM-DD.json 的当天快照。
type DayData struct {
	Date    string `json:"date"` // 参考时间格式 2006-01-02
	Commits []Item `json:"commits"`
	Notes   []Item `json:"notes"`
	Papers  []Item `json:"papers"`
	Report  string `json:"report"` // 日报 markdown，与 reports/*.md 同源
}

// Counts 是某天三源的条目计数。
type Counts struct {
	Commits int `json:"commits"`
	Notes   int `json:"notes"`
	Papers  int `json:"papers"`
}

func (c Counts) Total() int { return c.Commits + c.Notes + c.Papers }

// IndexEntry 是 index.json 的一行，喂给热力图 / 仪表盘。
type IndexEntry struct {
	Date   string `json:"date"`
	Counts Counts `json:"counts"`
	Level  int    `json:"level"` // 0..4，映射 muelsyse 主题 --lv0..--lv4 的热力色阶
}

// Article 是一篇笔记文章（跨天去重后的最新版），喂给「文章」博客页。
type Article struct {
	ID      string   `json:"id"` // = 笔记 Item.ID（obs-<相对路径>），稳定标识
	Title   string   `json:"title"`
	Tags    []string `json:"tags,omitempty"`
	Date    string   `json:"date"`    // 最近一次上传日期
	Snippet string   `json:"snippet"` // 列表展示用（取 Item.Detail）
	Content string   `json:"content"` // 全文：供阅读页渲染 + 前端全文搜索
}

// Musing 是手写的随笔（乐评 / 书评 / 播客 / 想法），喂给「关于我」页。
type Musing struct {
	ID       string `json:"id"` // date + "-" + slug(title)，稳定可读
	Title    string `json:"title"`
	Category string `json:"category"` // 乐评 / 书评 / 播客 / 想法 / 其他
	Date     string `json:"date"`
	Content  string `json:"content"` // markdown 正文
}

// LevelFor 把当天总活动量映射到热力色阶 0..4。
func LevelFor(total int) int {
	switch {
	case total <= 0:
		return 0
	case total <= 2:
		return 1
	case total <= 5:
		return 2
	case total <= 9:
		return 3
	default:
		return 4
	}
}

// BuildDayData 按 Source 把混合 Items 分组，组装成当天快照。
// report 是 agent 产出的对比日报（可为空）。
func BuildDayData(date string, items []Item, report string) DayData {
	d := DayData{Date: date, Report: report}
	for _, it := range items {
		switch it.Source {
		case SourceGit:
			d.Commits = append(d.Commits, it)
		case SourceObsidian:
			d.Notes = append(d.Notes, it)
		case SourceZotero:
			d.Papers = append(d.Papers, it)
		}
	}
	return d
}

// CountsOf 统计一天快照的三源条目数。
func (d DayData) CountsOf() Counts {
	return Counts{
		Commits: len(d.Commits),
		Notes:   len(d.Notes),
		Papers:  len(d.Papers),
	}
}

// IndexEntryOf 由当天快照生成 index.json 的一行。
func (d DayData) IndexEntryOf() IndexEntry {
	c := d.CountsOf()
	return IndexEntry{Date: d.Date, Counts: c, Level: LevelFor(c.Total())}
}

// SortItems 让条目按 Source 再按 ID 稳定排序，抹平并发采集带来的乱序。
func SortItems(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Source != items[j].Source {
			return items[i].Source < items[j].Source
		}
		return items[i].ID < items[j].ID
	})
}
