package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yixinzhangagent/aide/internal/agent"
	"github.com/yixinzhangagent/aide/internal/llm"
)

// newExplainCmd 创建代码解释命令。
// 目标格式为 <相对路径> 或 <相对路径>:<行号>。
func newExplainCmd(d *deps) *cobra.Command {
	var follow string
	cmd := &cobra.Command{
		Use:   "explain <文件[:行号]>",
		Short: "用编码模型解释工作目录中的代码",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			relPath, lineHint, err := parseTarget(target)
			if err != nil {
				return err
			}

			// 复用只读工具读取，保证路径安全与大小限制。
			tool := agent.NewReadFileTool(d.cfg.WorkDir, d.cfg.MaxFileBytes)
			content, err := tool.Execute(context.Background(),
				fmt.Sprintf(`{"path":%q}`, relPath))
			if err != nil {
				return fmt.Errorf("读取文件失败: %w", err)
			}

			prompt := buildExplainPrompt(relPath, lineHint, content, follow)
			resp, err := d.client.Chat(cmd.Context(), &llm.ChatRequest{
				Model: d.cfg.CoderModel,
				Messages: []llm.Message{
					llm.SystemText("你是资深工程师，用简洁准确的中文解释代码，先给整体作用，再讲关键逻辑与注意点。"),
					llm.UserText(prompt),
				},
				Temperature: 0.3,
			})
			if err != nil {
				return err
			}
			_, _ = io.WriteString(d.out, strings.TrimSpace(resp.Choices[0].Message.Content)+"\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&follow, "q", "", "在解释基础上追加一个具体问题")
	return cmd
}

// parseTarget 拆分 "path" 或 "path:line"。
func parseTarget(target string) (path string, line int, err error) {
	if i := strings.LastIndex(target, ":"); i >= 0 {
		// 排除 Windows 盘符式的单冒号（本项目运行在 macOS，简单处理）。
		maybeLine := target[i+1:]
		if n, perr := strconv.Atoi(maybeLine); perr == nil {
			if n <= 0 {
				return "", 0, errors.New("行号必须为正整数")
			}
			return target[:i], n, nil
		}
	}
	return target, 0, nil
}

func buildExplainPrompt(relPath string, line int, content, follow string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "请解释文件 `%s`：", relPath)
	if line > 0 {
		fmt.Fprintf(&b, "请重点关注第 %d 行附近。", line)
	}
	b.WriteString("\n\n```\n")
	b.WriteString(content)
	b.WriteString("\n```\n")
	if follow = strings.TrimSpace(follow); follow != "" {
		fmt.Fprintf(&b, "\n另外请回答：%s", follow)
	}
	return b.String()
}
