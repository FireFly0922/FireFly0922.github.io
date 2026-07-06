package collect

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // 纯 Go sqlite 驱动，无需 cgo，可交叉编译到 Windows

	"personalweb/internal/contract"
)

// Zotero 只读 zotero.sqlite，取当天更新过的文献（设计文档 v2）。
// 用 ?mode=ro&immutable=1 打开：只读且假定文件不被改，从而绕开 Zotero 运行时的库锁。
type Zotero struct {
	DB string // zotero.sqlite 路径
}

func (Zotero) Name() contract.Source { return contract.SourceZotero }

func (z Zotero) Collect(ctx context.Context, day time.Time) ([]contract.Item, error) {
	if z.DB == "" {
		return nil, nil // 未配置：安静跳过
	}

	db, err := sql.Open("sqlite", "file:"+z.DB+"?mode=ro&immutable=1")
	if err != nil {
		return nil, fmt.Errorf("打开 zotero.sqlite: %w", err)
	}
	defer db.Close()

	// dateModified 是 UTC 字符串 "YYYY-MM-DD HH:MM:SS"，可按字典序比较。
	// 把「今天」的本地零点/次日零点换算成 UTC 边界，做时区正确的当天筛选。
	since := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	until := since.AddDate(0, 0, 1)
	const layout = "2006-01-02 15:04:05"

	const query = `
SELECT i.key,
       COALESCE(vt.value, '') AS title,
       COALESCE(va.value, '') AS abstract,
       it.typeName,
       i.dateModified
FROM items i
JOIN itemTypes it ON it.itemTypeID = i.itemTypeID
LEFT JOIN itemData dt ON dt.itemID = i.itemID
     AND dt.fieldID = (SELECT fieldID FROM fields WHERE fieldName = 'title')
LEFT JOIN itemDataValues vt ON vt.valueID = dt.valueID
LEFT JOIN itemData da ON da.itemID = i.itemID
     AND da.fieldID = (SELECT fieldID FROM fields WHERE fieldName = 'abstractNote')
LEFT JOIN itemDataValues va ON va.valueID = da.valueID
WHERE i.itemID NOT IN (SELECT itemID FROM deletedItems)
  AND it.typeName NOT IN ('attachment', 'note', 'annotation')
  AND i.dateModified >= ? AND i.dateModified < ?
ORDER BY i.dateModified DESC`

	rows, err := db.QueryContext(ctx, query, since.UTC().Format(layout), until.UTC().Format(layout))
	if err != nil {
		return nil, fmt.Errorf("查询 zotero: %w", err)
	}
	defer rows.Close()

	var items []contract.Item
	for rows.Next() {
		var key, title, abstract, typeName, modified string
		if err := rows.Scan(&key, &title, &abstract, &typeName, &modified); err != nil {
			return nil, err
		}
		if title == "" {
			title = "(无标题) " + typeName
		}
		detail := typeName + " · 更新于 " + modified + " (UTC)"
		// Content：类型/时间 + 摘要（若有），供详情页展示。
		content := detail
		if abstract = strings.TrimSpace(abstract); abstract != "" {
			content += "\n\n" + abstract
		}
		items = append(items, contract.Item{
			ID:      "zot-" + key,
			Source:  contract.SourceZotero,
			Title:   title,
			Detail:  detail,
			Content: content,
			Tags:    []string{typeName},
		})
	}
	return items, rows.Err()
}

var _ Collector = Zotero{}
