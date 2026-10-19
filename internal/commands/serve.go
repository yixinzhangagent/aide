package commands

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/yixinzhangagent/aide/internal/server"
)

// newServeCmd 创建本地 HTTP 服务命令，支持 Ctrl+C 优雅关闭。
func newServeCmd(d *deps) *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动本地 HTTP 服务（API + 内置网页）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if addr == "" {
				addr = d.cfg.HTTPAddr
			}
			srv := server.New(d.client, d.cfg, addr)

			errCh := make(chan error, 1)
			go func() { errCh <- srv.ListenAndServe() }()

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			select {
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := srv.Shutdown(shutdownCtx); err != nil {
					return err
				}
				_, _ = d.out.Write([]byte("服务已关闭。\n"))
				return nil
			}
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "", "监听地址（默认取配置 http_addr，仅 127.0.0.1）")
	return cmd
}
