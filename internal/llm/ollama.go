package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient 是一个面向 OpenAI 兼容 /chat/completions 端点的客户端。
// 它可用于本地 Ollama，也可用于任意兼容云端（通过 apiKey 鉴权）。
type OpenAIClient struct {
	baseURL    string
	apiKey     string
	timeout    time.Duration
	httpClient *http.Client
}

// NewOpenAIClient 构造客户端。
// baseURL 为服务基础地址（如 http://127.0.0.1:11434/v1）；
// apiKey 在本地 Ollama 场景可留空；timeout 仅作用于非流式请求，
// 流式请求的生命周期由传入的 context 控制（避免大模型被整体超时打断）。
func NewOpenAIClient(baseURL, apiKey string, timeout time.Duration) *OpenAIClient {
	return &OpenAIClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		timeout:    timeout,
		httpClient: &http.Client{},
	}
}

// Chat 执行一次非流式补全。
func (c *OpenAIClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	body, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()

	var resp ChatResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM 响应中没有 choices")
	}
	return &resp, nil
}

// ChatStream 执行一次流式补全，逐块回调。
func (c *OpenAIClient) ChatStream(ctx context.Context, req *ChatRequest, onChunk func(ChatChunk)) error {
	req.Stream = true
	body, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	return parseSSE(body, onChunk)
}

// do 发送 HTTP 请求并在状态码非 200 时返回带服务端信息的错误。
func (c *OpenAIClient) do(ctx context.Context, req *ChatRequest) (io.ReadCloser, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("编码 LLM 请求失败: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("构造 LLM 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用 LLM 服务失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("LLM 返回非预期状态 %d: %s",
			resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return resp.Body, nil
}

// parseSSE 解析 OpenAI 风格的 Server-Sent Events：
// 每个事件形如 `data: {json}`，流以 `data: [DONE]` 结束。
func parseSSE(r io.Reader, onChunk func(ChatChunk)) error {
	scanner := bufio.NewScanner(r)
	// 单行可能包含较大的 JSON，显式放大缓冲区。
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}
		var chunk ChatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return fmt.Errorf("解析流式数据块失败: %w", err)
		}
		onChunk(chunk)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取流式响应失败: %w", err)
	}
	return nil
}
