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
