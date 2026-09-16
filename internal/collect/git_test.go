package collect

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGitCollectsTodayCommits(t *testing.T) {
	repo := t.TempDir()
	ctx := context.Background()

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "tester")
	git("config", "commit.gpgsign", "false") // 避免全局签名配置导致提交失败

	writeFile(t, filepath.Join(repo, "a.txt"), "hello")
	git("add", ".")
	git("commit", "-m", "feat: 今天的提交")

	items, err := Git{Repos: []string{repo}}.Collect(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 条今日提交，实得 %d: %+v", len(items), items)
	}
	if items[0].Title != "feat: 今天的提交" {
		t.Errorf("提交标题不符: %q", items[0].Title)
	}
	repoName := filepath.Base(repo)
	if len(items[0].Tags) != 1 || items[0].Tags[0] != repoName {
		t.Errorf("应以仓库名为 tag，实得 %v", items[0].Tags)
	}
}

func TestNormalizeGitURL(t *testing.T) {
	cases := map[string]string{
		"git@github.com:user/repo.git":       "https://github.com/user/repo",
		"https://github.com/user/repo.git":   "https://github.com/user/repo",
		"https://github.com/user/repo":       "https://github.com/user/repo",
		"ssh://git@github.com/user/repo.git": "https://github.com/user/repo",
		"git@gitlab.com:group/proj.git":      "https://gitlab.com/group/proj",
		"":                                   "",
	}
	for in, want := range cases {
		if got := normalizeGitURL(in); got != want {
			t.Errorf("normalizeGitURL(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestGitSkipsNonRepo(t *testing.T) {
	// 非 git 目录应被跳过而非报错。
	dir := t.TempDir()
	items, err := Git{Repos: []string{dir}}.Collect(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("非仓库不应报错: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("非仓库应无条目，实得 %d", len(items))
	}
}

func TestGitFiltersOutOthersCommits(t *testing.T) {
	// 共享仓库里别人当天的提交不应混进自己的学习痕迹。
	repo := t.TempDir()
	ctx := context.Background()

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "me@example.com")
	git("config", "user.name", "tester")
	git("config", "commit.gpgsign", "false")

	writeFile(t, filepath.Join(repo, "a.txt"), "mine")
	git("add", ".")
	git("commit", "-m", "feat: 我的提交")

	// 别人的提交（-c 只对这一次命令生效，不改仓库配置）
	writeFile(t, filepath.Join(repo, "b.txt"), "others")
	git("add", ".")
	git("-c", "user.name=other", "-c", "user.email=other@example.com", "commit", "-m", "feat: 别人的提交")

	items, err := Git{Repos: []string{repo}}.Collect(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("应只采自己 1 条提交，实得 %d: %+v", len(items), items)
	}
	if items[0].Title != "feat: 我的提交" {
		t.Errorf("标题不符: %q", items[0].Title)
	}
}

func TestGitFetchPullsRemoteCommits(t *testing.T) {
	// 远端有一笔今天的提交、本地还没拉：Collect 应自动 fetch 并采到它。
	root := t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	bare := filepath.Join(root, "remote.git")
	git(root, "init", "--bare", "--initial-branch=main", bare)

	// clone1：采集的本地仓库，先提交一笔本地的
	clone1 := filepath.Join(root, "clone1")
	git(root, "clone", bare, clone1)
	git(clone1, "config", "user.email", "me@example.com")
	git(clone1, "config", "user.name", "tester")
	git(clone1, "config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(clone1, "a.txt"), "local")
	git(clone1, "add", ".")
	git(clone1, "commit", "-m", "feat: 本地提交")
	git(clone1, "push", "origin", "HEAD:main")

	// clone2 模拟「别的机器/网页」：提交一笔并推到远端，clone1 毫不知情
	clone2 := filepath.Join(root, "clone2")
	git(root, "clone", bare, clone2)
	git(clone2, "config", "user.email", "me@example.com")
	git(clone2, "config", "user.name", "tester")
	git(clone2, "config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(clone2, "b.txt"), "remote")
	git(clone2, "add", ".")
	git(clone2, "commit", "-m", "feat: 远端提交")
	git(clone2, "push", "origin", "HEAD:main")

	// 此时 clone1 本地只见过 1 笔；Collect 应 fetch 到远端那笔（--all 让它可见）
	items, err := Git{Repos: []string{clone1}}.Collect(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 fetch 后采到 2 条（本地 + 远端），实得 %d: %+v", len(items), items)
	}
}
