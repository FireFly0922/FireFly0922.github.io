// Package server 提供本地 localhost 勾选页 —— 唯一有真实逻辑的前端。
//
// 设计文档 §3：列出今天采集到的条目 + 勾选框，提供「总结」「上传」两个按钮。
// 确认和总结都发生在这里，因为数据和 API key 都在本地最安全。
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"personalweb/internal/agent"
	"personalweb/internal/collect"
	"personalweb/internal/contract"
	"personalweb/internal/push"
	"personalweb/internal/store"
)

//go:embed web
var webFS embed.FS

// Server 持有本地半的全部依赖与一次会话的临时状态。
type Server struct {
	Collectors []collect.Collector
	Agent      *agent.Agent
	Store      *store.Store
	Pusher     *push.Pusher

	// Now 返回“今天”，测试可注入固定时间。
	Now func() time.Time

	mu        sync.Mutex
	collected []contract.Item // 最近一次采集的条目
	report    string          // 最近一次总结的日报
}

// New 组装一个 Server。
func New(collectors []collect.Collector, ag *agent.Agent, st *store.Store, pusher *push.Pusher) *Server {
	return &Server{
		Collectors: collectors,
		Agent:      ag,
		Store:      st,
		Pusher:     pusher,
		Now:        time.Now,
	}
}

// Handler 返回装好路由的 http.Handler。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// 内嵌前端：把 embed 的 web/ 子目录挂到根路径。
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", noCache(http.FileServer(http.FS(sub))))

	mux.HandleFunc("/api/collect", s.handleCollect)
	mux.HandleFunc("/api/summarize", s.handleSummarize)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/musing", s.handleMusing)

	// 本地预览公网站：把 site 目录挂到 /site/，
	// 页面里 fetch("./data/index.json") 即解析为 /site/data/index.json。
	// （线上部署时 site/ 是纯静态托管，这条路由只为本地看效果。）
	siteDir := s.Store.SiteDir
	mux.Handle("/site/", noCache(http.StripPrefix("/site/", http.FileServer(http.Dir(siteDir)))))

	return mux
}

// noCache 禁用浏览器缓存：本地预览时改了 js/css 刷新即生效，
// 避免「文件名没变→浏览器用旧缓存→看不到改动」的困惑。
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		h.ServeHTTP(w, r)
	})
}

func (s *Server) today() time.Time { return s.Now() }
