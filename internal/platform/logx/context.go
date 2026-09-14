package logx

import (
	"context"

	"go.uber.org/zap"
)

type (
	loggerKey    struct{}
	requestIDKey struct{}
)

// global 是当 context 中没有 logger 时，From 函数返回的备用日志器。
// 这种情况常见于测试、后台任务和 CLI 命令中。
var global = zap.NewNop()

// SetGlobal 设置进程级的备用日志器。
// 在启动阶段调用一次，在任何 goroutine 可能输出日志之前。
func SetGlobal(l *zap.Logger) {
	if l != nil {
		global = l
	}
}

// Global 返回全局备用日志器。
func Global() *zap.Logger {
	return global
}

// Into 返回一个携带了 l 的 context。
func Into(ctx context.Context, l *zap.Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey{}, l)
}

// From 返回 context 中携带的日志器；若不存在则返回全局备用日志器。
//
// 该方法永远不返回 nil，调用方始终可以安全地写 log.From(ctx).Info(...)，
// 无需额外的判空逻辑。
func From(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok && l != nil {
			return l
		}
	}
	return global
}

// WithRequestID 返回一个携带 request id 的 context，以及一个带有该 id 标签的日志器。
// 将 id 与日志器分开存储，可以让非日志相关代码（如出站 HTTP 调用、消息头）也能传递该 id。
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, requestIDKey{}, id)
	return Into(ctx, From(ctx).With(zap.String("request_id", id)))
}

// RequestID 返回 ctx 中携带的 request id，若不存在则返回空字符串。
func RequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// With 返回一个 context，其日志器已附加了额外字段，
// 以便服务能够为请求中的每一条后续日志行添加上下文信息。
func With(ctx context.Context, fields ...zap.Field) context.Context {
	if len(fields) == 0 {
		return ctx
	}
	return Into(ctx, From(ctx).With(fields...))
}
