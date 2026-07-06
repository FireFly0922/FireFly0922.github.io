// Command daily-agent 是本地半的主程序：装配依赖、起 localhost 勾选页。
//
// v1 用假数据读取器 + mock LLM 打通整条链路（采集 → 勾选 → 总结 → 上传）。
// v2 把 stub 换成真实读取器即可，装配代码几乎不动；LLM 已可用 -llm real 切到真实 Anthropic。
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"personalweb/internal/agent"
	"personalweb/internal/collect"
	"personalweb/internal/config"
	"personalweb/internal/llm"
	"personalweb/internal/memory"
	"personalweb/internal/push"
	"personalweb/internal/server"
	"personalweb/internal/store"
)

func main() {
	var (
		addr    = flag.String("addr", "127.0.0.1:8765", "localhost 勾选页监听地址")
		siteDir = flag.String("site", "./site", "站点数据目录（days/reports/index.json 的根）")
		repoDir = flag.String("repo", ".", "git 仓库根目录（上传时 commit 的位置）")
		model   = flag.String("model", "claude-sonnet-5", "LLM 模型（-llm real 时生效）")
		llmKind = flag.String("llm", "mock", "LLM 后端：mock（假回复，不花钱）| real（真实 Anthropic，需 API key）")
		envFile = flag.String("env", ".env", "存放 ANTHROPIC_API_KEY 的 .env 文件路径")
		doPush  = flag.Bool("push", false, "上传时是否执行 git push（无远端时保持关闭）")

		collectors = flag.String("collectors", "stub", "读取器：stub（假数据）| real（真实 git + obsidian + zotero）")
		gitRepos   = flag.String("git-repos", ".", "real 模式下要采集提交的 git 仓库，逗号分隔")
		vault      = flag.String("vault", "", "real 模式下 Obsidian vault 路径（如 C:\\Users\\you\\Documents\\Obsidian Vault）")
		zoteroDB   = flag.String("zotero", "", "real 模式下 zotero.sqlite 路径（如 C:\\Users\\you\\Zotero\\zotero.sqlite）")
	)
	flag.Parse()

	// 从 .env 加载密钥（真实环境变量优先，不覆盖）。
	if err := config.LoadDotEnv(*envFile); err != nil {
		log.Fatalf("加载 %s 失败: %v", *envFile, err)
	}

	st := store.New(*siteDir)
	mem := memory.YesterdayReport{Dir: st.ReportsDir()}

	client := buildLLM(*llmKind, *model)
	ag := agent.New(client, mem, *model)

	pusher := &push.Pusher{
		RepoDir: *repoDir,
		Paths:   []string{"site/data", "site/reports"},
		Push:    *doPush,
	}

	srv := server.New(buildCollectors(*collectors, *gitRepos, *vault, *zoteroDB), ag, st, pusher)

	log.Printf("daily-agent 启动：http://%s  (site=%s, llm=%s, collectors=%s)", *addr, *siteDir, *llmKind, *collectors)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatalf("服务退出: %v", err)
	}
}

// buildCollectors 按 -collectors 选读取器。real 模式接 git + obsidian + zotero 真实源。
// 未配置路径的源会安静返回空，不影响其余源。
func buildCollectors(kind, gitRepos, vault, zoteroDB string) []collect.Collector {
	switch kind {
	case "real":
		var repos []string
		for _, r := range strings.Split(gitRepos, ",") {
			if r = strings.TrimSpace(r); r != "" {
				repos = append(repos, r)
			}
		}
		log.Printf("真实读取器：git=%v obsidian=%q zotero=%q", repos, vault, zoteroDB)
		return []collect.Collector{
			collect.Git{Repos: repos},
			collect.Obsidian{Vault: vault},
			collect.Zotero{DB: zoteroDB},
		}
	case "stub":
		return collect.DefaultStubs()
	default:
		log.Fatalf("未知 -collectors 值 %q（应为 stub 或 real）", kind)
		return nil
	}
}

// buildLLM 按 -llm 选后端。real 缺 key 时直接退出并给出指引。
func buildLLM(kind, model string) llm.Client {
	switch kind {
	case "real":
		// 支持官方 Anthropic 或任意 Anthropic 兼容网关（如 DeepSeek）：
		//   ANTHROPIC_AUTH_TOKEN → Authorization: Bearer（第三方网关常用）
		//   ANTHROPIC_API_KEY    → x-api-key（官方）
		//   ANTHROPIC_BASE_URL   → 自定义端点，默认 https://api.anthropic.com
		//   ANTHROPIC_MODEL      → 覆盖 -model
		token := os.Getenv("ANTHROPIC_AUTH_TOKEN")
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if token == "" && apiKey == "" {
			log.Fatalf("-llm real 需要密钥：在 .env 写入 ANTHROPIC_AUTH_TOKEN=（第三方网关）或 ANTHROPIC_API_KEY=（官方）")
		}
		if m := os.Getenv("ANTHROPIC_MODEL"); m != "" {
			model = m
		}
		client := llm.NewAnthropic(apiKey, model)
		client.AuthToken = token
		if base := os.Getenv("ANTHROPIC_BASE_URL"); base != "" {
			client.BaseURL = base
		}
		authKind := "x-api-key"
		if token != "" {
			authKind = "Bearer"
		}
		log.Printf("LLM 真实客户端：base=%s model=%s auth=%s", client.BaseURL, model, authKind)
		return client
	case "mock":
		return llm.Mock{}
	default:
		log.Fatalf("未知 -llm 值 %q（应为 mock 或 real）", kind)
		return nil
	}
}
