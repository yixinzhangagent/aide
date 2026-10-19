// Package commands 实现 aide 的命令行接口（cobra）。
//
// 根命令负责加载配置与构造 LLM 客户端，并通过共享的 deps 把这些依赖
// 注入各子命令；子命令各自对应一个可立即产生价值的具体动作。
package commands

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yixinzhangagent/aide/internal/agent"
	"github.com/yixinzhangagent/aide/internal/config"
	"github.com/yixinzhangagent/aide/internal/llm"
	"github.com/yixinzhangagent/aide/internal/version"
)

// deps 是各子命令共享的运行依赖。
type deps struct {
	cfg    *config.Config
	client llm.Client
	out    io.Writer
}

// NewRootCmd 构造完整的根命令树。
func NewRootCmd() *cobra.Command {
	var (
		configPath string
		model      string
		serverURL  string
		workDir    string
		verbose    bool
	)
	d := &deps{out: os.Stdout}

	root := &cobra.Command{
		Use:           "aide",
		Short:         "aide —— 运行在你本机的个人 AI 助理",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			// 显式传入的全局 flag 覆盖配置文件。
			if cmd.Flags().Changed("model") {
				cfg.Model = model
			}
			if cmd.Flags().Changed("server-url") {
				cfg.ServerURL = serverURL
			}
			if cmd.Flags().Changed("work-dir") {
				cfg.WorkDir = workDir
			}
			cfg.Verbose = verbose || cfg.Verbose
			if err := cfg.Validate(); err != nil {
				return err
			}
			d.cfg = cfg
			d.client = llm.NewOpenAIClient(cfg.ServerURL, cfg.APIKey, cfg.Timeout)
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&configPath, "config", "c", "", "配置文件路径（默认查找 ./config.yaml）")
	pf.StringVar(&model, "model", "", "覆盖默认对话模型")
	pf.StringVar(&serverURL, "server-url", "", "覆盖 LLM 服务地址（OpenAI 兼容）")
	pf.StringVar(&workDir, "work-dir", "", "覆盖工作目录（只读工具的根）")
	pf.BoolVarP(&verbose, "verbose", "v", false, "输出调试信息")

	root.AddCommand(
		newChatCmd(d),
		newAskCmd(d),
		newExplainCmd(d),
		newCommitCmd(d),
		newServeCmd(d),
		newVersionCmd(),
	)
	return root
}

// Execute 运行根命令，返回最上层错误。
func Execute() error {
	return NewRootCmd().Execute()
}

// streamOnce 以流式方式进行一轮对话，把增量文本实时写入 d.out，
// 并返回本次完整的助手回答（用于沉淀到历史）。
func (d *deps) streamOnce(ctx context.Context, model string, history []llm.Message, prompt string) (string, error) {
	messages := make([]llm.Message, 0, len(history)+2)
	messages = append(messages, llm.SystemText(agent.DefaultSystemPrompt))
	messages = append(messages, history...)
	messages = append(messages, llm.UserText(prompt))

	req := &llm.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: d.cfg.Temperature,
	}

	var sb strings.Builder
	err := d.client.ChatStream(ctx, req, func(chunk llm.ChatChunk) {
		if len(chunk.Choices) == 0 {
			return
		}
		text := chunk.Choices[0].Delta.Content
		if text != "" {
			_, _ = io.WriteString(d.out, text)
			sb.WriteString(text)
		}
	})
	if err != nil {
		return "", err
	}
	return sb.String(), nil
}

// versionCmd 单独在此定义，避免再开一个文件。
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "打印版本信息",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = io.WriteString(cmd.OutOrStdout(), version.String()+"\n")
		},
	}
}
