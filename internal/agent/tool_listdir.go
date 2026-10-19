package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/yixinzhangagent/aide/internal/llm"
)

// ListDirTool 是只读工具：列出工作目录内某一层的文件与子目录。
type ListDirTool struct {
	root string
}

// NewListDirTool 创建目录列举工具。
func NewListDirTool(root string) *ListDirTool {
	return &ListDirTool{root: root}
}

// Name 实现 Tool。
func (t *ListDirTool) Name() string { return "list_dir" }

// Definition 实现 Tool。
func (t *ListDirTool) Definition() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "list_dir",
			Description: "列出工作目录内某个目录下的文件与子目录（仅一层，不递归）。目录名以 / 结尾，隐藏项默认不显示。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "相对于工作目录的目录路径，省略或为 . 表示根目录",
					},
				},
			},
		},
	}
}

type listDirArgs struct {
	Path string `json:"path"`
}

// Execute 实现 Tool。
func (t *ListDirTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args listDirArgs
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("参数解析失败: %w", err)
		}
	}
	if args.Path == "" {
		args.Path = "."
	}
	full, err := safeJoin(t.root, args.Path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("路径不是目录: %s", args.Path)
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "(空目录)", nil
	}
	return strings.Join(names, "\n"), nil
}
