package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/yixinzhangagent/aide/internal/llm"
)

// ReadFileTool 是只读工具：读取工作目录内的文本文件。
type ReadFileTool struct {
	root     string
	maxBytes int64
}

// NewReadFileTool 创建读取工具，maxBytes 限制单文件大小。
func NewReadFileTool(root string, maxBytes int64) *ReadFileTool {
	return &ReadFileTool{root: root, maxBytes: maxBytes}
}

// Name 实现 Tool。
func (t *ReadFileTool) Name() string { return "read_file" }

// Definition 实现 Tool。
func (t *ReadFileTool) Definition() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "read_file",
			Description: "读取工作目录内的文本文件内容。只读，不能修改或删除文件。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "相对于工作目录的文件路径",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

type readFileArgs struct {
	Path string `json:"path"`
}

// Execute 实现 Tool。
func (t *ReadFileTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args readFileArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("缺少参数 path")
	}
	full, err := safeJoin(t.root, args.Path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("路径是目录而非文件: %s", args.Path)
	}
	if info.Size() > t.maxBytes {
		return "", fmt.Errorf("文件过大（%d 字节），上限 %d 字节", info.Size(), t.maxBytes)
	}
	// full 已通过 safeJoin 限制在工作目录内，G304 在此是安全的。
	f, err := os.Open(full) // #nosec G304
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	// 多读 1 字节用于判断是否超限，避免文件刚好等于上限时被误判。
	data, err := io.ReadAll(io.LimitReader(f, t.maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > t.maxBytes {
		return "", fmt.Errorf("文件超过读取上限 %d 字节", t.maxBytes)
	}
	return string(data), nil
}
