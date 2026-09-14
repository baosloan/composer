package cmd

import (
	"composer/internal/app"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		// NotifyContext 在收到 SIGINT/SIGTERM 时取消，
		// 正是这一点使得容器的停止信号触发优雅排空（drain），
		// 而不是被直接杀死（kill）。
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		a, err := app.New(ctx, configPath, version)
		if err != nil {
			return err
		}
		defer func() { _ = a.Close() }()

		return a.Run(ctx)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
