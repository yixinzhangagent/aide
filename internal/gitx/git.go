// Package gitx 封装 aide 所需的少量 git 只读与提交操作。
//
// 通过 Runner 抽象把“执行 git 命令”与“业务逻辑”解耦，
// 既可用真实 git 跑，也能在单元测试中用假的 Runner 验证参数构造。
package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner 执行一条 git 命令并返回标准输出。
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

// ExecRunner 调用系统真实的 git 可执行文件。
type ExecRunner struct{}

// Run 实现 Runner。
func (ExecRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	// 可执行文件固定为 git，参数仅为 git 子命令，G204 在此是安全的。
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// Repo 表示一个工作目录下的 git 仓库。
type Repo struct {
	dir    string
	runner Runner
}

// NewRepo 创建绑定到 dir 的仓库句柄。
func NewRepo(dir string) *Repo {
	return &Repo{dir: dir, runner: ExecRunner{}}
}

// StagedDiff 返回已暂存改动的完整 diff。
func (r *Repo) StagedDiff(ctx context.Context) (string, error) {
	return r.runner.Run(ctx, r.dir, "diff", "--staged", "--no-color", "--")
}

// UnstagedDiff 返回工作区未暂存改动的 diff。
func (r *Repo) UnstagedDiff(ctx context.Context) (string, error) {
	return r.runner.Run(ctx, r.dir, "diff", "--no-color", "--")
}

// HasStagedChanges 判断是否存在已暂存的改动。
func (r *Repo) HasStagedChanges(ctx context.Context) (bool, error) {
	out, err := r.runner.Run(ctx, r.dir, "diff", "--staged", "--name-only", "--")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Commit 用给定的完整提交信息执行 git commit。
func (r *Repo) Commit(ctx context.Context, message string) error {
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("提交信息不能为空")
	}
	_, err := r.runner.Run(ctx, r.dir, "commit", "-m", message)
	return err
}
