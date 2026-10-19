// Package version 保存构建期通过 -ldflags -X 注入的版本信息。
package version

import "fmt"

// 这些变量在构建时由 Makefile 通过 -ldflags "-X ..." 覆盖；
// 未注入时（如直接 go run）给出可读的默认值。
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// String 返回单行人类可读的版本描述。
func String() string {
	return fmt.Sprintf("aide %s (commit %s, built %s)", Version, Commit, BuildTime)
}
