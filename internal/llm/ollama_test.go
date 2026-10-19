package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// chatResponseJSON 是一个标准的非流式成功响应。
const chatResponseJSON = `{
  "id": "chatcmpl-1",
  "model": "m",
  "choices": [
    {"index": 0, "message": {"role": "assistant", "content": "hello"}, "finish_reason": "stop"}
  ]
}`

func TestChatSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("请求路径 = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, chatResponseJSON)
	}))
	defer srv.Close()

	client := NewOpenAIClient(srv.URL, "", time.Second)
	resp, err := client.Chat(context.Background(), &ChatRequest{Model: "m", Messages: []Message{UserText("hi")}})
	if err != nil {
		t.Fatalf("Chat 返回错误: %v", err)
	}
	if resp.Choices[0].Message.Content != "hello" {
		t.Errorf("内容 = %q", resp.Choices[0].Message.Content)
	}
}

func TestChatRequestBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("服务端无法解析请求: %v", err)
		}
		if req.Model != "the-model" {
			t.Errorf("Model = %q", req.Model)
		}
		if len(req.Messages) != 1 || req.Messages[0].Content != "hi" {
			t.Errorf("Messages = %+v", req.Messages)
		}
		if req.Stream {
			t.Errorf("非流式请求不应设置 stream=true")
		}
		_, _ = io.WriteString(w, chatResponseJSON)
	}))
	defer srv.Close()

	client := NewOpenAIClient(srv.URL, "", time.Second)
	_, err := client.Chat(context.Background(), &ChatRequest{Model: "the-model", Messages: []Message{UserText("hi")}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestChatSendsAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, chatResponseJSON)
	}))
	defer srv.Close()

	client := NewOpenAIClient(srv.URL, "secret-key", time.Second)
	if _, err := client.Chat(context.Background(), &ChatRequest{Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestChatNoAuthWhenEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, chatResponseJSON)
	}))
	defer srv.Close()

	client := NewOpenAIClient(srv.URL, "", time.Second)
	if _, err := client.Chat(context.Background(), &ChatRequest{Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Errorf("无 apiKey 时不应携带 Authorization，得到 %q", gotAuth)
	}
}

func TestChatServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":"boom"}`)
	}))
	defer srv.Close()

	client := NewOpenAIClient(srv.URL, "", time.Second)
	if _, err := client.Chat(context.Background(), &ChatRequest{Model: "m"}); err == nil {
		t.Fatal("500 状态应当返回错误")
	} else if !strings.Contains(err.Error(), "500") {
		t.Errorf("错误中应包含状态码，得到 %v", err)
	}
}

func TestChatBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not-json")
	}))
	defer srv.Close()

	client := NewOpenAIClient(srv.URL, "", time.Second)
	if _, err := client.Chat(context.Background(), &ChatRequest{Model: "m"}); err == nil {
		t.Fatal("非法 JSON 应当返回错误")
	}
}

func TestChatEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[]}`)
	}))
	defer srv.Close()

	client := NewOpenAIClient(srv.URL, "", time.Second)
	if _, err := client.Chat(context.Background(), &ChatRequest{Model: "m"}); err == nil {
		t.Fatal("空 choices 应当返回错误")
	}
}

func TestChatTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(w, chatResponseJSON)
	}))
	defer srv.Close()

	client := NewOpenAIClient(srv.URL, "", 10*time.Millisecond)
	if _, err := client.Chat(context.Background(), &ChatRequest{Model: "m"}); err == nil {
		t.Fatal("超时应当返回错误")
	}
}

func TestStreamSuccess(t *testing.T) {
	sse := "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, sse)
	}))
	defer srv.Close()

	var sb strings.Builder
	client := NewOpenAIClient(srv.URL, "", time.Second)
	err := client.ChatStream(context.Background(), &ChatRequest{Model: "m"}, func(chunk ChatChunk) {
		if len(chunk.Choices) > 0 {
			sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	})
	if err != nil {
		t.Fatalf("ChatStream 返回错误: %v", err)
	}
	if sb.String() != "Hello world" {
		t.Errorf("拼接内容 = %q", sb.String())
	}
}

func TestStreamBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "data: not-json\n\n")
	}))
	defer srv.Close()

	client := NewOpenAIClient(srv.URL, "", time.Second)
	if err := client.ChatStream(context.Background(), &ChatRequest{Model: "m"}, func(ChatChunk) {}); err == nil {
		t.Fatal("非法流式块应当返回错误")
	}
}

func TestBaseURLTrailingSlash(t *testing.T) {
	c := NewOpenAIClient("http://x/v1/", "", time.Second)
	if c.baseURL != "http://x/v1" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
}
