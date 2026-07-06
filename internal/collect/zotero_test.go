package collect

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// 构造一个最小的 zotero.sqlite（只含查询用到的表），塞入一条今天更新的文献。
func makeZoteroDB(t *testing.T, todayUTC, oldUTC string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "zotero.sqlite")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE fields(fieldID INTEGER PRIMARY KEY, fieldName TEXT)`,
		`CREATE TABLE itemTypes(itemTypeID INTEGER PRIMARY KEY, typeName TEXT)`,
		`CREATE TABLE items(itemID INTEGER PRIMARY KEY, itemTypeID INTEGER, key TEXT, dateAdded TEXT, dateModified TEXT)`,
		`CREATE TABLE itemData(itemID INTEGER, fieldID INTEGER, valueID INTEGER)`,
		`CREATE TABLE itemDataValues(valueID INTEGER PRIMARY KEY, value TEXT)`,
		`CREATE TABLE deletedItems(itemID INTEGER)`,
		`INSERT INTO fields VALUES (1,'title'),(2,'abstractNote')`,
		`INSERT INTO itemTypes VALUES (1,'journalArticle'),(2,'attachment')`,
		// 今天更新的文献（应采到），含标题与摘要
		`INSERT INTO items VALUES (10,1,'AAAA','` + todayUTC + `','` + todayUTC + `')`,
		`INSERT INTO itemDataValues VALUES (100,'Attention Is All You Need'),(101,'A transformer abstract.')`,
		`INSERT INTO itemData VALUES (10,1,100),(10,2,101)`,
		// 附件（应被 typeName 过滤掉）
		`INSERT INTO items VALUES (11,2,'BBBB','` + todayUTC + `','` + todayUTC + `')`,
		// 两天前的文献（应被日期过滤掉）
		`INSERT INTO items VALUES (12,1,'CCCC','` + oldUTC + `','` + oldUTC + `')`,
		`INSERT INTO itemDataValues VALUES (120,'Old Paper')`,
		`INSERT INTO itemData VALUES (12,1,120)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("建库 %q: %v", s, err)
		}
	}
	return path
}

func TestZoteroCollectsTodayItems(t *testing.T) {
	now := time.Now()
	todayUTC := now.UTC().Format("2006-01-02 15:04:05")
	oldUTC := now.AddDate(0, 0, -2).UTC().Format("2006-01-02 15:04:05")

	path := makeZoteroDB(t, todayUTC, oldUTC)

	items, err := Zotero{DB: path}.Collect(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 条今日文献，实得 %d: %+v", len(items), items)
	}
	if items[0].Title != "Attention Is All You Need" {
		t.Errorf("标题不符: %q", items[0].Title)
	}
	if len(items[0].Tags) != 1 || items[0].Tags[0] != "journalArticle" {
		t.Errorf("应以文献类型为 tag，实得 %v", items[0].Tags)
	}
	if !strings.Contains(items[0].Content, "A transformer abstract.") {
		t.Errorf("Content 应含摘要，实得 %q", items[0].Content)
	}
}

func TestZoteroEmptyPath(t *testing.T) {
	items, err := Zotero{DB: ""}.Collect(context.Background(), time.Now())
	if err != nil || items != nil {
		t.Fatalf("空路径应安静返回 nil,nil，实得 %v, %v", items, err)
	}
}
