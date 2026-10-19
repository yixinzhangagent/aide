package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yixinzhangagent/aide/internal/config"
	"github.com/yixinzhangagent/aide/internal/llm"
)

type fakeClient struct {
	resp *llm.ChatResponse
	err  error
}

func (f *fakeClient) Chat(context.Context, *llm.ChatRequest) (*llm.ChatResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.resp != nil {
		return f.resp, nil
	}
	return &llm.ChatResponse{Choices: []llm.Choice{{
		Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
	}}}, nil
}

func (f *fakeClient) ChatStream(context.Context, *llm.ChatRequest, func(llm.ChatChunk)) error {
	return nil
}

func newTestServer(t *testing.T, client llm.Client) *httptest.Server {
	t.Helper()
	srv := New(client, config.Default(), "127.0.0.1:0")
	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)
	return ts
}

func TestHealth(t *testing.T) {
	ts := newTestServer(t, &fakeClient{})
	res, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("状态 = %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "ok") {
		t.Errorf("健康检查返回 = %q", body)
	}
}

func TestChatSuccess(t *testing.T) {
	client := &fakeClient{resp: &llm.ChatResponse{Choices: []llm.Choice{{
		Message: llm.Message{Role: llm.RoleAssistant, Content: "你好呀"},
	}}}}
	ts := newTestServer(t, client)
	res, err := http.Post(ts.URL+"/v1/chat", "application/json",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("状态 = %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "你好呀") {
		t.Errorf("响应 = %q", body)
	}
}

func TestChatNoMessages(t *testing.T) {
	ts := newTestServer(t, &fakeClient{})
	res, err := http.Post(ts.URL+"/v1/chat", "application/json",
		strings.NewReader(`{"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("空 messages 应 400，得到 %d", res.StatusCode)
	}
}

func TestChatBadJSON(t *testing.T) {
	ts := newTestServer(t, &fakeClient{})
	res, err := http.Post(ts.URL+"/v1/chat", "application/json",
		strings.NewReader(`{bad`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("非法 JSON 应 400，得到 %d", res.StatusCode)
	}
}

func TestChatBackendError(t *testing.T) {
	ts := newTestServer(t, &fakeClient{err: io.ErrClosedPipe})
	res, err := http.Post(ts.URL+"/v1/chat", "application/json",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("后端错误应 502，得到 %d", res.StatusCode)
	}
}

func TestIndexPage(t *testing.T) {
	ts := newTestServer(t, &fakeClient{})
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("状态 = %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "aide") {
		t.Errorf("首页应包含 aide，得到 %q", string(body))
	}
}
