package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yixinzhangagent/aide/internal/llm"
)

// fakeClient 按预设顺序返回响应，不足时重复最后一个；并记录所有请求。
type fakeClient struct {
	responses []*llm.ChatResponse
	requests  []*llm.ChatRequest
}

func (f *fakeClient) Chat(_ context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	f.requests = append(f.requests, req)
	idx := len(f.requests) - 1
	if idx < len(f.responses) {
		return f.responses[idx], nil
	}
	return f.responses[len(f.responses)-1], nil
}

func (f *fakeClient) ChatStream(_ context.Context, _ *llm.ChatRequest, _ func(llm.ChatChunk)) error {
	return nil
}

func toolCallResp(name, argsJSON string) *llm.ChatResponse {
	return &llm.ChatResponse{Choices: []llm.Choice{{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID:       "call_1",
				Type:     "function",
				Function: llm.FunctionCall{Name: name, Arguments: argsJSON},
			}},
		},
		FinishReason: "tool_calls",
	}}}
}

func finalResp(text string) *llm.ChatResponse {
	return &llm.ChatResponse{Choices: []llm.Choice{{
		Message:      llm.Message{Role: llm.RoleAssistant, Content: text},
		FinishReason: "stop",
	}}}
}

// ---- 路径安全 ----

func TestSafeJoinAllowed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := safeJoin(root, "a.txt")
	if err != nil {
		t.Fatalf("合法路径不应报错: %v", err)
	}
	if filepath.Base(got) != "a.txt" {
		t.Errorf("got = %q", got)
	}
}

func TestSafeJoinTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := safeJoin(root, "../etc/passwd"); err == nil {
		t.Fatal("目录穿越应被拒绝")
	}
}

func TestSafeJoinAbsoluteOutside(t *testing.T) {
	root := t.TempDir()
	if _, err := safeJoin(root, "/etc/passwd"); err == nil {
		t.Fatal("指向 root 外部的绝对路径应被拒绝")
	}
}

func TestSafeJoinSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := safeJoin(root, "link"); err == nil {
		t.Fatal("指向外部的符号链接应被拒绝")
	}
}

func TestSafeJoinNonexistent(t *testing.T) {
	if _, err := safeJoin(t.TempDir(), "no-file"); err == nil {
		t.Fatal("不存在的路径应报错")
	}
}

// ---- read_file ----

func TestReadFileTool(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := NewReadFileTool(root, 1024)
	out, err := tool.Execute(context.Background(), `{"path":"f.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "hi" {
		t.Errorf("out = %q", out)
	}
}

func TestReadFileToolBadArgs(t *testing.T) {
	tool := NewReadFileTool(t.TempDir(), 1024)
	if _, err := tool.Execute(context.Background(), `{bad`); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
	if _, err := tool.Execute(context.Background(), `{"path":""}`); err == nil {
		t.Fatal("空 path 应报错")
	}
}

func TestReadFileToolDir(t *testing.T) {
	tool := NewReadFileTool(t.TempDir(), 1024)
	if _, err := tool.Execute(context.Background(), `{"path":"."}`); err == nil {
		t.Fatal("读取目录应报错")
	}
}

func TestReadFileToolTooLarge(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big"), make([]byte, 100), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := NewReadFileTool(root, 10)
	if _, err := tool.Execute(context.Background(), `{"path":"big"}`); err == nil {
		t.Fatal("超限文件应报错")
	}
}

// ---- list_dir ----

func TestListDirTool(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := NewListDirTool(root)
	out, err := tool.Execute(context.Background(), `{"path":"."}`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "sub/") || !contains(out, "a.txt") {
		t.Errorf("应包含 sub/ 与 a.txt，得到 %q", out)
	}
	if contains(out, "hidden") {
		t.Errorf("隐藏项不应显示，得到 %q", out)
	}
}

func TestListDirToolNotDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := NewListDirTool(root)
	if _, err := tool.Execute(context.Background(), `{"path":"f"}`); err == nil {
		t.Fatal("对文件调用 list_dir 应报错")
	}
}

// ---- Agent 编排 ----

func TestAgentPlainChat(t *testing.T) {
	fc := &fakeClient{responses: []*llm.ChatResponse{finalResp("你好")}}
	a := New(fc, "m")
	out, err := a.Run(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if out != "你好" {
		t.Errorf("out = %q", out)
	}
}

func TestAgentToolLoop(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	fc := &fakeClient{responses: []*llm.ChatResponse{
		toolCallResp("read_file", `{"path":"f.txt"}`),
		finalResp("文件内容是 data"),
	}}
	a := New(fc, "m", WithTools(NewReadFileTool(root, 1024)))
	out, err := a.Run(context.Background(), "读一下 f.txt")
	if err != nil {
		t.Fatal(err)
	}
	if out != "文件内容是 data" {
		t.Errorf("out = %q", out)
	}
	// 历史中应包含一条 tool 结果消息。
	var hasTool bool
	for _, m := range a.History() {
		if m.Role == llm.RoleTool && m.Content == "data" {
			hasTool = true
		}
	}
	if !hasTool {
		t.Fatal("历史中应包含工具结果 data")
	}
}

func TestAgentUnknownTool(t *testing.T) {
	fc := &fakeClient{responses: []*llm.ChatResponse{
		toolCallResp("nope", `{}`),
		finalResp("我无法使用该工具"),
	}}
	a := New(fc, "m", WithTools(NewReadFileTool(t.TempDir(), 1024)))
	out, err := a.Run(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if out != "我无法使用该工具" {
		t.Errorf("out = %q", out)
	}
	var hasError bool
	for _, m := range a.History() {
		if m.Role == llm.RoleTool && contains(m.Content, "没有名为") {
			hasError = true
		}
	}
	if !hasError {
		t.Fatal("未知工具应回传错误信息")
	}
}

func TestAgentMaxTurns(t *testing.T) {
	// 模型持续请求工具，永远不给最终答案 → 达到上限报错。
	fc := &fakeClient{responses: []*llm.ChatResponse{
		toolCallResp("read_file", `{"path":"f.txt"}`),
	}}
	a := New(fc, "m", WithMaxToolTurns(2), WithTools(NewReadFileTool(t.TempDir(), 1024)))
	if _, err := a.Run(context.Background(), "x"); err == nil {
		t.Fatal("达到最大工具轮数应报错")
	}
}

func TestToolDefinitions(t *testing.T) {
	root := t.TempDir()
	rf := NewReadFileTool(root, 10)
	ld := NewListDirTool(root)
	if rf.Name() != "read_file" || rf.Definition().Function.Name != "read_file" {
		t.Error("read_file 定义不正确")
	}
	if ld.Name() != "list_dir" || ld.Definition().Function.Name != "list_dir" {
		t.Error("list_dir 定义不正确")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
