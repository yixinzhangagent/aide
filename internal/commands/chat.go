package commands

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yixinzhangagent/aide/internal/llm"
)

// newChatCmd 创建交互式对话命令。
func newChatCmd(d *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "开始与本地模型的多轮流式对话",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			_, _ = io.WriteString(d.out, "已进入对话（模型 "+d.cfg.Model+"）。输入 exit/quit 或 Ctrl+C 退出。\n")

			scanner := bufio.NewScanner(cmd.InOrStdin())
			// 单行输入可能较长，放大内部缓冲。
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

			var history []llm.Message
			for {
				_, _ = io.WriteString(d.out, "\n> ")
				if !scanner.Scan() {
					break // EOF
				}
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				if line == "exit" || line == "quit" {
					break
				}
				answer, err := d.streamOnce(ctx, d.cfg.Model, history, line)
				if err != nil {
					return err
				}
				history = append(history, llm.UserText(line), llm.Message{
					Role: llm.RoleAssistant, Content: answer,
				})
				_, _ = io.WriteString(d.out, "\n")
			}
			_, _ = fmt.Fprintln(d.out, "已退出对话。")
			return nil
		},
	}
}
