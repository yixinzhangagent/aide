// Package llm 定义与大语言模型交互的类型与客户端抽象。
//
// 数据结构遵循 OpenAI 的 /chat/completions 格式，因此可以直接对接
// 本地 Ollama（其内置 OpenAI 兼容端点），也可切换到任意兼容的云端服务。
package llm

import "context"

// 对话角色常量。
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message 表示一条对话消息。
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	// Name 为工具消息对应的工具名（部分后端需要）。
	Name string `json:"name,omitempty"`
}

// ToolCall 是模型发起的一次工具调用。
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 记录要调用的函数名与 JSON 字符串形式的参数。
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool 是提供给模型的工具定义（OpenAI function-calling 格式）。
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction 描述工具名、用途与 JSON Schema 参数。
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatRequest 是一次补全请求。
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse 是一次非流式补全响应。
type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// Choice 是单个候选结果。
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ChatChunk 是流式响应中的一个增量块。
type ChatChunk struct {
	Choices []ChunkChoice `json:"choices"`
}

// ChunkChoice 是流式中的单个候选；FinishReason 在未结束时为 null。
type ChunkChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

// Delta 是本块相对上一块的增量内容。
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// Client 是 LLM 后端的抽象，便于替换实现与在测试中模拟。
type Client interface {
	// Chat 执行一次非流式补全。
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// ChatStream 执行一次流式补全，每收到一个文本块即回调 onChunk。
	ChatStream(ctx context.Context, req *ChatRequest, onChunk func(chunk ChatChunk)) error
}

// Text 是一个便捷构造：返回纯文本的用户消息。
func UserText(text string) Message { return Message{Role: RoleUser, Content: text} }

// SystemText 是一个便捷构造：返回系统消息。
func SystemText(text string) Message { return Message{Role: RoleSystem, Content: text} }
