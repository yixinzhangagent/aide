package gitx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockRunner 记录被调用的参数并返回预设结果。
type mockRunner struct {
	calls [][]string
	out   string
	err   error
}

func (m *mockRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	m.calls = append(m.calls, args)
	return m.out, m.err
}

func TestStagedDiff(t *testing.T) {
	m := &mockRunner{out: "diff-content"}
	r := &Repo{dir: ".", runner: m}
	out, err := r.StagedDiff(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out != "diff-content" {
		t.Errorf("out = %q", out)
	}
	joined := strings.Join(m.calls[0], " ")
	if !strings.Contains(joined, "diff --staged") {
		t.Errorf("参数中应包含 diff --staged，得到 %q", joined)
	}
}

func TestUnstagedDiff(t *testing.T) {
	m := &mockRunner{}
	r := &Repo{dir: ".", runner: m}
	if _, err := r.UnstagedDiff(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(m.calls[0], " "), "diff") {
		t.Errorf("应调用 git diff，得到 %v", m.calls[0])
	}
}

func TestHasStagedChanges(t *testing.T) {
	m := &mockRunner{out: "file1.go\nfile2.go\n"}
	r := &Repo{dir: ".", runner: m}
	has, err := r.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("存在文件名时应返回 true")
	}

	m2 := &mockRunner{out: "  \n"}
	r2 := &Repo{dir: ".", runner: m2}
	has2, err := r2.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if has2 {
		t.Error("空白输出应返回 false")
	}
}

func TestCommitEmptyMessage(t *testing.T) {
	m := &mockRunner{}
	r := &Repo{dir: ".", runner: m}
	if err := r.Commit(context.Background(), "  "); err == nil {
		t.Fatal("空提交信息应报错")
	}
	if len(m.calls) != 0 {
		t.Error("空信息时不应真正调用 git")
	}
}

func TestCommitPropagatesError(t *testing.T) {
	m := &mockRunner{err: fmt.Errorf("git failed")}
	r := &Repo{dir: ".", runner: m}
	if err := r.Commit(context.Background(), "msg"); err == nil {
		t.Fatal("runner 出错时应向上传播")
	}
}

// runGit 在真实临时仓库中执行 git 命令，失败即终止测试。
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	r := ExecRunner{}
	if _, err := r.Run(context.Background(), dir, args...); err != nil {
		t.Fatalf("git %v 失败: %v", args, err)
	}
}

// TestRealTempRepo 用真实的临时 git 仓库验证完整流程。
func TestRealTempRepo(t *testing.T) {
	dir := t.TempDir()
	// 初始化仓库并设置本地身份（不依赖用户全局配置）。
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")

	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "initial")

	// 修改并暂存，验证 StagedDiff / HasStagedChanges。
	if err := os.WriteFile(file, []byte("hello world\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")

	repo := NewRepo(dir)
	has, err := repo.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("修改并暂存后应有 staged 改动")
	}
	diff, err := repo.StagedDiff(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "hello world") {
		t.Errorf("staged diff 应包含新内容，得到 %q", diff)
	}

	// 用真实提交收尾。
	if err := repo.Commit(context.Background(), "update a"); err != nil {
		t.Fatalf("Commit 失败: %v", err)
	}
	hasAfter, err := repo.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hasAfter {
		t.Error("提交后不应再有 staged 改动")
	}
}
