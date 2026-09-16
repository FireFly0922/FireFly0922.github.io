package collect

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"personalweb/internal/contract"
)

// Git 读取一批本地仓库当天的提交（设计文档 v2）。
// 用 os/exec 调系统 git：复用已装 git 与凭据，Windows / WSL 都行。
// 采集前会 best-effort fetch，且只采自己邮箱（git config user.email）的提交。
type Git struct {
	Repos []string // git 仓库根目录列表（本地文件夹路径，不是网址）
}

func (Git) Name() contract.Source { return contract.SourceGit }

// 字段分隔符 \x1f、记录分隔符 \x1e —— 正文里几乎不可能出现，解析稳。
const (
	gitFieldSep  = "\x1f"
	gitRecordSep = "\x1e"
	gitFormat    = "%H" + gitFieldSep + "%s" + gitFieldSep + "%b" + gitRecordSep
)

func (g Git) Collect(ctx context.Context, day time.Time) ([]contract.Item, error) {
	since := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	until := since.AddDate(0, 0, 1)

	var items []contract.Item
	for _, repo := range g.Repos {
		gitFetch(ctx, repo) // best-effort：先拉远端，网页/别的机器上的提交才看得到
		author := gitUserEmail(ctx, repo)
		commits, err := gitLog(ctx, repo, since, until, author)
		if err != nil {
			// 该路径不是 git 仓库、或 git 不可用：跳过而非拖垮整条采集。
			continue
		}
		repoName := filepath.Base(strings.TrimRight(repo, `/\`))
		remote := gitRemoteURL(ctx, repo) // 有 GitHub/GitLab 远端时，提交可跳转
		for _, c := range commits {
			body := strings.TrimSpace(c.body)
			detail := body
			if detail == "" {
				detail = "（无正文）"
			}
			// Content：完整提交信息（标题 + 正文 + 短哈希），供详情页展示。
			content := repoName + "@" + shortHash(c.hash) + "\n\n" + c.subject
			if body != "" {
				content += "\n\n" + body
			}
			var url string
			if remote != "" {
				url = remote + "/commit/" + c.hash // 完整哈希，跳到远端提交页
			}
			items = append(items, contract.Item{
				ID:      "git-" + repoName + "-" + shortHash(c.hash),
				Source:  contract.SourceGit,
				Title:   c.subject,
				Detail:  detail,
				Content: content,
				URL:     url,
				Tags:    []string{repoName},
			})
		}
	}
	return items, nil
}

type gitCommit struct {
	hash    string
	subject string
	body    string
}

// gitFetch 尝试拉取仓库远端的新提交，失败静默跳过。
// 有的提交是在 GitHub 网页或别的机器上做的，本地不 fetch 就永远看不到；
// 无远端/离线时命令报错，直接忽略，不影响本地提交的采集。
func gitFetch(ctx context.Context, repo string) {
	const fetchTimeout = 20 * time.Second
	cctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", "-C", repo, "fetch", "--all", "--quiet")
	_ = cmd.Run()
}

// gitUserEmail 取仓库配置的提交者邮箱（含全局配置），未配置返回空串。
func gitUserEmail(ctx context.Context, repo string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "config", "user.email")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitLog 跑 `git -C repo log --since --until`，解析出今天的提交。
// author 非空时只采该邮箱的提交：共享仓库里别人的提交不该算进自己的学习痕迹。
func gitLog(ctx context.Context, repo string, since, until time.Time, author string) ([]gitCommit, error) {
	const layout = "2006-01-02 15:04:05"
	args := []string{"-C", repo, "log",
		"--no-merges",
		"--all", // 含 origin/* 等远端引用：fetch 到的提交无需先合进本地分支也能被看到
		"--since=" + since.Format(layout),
		"--until=" + until.Format(layout),
	}
	if author != "" {
		// --author 是正则，匹配 "Name <email>" 整串；锚定到 <email> 结尾，
		// 避免子串误配（如 a@x.com 配中 ba@x.com）。
		args = append(args, "--author= <"+regexp.QuoteMeta(author)+">$")
	}
	args = append(args, "--pretty=format:"+gitFormat)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []gitCommit
	for _, rec := range strings.Split(string(out), gitRecordSep) {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		fields := strings.SplitN(rec, gitFieldSep, 3)
		if len(fields) < 2 {
			continue
		}
		c := gitCommit{hash: fields[0], subject: fields[1]}
		if len(fields) == 3 {
			c.body = fields[2]
		}
		commits = append(commits, c)
	}
	return commits, nil
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

// gitRemoteURL 取仓库 origin 远端并归一化成 https 网页地址；无远端返回空串。
func gitRemoteURL(ctx context.Context, repo string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return normalizeGitURL(strings.TrimSpace(string(out)))
}

// normalizeGitURL 把各种 git 远端地址转成 https 网页根地址。
//
//	git@github.com:user/repo.git    → https://github.com/user/repo
//	ssh://git@github.com/user/repo   → https://github.com/user/repo
//	https://github.com/user/repo.git → https://github.com/user/repo
func normalizeGitURL(u string) string {
	if u == "" {
		return ""
	}
	u = strings.TrimSuffix(u, ".git")
	switch {
	case strings.HasPrefix(u, "git@"):
		u = strings.TrimPrefix(u, "git@")
		if i := strings.Index(u, ":"); i >= 0 {
			u = "https://" + u[:i] + "/" + u[i+1:]
		}
	case strings.HasPrefix(u, "ssh://"):
		u = strings.TrimPrefix(strings.TrimPrefix(u, "ssh://"), "git@")
		u = "https://" + u
	}
	return u
}

var _ Collector = Git{}
