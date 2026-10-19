// Package server 实现 aide 的本地 HTTP 服务。
//
// 它复用与 CLI 相同的 llm.Client，对外暴露：
//   - GET  /healthz  健康检查；
//   - POST /v1/chat  OpenAI 风格的对话接口；
//   - GET  /         内嵌的极简聊天网页（go:embed，无外部前端依赖）。
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/yixinzhangagent/aide/internal/config"
	"github.com/yixinzhangagent/aide/internal/llm"
)

// webFS 嵌入随包发布的静态网页。
//
//go:embed web
var webFS embed.FS

// New 构造一个已注册全部路由的 *http.Server。
// 默认只监听回环地址（由 addr 决定），不向局域网暴露。
func New(client llm.Client, cfg *config.Config, addr string) *http.Server {
	h := &handlers{client: client, cfg: cfg}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("POST /v1/chat", h.chat)

	// 静态网页：从嵌入 FS 的 web 子目录提供。
	static, err := fs.Sub(webFS, "web")
	if err != nil {
		panic("内嵌静态资源缺失: " + err.Error())
	}
	mux.Handle("GET /", http.FileServer(http.FS(static)))

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
}
