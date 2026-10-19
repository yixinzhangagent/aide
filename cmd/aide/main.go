// Command aide 是运行在本机的个人 AI 助理的入口。
package main

import (
	"fmt"
	"os"

	"github.com/yixinzhangagent/aide/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}
