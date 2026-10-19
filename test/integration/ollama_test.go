//go:build integration

// 集成测试：真实调用本地运行的 Ollama，验证端到端链路。
// 运行方式：make test-integration（需先启动 Ollama）。
package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yixinzhangagent/aide/internal/llm"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TestOllamaChat 验证非流式对话能拿到非空回答。
func TestOllamaChat(t *testing.T) {
	serverURL := envOr("AIDE_SERVER_URL", "http://127.0.0.1:11434/v1")
	model := envOr("AIDE_MODEL", "qwen3.6:35b-mlx")

	client := llm.NewOpenAIClient(serverURL, "", 5*time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	resp, err := client.Chat(ctx, &llm.ChatRequest{
		Model:    model,
		Messages: []llm.Message{llm.UserText("用一句话介绍你自己。")},
	})
	if err != nil {
		t.Fatalf("调用 Ollama 失败: %v", err)
	}
	answer := strings.TrimSpace(resp.Choices[0].Message.Content)
	if answer == "" {
		t.Fatal("返回了空回答")
	}
	t.Logf("模型回答: %s", answer)
}

// TestOllamaStream 验证流式对话能拼出非空内容。
func TestOllamaStream(t *testing.T) {
	serverURL := envOr("AIDE_SERVER_URL", "http://127.0.0.1:11434/v1")
	model := envOr("AIDE_MODEL", "qwen3.6:35b-mlx")

	client := llm.NewOpenAIClient(serverURL, "", 5*time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var sb strings.Builder
	err := client.ChatStream(ctx, &llm.ChatRequest{
		Model:    model,
		Messages: []llm.Message{llm.UserText("用一句话说明什么是递归。")},
	}, func(chunk llm.ChatChunk) {
		if len(chunk.Choices) > 0 {
			sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	})
	if err != nil {
		t.Fatalf("流式调用失败: %v", err)
	}
	if strings.TrimSpace(sb.String()) == "" {
		t.Fatal("流式拼出了空内容")
	}
	t.Logf("流式回答: %s", sb.String())
}
