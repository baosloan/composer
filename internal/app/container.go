package app

import (
	"composer/internal/config"
	"composer/internal/modules/health"
	"composer/internal/platform/logx"
	"context"
	"errors"

	"go.uber.org/zap"
)

type Container struct {
	Cfg     *config.Config
	Logger  *zap.Logger
	Version string

	//模块
	Health *health.Handler
}

// NewContainer 构建依赖关系图。
//
// 任何失败都会中止启动。一个带着损坏的数据库启动、只在第一个请求时才失败的项目， 比一个直接拒绝启动的项目更难排查。
func NewContainer(ctx context.Context, cfg *config.Config, logger *zap.Logger, version string) (*Container, error) {
	c := &Container{Cfg: cfg, Logger: logger, Version: version}

	// 模块。每个模块都按照 repository → service → handler 的顺序在一行中完成初始化。
	c.Health = health.NewHandler(version)

	logx.SetGlobal(logger)
	return c, nil
}

// healthCheckers 用于描述就绪探针实际会检查哪些依赖。
//
// 数据库属于关键依赖：如果数据库不可用，基本上所有接口都无法正常工作。
// 缓存和消息代理则不是关键依赖，因此它们发生故障时只会让服务进入降级状态，
// 而不会直接将当前副本从负载均衡器中移除。
func (c *Container) healthCheckers() []health.Checker {
	checkers := []health.Checker{}
	return checkers
}

// Close 按构建顺序的逆序释放所有依赖。
//
// 错误会被收集而不会提前返回：关闭 Kafka 失败时，不应泄漏数据库连接。
func (c *Container) Close() error {
	var errs []error
	if c.Logger != nil {
		// 在某些平台上，当 stdout 是终端时，Sync 会失败；
		// 这不值得作为关闭错误来报告。
		_ = c.Logger.Sync()
	}

	return errors.Join(errs...)
}
