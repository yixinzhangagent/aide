package agent

import (
	"context"
	"fmt"

	"github.com/yixinzhangagent/aide/internal/llm"
)

// DefaultSystemPrompt 是默认的系统提示词。
const DefaultSystemPrompt = `你是 aide，一个运行在用户本机的 AI 助理。
你可以使用提供的只读工具查看工作目录中的文件与目录。
请基于实际文件内容回答，简洁、准确；当信息不足或无法确定时，明确说明，不要编造。`

// Agent 编排与 LLM 的多轮对话及工具调用。
type Agent struct {
	client       llm.Client
	model        string
	tools        map[string]Tool
	orderedTools []Tool
	history      []llm.Message
	maxToolTurns int
	temperature  float64
	systemPrompt string
}

// Option 用于在构造时配置 Agent。
type Option func(*Agent)

// New 创建 Agent；通过 opts 注入工具与各项参数。
func New(client llm.Client, model string, opts ...Option) *Agent {
	a := &Agent{
		client:       client,
		model:        model,
		tools:        map[string]Tool{},
		maxToolTurns: 6,
		temperature:  0.7,
		systemPrompt: DefaultSystemPrompt,
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

// WithTools 注册一个或多个工具。
func WithTools(tools ...Tool) Option {
	return func(a *Agent) {
		for _, t := range tools {
			if _, exists := a.tools[t.Name()]; !exists {
				a.orderedTools = append(a.orderedTools, t)
			}
			a.tools[t.Name()] = t
		}
	}
}

// WithMaxToolTurns 设置工具调用的最大轮数。
func WithMaxToolTurns(n int) Option {
	return func(a *Agent) {
		if n > 0 {
			a.maxToolTurns = n
		}
	}
}

// WithTemperature 设置采样温度。
func WithTemperature(t float64) Option {
	return func(a *Agent) { a.temperature = t }
}

// WithSystemPrompt 覆盖默认系统提示词。
func WithSystemPrompt(p string) Option {
	return func(a *Agent) {
		if p != "" {
			a.systemPrompt = p
		}
	}
}

// WithHistory 用已有历史初始化（用于多轮会话恢复）。
func WithHistory(msgs []llm.Message) Option {
	return func(a *Agent) { a.history = append(a.history, msgs...) }
}

// History 返回当前对话历史的副本。
func (a *Agent) History() []llm.Message {
	cp := make([]llm.Message, len(a.history))
	copy(cp, a.history)
	return cp
}

// Run 处理一次用户输入，必要时多轮调用工具，返回模型的最终文本。
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	a.history = append(a.history, llm.UserText(userInput))

	for turn := 0; turn < a.maxToolTurns; turn++ {
		resp, err := a.client.Chat(ctx, a.buildRequest())
		if err != nil {
			return "", err
		}
		msg := resp.Choices[0].Message
		a.history = append(a.history, msg)

		// 没有工具调用即得到最终答案。
		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}
		// 执行工具并把结果回填，进入下一轮。
		a.history = append(a.history, a.executeToolCalls(ctx, msg.ToolCalls)...)
	}
	return "", fmt.Errorf("已达到最大工具轮数 %d，仍未得到最终答案", a.maxToolTurns)
}

// buildRequest 组装一次 LLM 请求（系统消息 + 历史 + 工具定义）。
func (a *Agent) buildRequest() *llm.ChatRequest {
	msgs := make([]llm.Message, 0, len(a.history)+1)
	msgs = append(msgs, llm.SystemText(a.systemPrompt))
	msgs = append(msgs, a.history...)

	tools := make([]llm.Tool, 0, len(a.orderedTools))
	for _, t := range a.orderedTools {
		tools = append(tools, t.Definition())
	}
	return &llm.ChatRequest{
		Model:       a.model,
		Messages:    msgs,
		Tools:       tools,
		Temperature: a.temperature,
	}
}

// executeToolCalls 依次执行模型请求的工具调用，返回对应的 tool 消息。
func (a *Agent) executeToolCalls(ctx context.Context, calls []llm.ToolCall) []llm.Message {
	msgs := make([]llm.Message, 0, len(calls))
	for _, call := range calls {
		content := a.runSingleTool(ctx, call)
		msgs = append(msgs, llm.Message{
			Role:       llm.RoleTool,
			ToolCallID: call.ID,
			Name:       call.Function.Name,
			Content:    content,
		})
	}
	return msgs
}

// runSingleTool 执行单个工具调用并把错误转成可读文本（错误也要回传给模型）。
func (a *Agent) runSingleTool(ctx context.Context, call llm.ToolCall) string {
	tool, ok := a.tools[call.Function.Name]
	if !ok {
		return fmt.Sprintf("错误：没有名为 %q 的工具", call.Function.Name)
	}
	out, err := tool.Execute(ctx, call.Function.Arguments)
	if err != nil {
		return fmt.Sprintf("工具 %q 执行出错: %v", call.Function.Name, err)
	}
	return out
}
