package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yixinzhangagent/aide/internal/gitx"
	"github.com/yixinzhangagent/aide/internal/llm"
)

// newCommitCmd 创建“按 diff 生成提交信息”的命令。
//
// 安全默认：仅打印生成的提交信息（dry-run），必须显式加 --apply 才提交。
func newCommitCmd(d *deps) *cobra.Command {
	var (
		apply           bool
		includeUnstaged bool
	)
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "根据已暂存改动生成 Conventional Commits 提交信息",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			repo := gitx.NewRepo(d.cfg.WorkDir)

			has, err := repo.HasStagedChanges(ctx)
			if err != nil {
				return fmt.Errorf("检查暂存区失败: %w", err)
			}
			if !has {
				return errors.New("没有已暂存的改动；请先 `git add`，本命令不会替你暂存")
			}

			diff, err := repo.StagedDiff(ctx)
			if err != nil {
				return fmt.Errorf("读取暂存 diff 失败: %w", err)
			}
			if includeUnstaged {
				if extra, err := repo.UnstagedDiff(ctx); err == nil && strings.TrimSpace(extra) != "" {
					diff += "\n" + extra
				}
			}

			message, err := d.generateCommitMessage(ctx, diff)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(d.out, message+"\n")

			if apply {
				if err := repo.Commit(ctx, message); err != nil {
					return fmt.Errorf("执行 git commit 失败: %w", err)
				}
				_, _ = io.WriteString(d.out, "\n已提交。\n")
				return nil
			}
			_, _ = io.WriteString(d.out, "\n（dry-run，未提交；确认无误后加 --apply 执行）\n")
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "真正执行 git commit（默认仅打印）")
	cmd.Flags().BoolVar(&includeUnstaged, "include-unstaged", false, "同时参考未暂存改动（仍不会自动 git add）")
	return cmd
}

// generateCommitMessage 让模型根据 diff 生成规范的单行提交信息。
func (d *deps) generateCommitMessage(ctx context.Context, diff string) (string, error) {
	const system = "你是严格遵循 Conventional Commits 规范的提交信息生成器。"
	prompt := "请根据下面的 git diff 生成一条提交信息。要求：\n" +
		"1. 格式 `type(scope): 简短描述`，type 从 feat/fix/docs/style/refactor/perf/test/chore/build/ci 中选择；\n" +
		"2. 描述用中文，简明说明改了什么，不超过 50 字；\n" +
		"3. 只输出提交信息本身，不要解释、不要代码块、不要多余空行。\n\n" +
		"Diff:\n" + diff

	resp, err := d.client.Chat(ctx, &llm.ChatRequest{
		Model: d.cfg.CoderModel,
		Messages: []llm.Message{
			llm.SystemText(system),
			llm.UserText(prompt),
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(firstLine(resp.Choices[0].Message.Content)), nil
}

// firstLine 取多行文本的第一行，防止模型附带解释性内容。
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
