package commands

import (
	"errors"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// newAskCmd 创建一次性问答命令，支持命令行参数或 stdin 管道。
func newAskCmd(d *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "ask [问题]",
		Short: "一次性问答（可通过管道输入）",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.TrimSpace(strings.Join(args, " "))
			if prompt == "" {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				prompt = strings.TrimSpace(string(data))
			}
			if prompt == "" {
				return errors.New("请提供问题，或通过管道把内容传入")
			}

			answer, err := d.streamOnce(cmd.Context(), d.cfg.Model, nil, prompt)
			if err != nil {
				return err
			}
			if answer != "" {
				_, _ = io.WriteString(d.out, "\n")
			}
			return nil
		},
	}
}
