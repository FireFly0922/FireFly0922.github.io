package contract

import (
	"encoding/json"
	"testing"
)

func TestLevelFor(t *testing.T) {
	cases := []struct {
		total int
		want  int
	}{
		{0, 0}, {1, 1}, {2, 1}, {3, 2}, {5, 2}, {6, 3}, {9, 3}, {10, 4}, {100, 4},
	}
	for _, c := range cases {
		if got := LevelFor(c.total); got != c.want {
			t.Errorf("LevelFor(%d)=%d, want %d", c.total, got, c.want)
		}
	}
}

func TestBuildDayDataGroupsBySource(t *testing.T) {
	items := []Item{
		{ID: "g1", Source: SourceGit},
		{ID: "n1", Source: SourceObsidian},
		{ID: "p1", Source: SourceZotero},
		{ID: "g2", Source: SourceGit},
	}
	d := BuildDayData("2025-07-05", items, "report body")

	if len(d.Commits) != 2 || len(d.Notes) != 1 || len(d.Papers) != 1 {
		t.Fatalf("分组错误: commits=%d notes=%d papers=%d", len(d.Commits), len(d.Notes), len(d.Papers))
	}
	if d.Report != "report body" || d.Date != "2025-07-05" {
		t.Fatalf("date/report 未正确设置: %+v", d)
	}

	c := d.CountsOf()
	if c.Total() != 4 {
		t.Fatalf("Total=%d, want 4", c.Total())
	}
	if e := d.IndexEntryOf(); e.Level != LevelFor(4) {
		t.Fatalf("IndexEntry.Level=%d, want %d", e.Level, LevelFor(4))
	}
}

func TestDayDataJSONRoundTrip(t *testing.T) {
	in := BuildDayData("2025-07-05", []Item{
		{ID: "g1", Source: SourceGit, Title: "t", Detail: "d", Tags: []string{"go"}},
	}, "r")

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out DayData
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Commits) != 1 || out.Commits[0].ID != "g1" || out.Report != "r" {
		t.Fatalf("往返后不一致: %+v", out)
	}
}

func TestSortItemsDeterministic(t *testing.T) {
	items := []Item{
		{ID: "z", Source: SourceZotero},
		{ID: "b", Source: SourceGit},
		{ID: "a", Source: SourceGit},
	}
	SortItems(items)
	if items[0].ID != "a" || items[1].ID != "b" || items[2].ID != "z" {
		t.Fatalf("排序不确定: %+v", items)
	}
}
