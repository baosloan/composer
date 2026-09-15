package app

import (
	"composer/internal/config"
	"context"
	"errors"
	"net/http"

	"go.uber.org/zap"
)

type App struct {
	cfg       *config.Config
	logger    *zap.Logger
	container *Container
	server    *http.Server
}

func New(ctx context.Context, configPath, version string) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		return nil, err
	}

	container, err := NewContainer(ctx, cfg, logger, version)
	if err != nil {
		// container 可能在失败前已经打开了一些连接；
		// 释放它已成功构建的部分资源。
		if container != nil {
			_ = container.Close()
		}
		return nil, err
	}
	engine, err := NewEngine(container)
	if err != nil {
		_ = container.Close()
		return nil, err
	}

	return &App{
		cfg:       cfg,
		logger:    logger,
		container: container,
		server: &http.Server{
			Addr:    cfg.Server.Addr(),
			Handler: engine,
			// Timeouts 限制了慢客户端占用连接的最长时间。
			// 如果没有这些超时设置，少量卡住的客户端就可能耗尽服务器资源。
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
			// 防止客户端发送无限大的请求头。
			MaxHeaderBytes: 1 << 20,
		},
	}, nil
}

// Run 会持续运行直到 ctx 被取消，然后优雅关闭。
func (a *App) Run(ctx context.Context) error {
	// 使用带缓冲的通道，确保 goroutine 总能发送并退出，即使没有接收方。
	// 如果这里使用无缓冲通道，会导致 goroutine 泄漏。
	errCh := make(chan error, 1)

	go func() {
		a.logger.Info("http server listening",
			zap.String("addr", a.server.Addr),
			zap.String("mode", a.cfg.Server.Mode),
			zap.String("base_path", a.cfg.Server.BasePath),
		)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		// 绑定失败，或监听器已关闭。无需执行关闭操作。
		return err
	case <-ctx.Done():
	}

	a.logger.Info("shutdown signal received, draining connections")

	// 给正在处理中的请求一个有限的时间窗口来完成。
	// 故意使用 background context：ctx 已经被取消，直接传给它会导致立即中止。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("graceful shutdown failed, closing connections", zap.Error(err))
		_ = a.server.Close()
	}

	a.logger.Info("shutdown complete")
	return nil
}

// Close 释放所有依赖。在 Run 返回后调用是安全的。
func (a *App) Close() error {
	var err error
	if a.logger != nil {
		// 在某些平台上，当 stdout 是终端时，Sync 会失败；
		// 这不值得作为关闭错误来报告。
		err = a.logger.Sync()
	}
	return err
}

// Config 暴露已加载的配置，供需要配置但无需启动服务器的 CLI 命令使用。
func (a *App) Config() *config.Config {
	return a.cfg
}
