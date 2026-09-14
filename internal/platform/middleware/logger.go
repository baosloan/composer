package middleware

import (
	"composer/internal/platform/logx"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger 为每个请求记录一条结构化日志，
// 并初始化后续各层都会使用的请求级日志记录器。
//
// 它必须在 RequestID 之后执行，
// 这样创建的日志记录器中已经包含用于请求关联的 request_id。
func Logger(base *zap.Logger, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if _, ok := skip[path]; ok {
			// 健康检查和指标监控接口通常每隔几秒就会被轮询一次；
			// 如果也记录这些请求日志，会淹没真正有价值的业务流量日志。
			c.Next()
			return
		}

		start := time.Now()
		query := c.Request.URL.RawQuery

		// 显式基于 base 创建请求级日志记录器，
		// 然后附加所有下层都应继承的公共字段。
		//
		// 从 base 派生，而不是依赖 context 中可能存在的日志记录器，
		// 可以让该中间件不受全局兜底日志记录器的影响，保持相对独立。
		ctx := logx.Into(c.Request.Context(), base.With(
			zap.String("request_id", logx.RequestID(c.Request.Context())),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
		))
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		status := c.Writer.Status()
		// 这里省略 method 和 path：
		// 上面创建的上下文日志记录器已经包含这些字段，
		// 如果再次添加，会导致输出的 JSON 中出现重复键。
		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.Int("bytes", c.Writer.Size()),
			zap.Duration("latency", time.Since(start)),
		}
		// 输出处理器通过 c.Error 记录的所有错误。
		if e := c.Errors.ByType(gin.ErrorTypePrivate).String(); e != "" {
			fields = append(fields, zap.String("errors", e))
		}

		l := logx.From(ctx)
		switch {
		case status >= 500:
			l.Error("http_request", fields...)
		case status >= 400:
			l.Warn("http_request", fields...)
		default:
			l.Info("http_request", fields...)
		}
	}
}
