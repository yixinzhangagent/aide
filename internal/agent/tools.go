// Package agent 实现 aide 的对话编排与工具调用。
//
// Agent 维护多轮对话历史，在模型需要时执行已注册的工具，把工具结果
// 回填给模型，直到模型给出最终文本。工具默认仅开放只读能力，并且
// 所有路径都被限制在工作目录内，确保不会越权访问本机其他文件。
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yixinzhangagent/aide/internal/llm"
)

// Tool 是一个可被模型调用的工具。
type Tool interface {
	// Name 返回工具的唯一名称。
	Name() string
	// Definition 返回 OpenAI function-calling 所需的工具定义。
	Definition() llm.Tool
	// Execute 执行工具；argsJSON 为模型给出的 JSON 字符串参数。
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// safeJoin 把工具参数中的路径解析为真实路径，并确保它位于 root 之内。
//
// 它会先 EvalSymlinks 解析 root 与目标，从而挡住两类越权：
//  1. 形如 "../secret" 的目录穿越；
//  2. 位于 root 内、但通过符号链接指向 root 外部的路径。
func safeJoin(root, p string) (string, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("工作目录不可访问: %w", err)
	}
	candidate := p
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	realTarget, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("路径不存在: %s", p)
		}
		return "", fmt.Errorf("路径不可访问: %s", p)
	}
	if !within(realRoot, realTarget) {
		return "", fmt.Errorf("路径越出工作目录，已拒绝: %s", p)
	}
	return realTarget, nil
}

// within 判断 target 是否等于 root 或位于 root 子树内。
func within(root, target string) bool {
	if root == target {
		return true
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
