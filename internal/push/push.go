// Package push 把站点数据快照 commit & push 到仓库。
//
// 设计文档 §4/§8：上传 = git push，git 历史即长期跟踪时间线，无需自造版本管理。
// v1 用 os/exec 调系统 git（Windows 上比 go-git 稳，复用已装 git 与凭据）。
package push

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Pusher 在 RepoDir 里对 Paths 执行 add / commit / push。
type Pusher struct {
	RepoDir string   // git 仓库根目录（monorepo 即项目根）
	Paths   []string // 要提交的路径，如 ["site/data", "site/reports"]
	Push    bool     // 是否执行 git push（无远端时置 false，只本地 commit）
}

// Result 汇报一次上传的结果，供 UI 展示。
type Result struct {
	Committed bool   `json:"committed"`
	Pushed    bool   `json:"pushed"`
	Message   string `json:"message"`
}

// Run 执行 add → commit →（可选）push。
// 对「不是 git 仓库」「无改动可提交」等常见情况给出友好提示而非硬失败。
func (p *Pusher) Run(ctx context.Context, day time.Time) (Result, error) {
	if !p.isGitRepo(ctx) {
		return Result{Message: fmt.Sprintf("%s 不是 git 仓库，跳过上传（可先 git init）", p.RepoDir)}, nil
	}

	paths := p.Paths
	if len(paths) == 0 {
		paths = []string{"site/data", "site/reports"}
	}
	if out, err := p.git(ctx, append([]string{"add"}, paths...)...); err != nil {
		return Result{}, fmt.Errorf("git add: %v (%s)", err, out)
	}

	// 无改动则不提交。
	if diff, _ := p.git(ctx, "diff", "--cached", "--quiet"); diff == "clean" {
		return Result{Message: "没有需要上传的改动"}, nil
	}

	msg := fmt.Sprintf("chore: %s 学习记录", day.Format("2006-01-02"))
	if out, err := p.git(ctx, "commit", "-m", msg); err != nil {
		return Result{}, fmt.Errorf("git commit: %v (%s)", err, out)
	}

	res := Result{Committed: true, Message: "已本地提交：" + msg}

	if p.Push {
		if out, err := p.git(ctx, "push"); err != nil {
			return res, fmt.Errorf("git push: %v (%s)", err, out)
		}
		res.Pushed = true
		res.Message = "已提交并推送：" + msg
	}
	return res, nil
}

func (p *Pusher) isGitRepo(ctx context.Context) bool {
	out, err := p.git(ctx, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// git 在 RepoDir 里运行一条 git 命令，返回合并后的输出。
// 对 `diff --cached --quiet` 这类以退出码表意的命令，用返回值 "clean"/"dirty" 表达。
func (p *Pusher) git(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = p.RepoDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()

	// git diff --cached --quiet：无差异退出 0，有差异退出 1。
	if len(args) >= 2 && args[0] == "diff" {
		if err == nil {
			return "clean", nil
		}
		return "dirty", nil
	}
	return strings.TrimSpace(buf.String()), err
}
