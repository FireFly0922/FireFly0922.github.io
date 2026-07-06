package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestYesterdayReport_Exists(t *testing.T) {
	dir := t.TempDir()
	today := time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC)
	yesterday := "2025-07-04"
	want := "# 昨天的日报\n继续推进 X。"
	if err := os.WriteFile(filepath.Join(dir, yesterday+".md"), []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := YesterdayReport{Dir: dir}.Context(context.Background(), today)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("读到 %q, want %q", got, want)
	}
}

func TestYesterdayReport_FirstDay(t *testing.T) {
	// 昨天没有记录时必须优雅返回，不能报错（第一天场景）。
	dir := t.TempDir()
	today := time.Date(2025, 7, 5, 0, 0, 0, 0, time.UTC)

	got, err := YesterdayReport{Dir: dir}.Context(context.Background(), today)
	if err != nil {
		t.Fatalf("第一天不应报错: %v", err)
	}
	if got != "（昨天没有记录）" {
		t.Fatalf("第一天返回 %q", got)
	}
}
