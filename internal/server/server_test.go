package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"personalweb/internal/agent"
	"personalweb/internal/collect"
	"personalweb/internal/contract"
	"personalweb/internal/llm"
	"personalweb/internal/memory"
	"personalweb/internal/push"
	"personalweb/internal/store"
)

// TestEndToEndDryRun 走一遍 v1 空跑：采集 → 总结 → 上传，断言快照落盘。
func TestEndToEndDryRun(t *testing.T) {
	siteDir := t.TempDir()
	st := store.New(siteDir)
	mem := memory.YesterdayReport{Dir: st.ReportsDir()}
	ag := agent.New(llm.Mock{}, mem, "mock")
	// 非 git 仓库：Pusher 应优雅跳过而非报错。
	pusher := &push.Pusher{RepoDir: siteDir, Push: false}

	srv := New(collect.DefaultStubs(), ag, st, pusher)
	srv.Now = func() time.Time { return time.Date(2025, 7, 5, 12, 0, 0, 0, time.UTC) }

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1) 采集
	var collectResp struct {
		Date  string          `json:"date"`
		Items []contract.Item `json:"items"`
	}
	postJSON(t, ts.URL+"/api/collect", nil, &collectResp)
	if collectResp.Date != "2025-07-05" || len(collectResp.Items) == 0 {
		t.Fatalf("采集结果异常: %+v", collectResp)
	}

	ids := make([]string, len(collectResp.Items))
	for i, it := range collectResp.Items {
		ids[i] = it.ID
	}

	// 2) 总结
	var sumResp struct {
		Report string `json:"report"`
	}
	postJSON(t, ts.URL+"/api/summarize", map[string]any{"ids": ids}, &sumResp)
	if sumResp.Report == "" {
		t.Fatal("总结返回空日报")
	}

	// 3) 上传
	var upResp struct {
		Saved bool `json:"saved"`
		Push  struct {
			Message string `json:"message"`
		} `json:"push"`
	}
	postJSON(t, ts.URL+"/api/upload", map[string]any{"ids": ids}, &upResp)
	if !upResp.Saved {
		t.Fatalf("上传未保存: %+v", upResp)
	}

	// 断言三处产物落盘
	mustExist(t, filepath.Join(siteDir, "data", "days", "2025-07-05.json"))
	mustExist(t, filepath.Join(siteDir, "reports", "2025-07-05.md"))
	mustExist(t, filepath.Join(siteDir, "data", "index.json"))
}

// TestSummarizeRequiresSelection 未勾选时应拒绝。
func TestSummarizeRequiresSelection(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	postJSON(t, ts.URL+"/api/collect", nil, &struct{}{})

	res, err := http.Post(ts.URL+"/api/summarize", "application/json",
		bytes.NewReader([]byte(`{"ids":[]}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("空勾选应返回 400，实得 %d", res.StatusCode)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	siteDir := t.TempDir()
	st := store.New(siteDir)
	mem := memory.YesterdayReport{Dir: st.ReportsDir()}
	ag := agent.New(llm.Mock{}, mem, "mock")
	srv := New(collect.DefaultStubs(), ag, st, &push.Pusher{RepoDir: siteDir})
	srv.Now = func() time.Time { return time.Date(2025, 7, 5, 12, 0, 0, 0, time.UTC) }
	return srv
}

func postJSON(t *testing.T, url string, body, out any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	res, err := http.Post(url, "application/json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(res.Body)
		t.Fatalf("POST %s -> %d: %s", url, res.StatusCode, buf.String())
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("期望文件存在 %s: %v", path, err)
	}
}
