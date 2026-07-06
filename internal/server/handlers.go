package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"

	"personalweb/internal/collect"
	"personalweb/internal/contract"
)

// handleCollect 跑三个读取器并发采集今日条目，暂存到会话状态。
func (s *Server) handleCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只接受 POST", http.StatusMethodNotAllowed)
		return
	}
	today := s.today()
	items, err := collect.CollectAll(r.Context(), today, s.Collectors...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.mu.Lock()
	s.collected = items
	s.report = ""
	s.mu.Unlock()

	writeJSON(w, map[string]any{
		"date":  today.Format("2006-01-02"),
		"items": items,
	})
}

type summarizeReq struct {
	IDs []string `json:"ids"` // 勾选的 Item.ID
}

// handleSummarize 取勾选的条目 → 跑 agent → 返回日报预览，并暂存。
func (s *Server) handleSummarize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只接受 POST", http.StatusMethodNotAllowed)
		return
	}
	var req summarizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}

	s.mu.Lock()
	selected := filterByID(s.collected, req.IDs)
	s.mu.Unlock()

	if len(selected) == 0 {
		writeErr(w, http.StatusBadRequest, "没有勾选任何条目")
		return
	}

	today := s.today()
	report, err := s.Agent.Report(r.Context(), today, selected)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "生成日报失败: "+err.Error())
		return
	}

	s.mu.Lock()
	s.report = report
	s.mu.Unlock()

	writeJSON(w, map[string]any{"report": report})
}

type uploadReq struct {
	IDs []string `json:"ids"`
}

// handleUpload 写快照（days/*.json + reports/*.md + index.json）并 commit/push。
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只接受 POST", http.StatusMethodNotAllowed)
		return
	}
	var req uploadReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}

	s.mu.Lock()
	selected := filterByID(s.collected, req.IDs)
	report := s.report
	s.mu.Unlock()

	if len(selected) == 0 {
		writeErr(w, http.StatusBadRequest, "没有勾选任何条目")
		return
	}
	if report == "" {
		writeErr(w, http.StatusBadRequest, "请先点「总结」生成日报再上传")
		return
	}

	today := s.today()
	if _, err := s.Store.Save(today, selected, report); err != nil {
		writeErr(w, http.StatusInternalServerError, "写快照失败: "+err.Error())
		return
	}

	res, err := s.Pusher.Run(r.Context(), today)
	if err != nil {
		// 快照已落盘，push 失败只作为提示，不算整体失败。
		writeJSON(w, map[string]any{
			"saved":   true,
			"push":    res,
			"warning": err.Error(),
		})
		return
	}
	writeJSON(w, map[string]any{"saved": true, "push": res})
}

type musingReq struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Content  string `json:"content"`
}

// handleMusing 接收手写随笔，写进 musings.json 并 commit/push（关于我页展示）。
func (s *Server) handleMusing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只接受 POST", http.StatusMethodNotAllowed)
		return
	}
	var req musingReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" || req.Content == "" {
		writeErr(w, http.StatusBadRequest, "标题与正文不能为空")
		return
	}
	if req.Category == "" {
		req.Category = "想法"
	}

	today := s.today()
	date := today.Format("2006-01-02")
	m := contract.Musing{
		ID:       date + "-" + slug(req.Title),
		Title:    req.Title,
		Category: req.Category,
		Date:     date,
		Content:  req.Content,
	}
	if err := s.Store.AddMusing(m); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存随笔失败: "+err.Error())
		return
	}

	res, err := s.Pusher.Run(r.Context(), today)
	if err != nil {
		writeJSON(w, map[string]any{"saved": true, "id": m.ID, "push": res, "warning": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"saved": true, "id": m.ID, "push": res})
}

// slug 把标题转成 url 友好的 id 片段：小写、空白转 -、去掉标点，保留字母/数字/汉字。
func slug(title string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', unicode.Is(unicode.Han, r):
			b.WriteRune(r)
			prevDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "untitled"
	}
	return out
}

// filterByID 按勾选 id 从采集结果里筛出条目（保持原顺序）。
func filterByID(items []contract.Item, ids []string) []contract.Item {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var out []contract.Item
	for _, it := range items {
		if want[it.ID] {
			out = append(out, it)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
