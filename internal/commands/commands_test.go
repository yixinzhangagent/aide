package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yixinzhangagent/aide/internal/config"
	"github.com/yixinzhangagent/aide/internal/gitx"
	"github.com/yixinzhangagent/aide/internal/llm"
)

// fakeClient 是命令层测试用的 LLM 客户端，可分别指定非流式与流式回答。
type fakeClient struct {
	chatContent   string
	streamContent string
	chatErr       error
}

func (f *fakeClient) Chat(context.Context, *llm.ChatRequest) (*llm.ChatResponse, error) {
	if f.chatErr != nil {
		return nil, f.chatErr
	}
	content := f.chatContent
	if content == "" {
		content = "非流式回答"
	}
	return &llm.ChatResponse{Choices: []llm.Choice{{
		Message:      llm.Message{Role: llm.RoleAssistant, Content: content},
		FinishReason: "stop",
	}}}, nil
}

func (f *fakeClient) ChatStream(_ context.Context, _ *llm.ChatRequest, onChunk func(llm.ChatChunk)) error {
	content := f.streamContent
	if content == "" {
		content = "流式回答"
	}
	onChunk(llm.ChatChunk{Choices: []llm.ChunkChoice{{Delta: llm.Delta{Content: content}}}})
	return nil
}

func testDeps(workDir string) (*deps, *bytes.Buffer) {
	cfg := config.Default()
	if workDir != "" {
		cfg.WorkDir = workDir
	}
	buf := &bytes.Buffer{}
	return &deps{cfg: cfg, client: &fakeClient{}, out: buf}, buf
}

// 每个测试里直接构造对应子命令执行。

func TestAskArgs(t *testing.T) {
	d, buf := testDeps("")
	d.client = &fakeClient{streamContent: "答案"}
	cmd := newAskCmd(d)
	cmd.SetArgs([]string{"你好"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "答案") {
		t.Errorf("输出应包含流式答案，得到 %q", buf.String())
	}
}

func TestAskStdin(t *testing.T) {
	d, buf := testDeps("")
	d.client = &fakeClient{streamContent: "管道答案"}
	cmd := newAskCmd(d)
	cmd.SetIn(strings.NewReader("通过管道的问题"))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "管道答案") {
		t.Errorf("输出应包含管道答案，得到 %q", buf.String())
	}
}

func TestAskNoPrompt(t *testing.T) {
	d, _ := testDeps("")
	cmd := newAskCmd(d)
	cmd.SetIn(strings.NewReader("   "))
	if err := cmd.Execute(); err == nil {
		t.Fatal("无问题应报错")
	}
}

func TestExplain(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, buf := testDeps(dir)
	d.client = &fakeClient{chatContent: "这是程序入口"}
	cmd := newExplainCmd(d)
	cmd.SetArgs([]string{"main.go"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "这是程序入口") {
		t.Errorf("输出应包含解释，得到 %q", buf.String())
	}
}

func TestExplainWithLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main(){}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, _ := testDeps(dir)
	d.client = &fakeClient{chatContent: "第2行是主函数"}
	cmd := newExplainCmd(d)
	cmd.SetArgs([]string{"main.go:2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestCommitDryRun(t *testing.T) {
	dir := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")

	d, buf := testDeps(dir)
	d.client = &fakeClient{chatContent: "feat: 新增 a 文件"}
	cmd := newCommitCmd(d)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "feat: 新增 a 文件") {
		t.Errorf("应包含提交信息，得到 %q", buf.String())
	}
	if !strings.Contains(buf.String(), "dry-run") {
		t.Errorf("默认应为 dry-run，得到 %q", buf.String())
	}
	// dry-run 不应真正提交。
	repo := gitx.NewRepo(dir)
	has, err := repo.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("dry-run 后暂存改动应仍在")
	}
}

func TestCommitApply(t *testing.T) {
	dir := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")

	d, _ := testDeps(dir)
	d.client = &fakeClient{chatContent: "feat: 新增 a 文件"}
	cmd := newCommitCmd(d)
	cmd.SetArgs([]string{"--apply"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	repo := gitx.NewRepo(dir)
	has, err := repo.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("--apply 后不应再有暂存改动")
	}
}

func TestCommitNoStaged(t *testing.T) {
	dir := initGitRepo(t)
	d, _ := testDeps(dir)
	cmd := newCommitCmd(d)
	if err := cmd.Execute(); err == nil {
		t.Fatal("无暂存改动应报错")
	}
}

func TestVersionCmd(t *testing.T) {
	cmd := newVersionCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "aide") {
		t.Errorf("版本输出应包含 aide，得到 %q", buf.String())
	}
}

func TestChatPipe(t *testing.T) {
	d, buf := testDeps("")
	d.client = &fakeClient{streamContent: "回复"}
	cmd := newChatCmd(d)
	cmd.SetIn(strings.NewReader("你好\nexit\n"))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "已退出对话") {
		t.Errorf("应正常退出，得到 %q", buf.String())
	}
}

// initGitRepo 创建并初始化一个临时 git 仓库。
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "t@e.com")
	runGit(t, dir, "config", "user.name", "T")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := (gitx.ExecRunner{}).Run(context.Background(), dir, args...); err != nil {
		t.Fatalf("git %v 失败: %v", args, err)
	}
}
