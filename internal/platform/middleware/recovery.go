package middleware

import (
	"composer/internal/platform/errorx"
	"composer/internal/platform/httpx"
	"composer/internal/platform/logx"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery 用于捕获 panic，将其转换为统一的 500 响应，并记录调用栈。
//
// 它必须注册在所有可能触发 panic 的中间件之前，
// 同时要放在 RequestID 之后，以便日志能够通过 request_id 追踪到对应请求。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			ctx := c.Request.Context()

			// broken pipe 表示客户端在响应过程中断开了连接。
			// 此时已经没有客户端可以接收 500 响应，
			// 而且如果每次下载取消都记录完整调用栈，只会产生大量无意义的日志，
			// 因此这里只做简单记录并直接结束处理。
			if isBrokenPipe(r) {
				logx.From(ctx).Warn("client disconnected",
					zap.Any("error", r),
					zap.String("path", c.Request.URL.Path),
				)
				c.Abort()
				return
			}

			logx.From(ctx).Error("panic_recovered",
				zap.Any("panic", r),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("ip", c.ClientIP()),
				zap.Stack("stack"),
			)

			// 仅当响应尚未发送时才写入响应；
			// 如果响应已经部分写出，再追加第二个响应体会导致响应内容损坏。
			if c.Writer.Written() {
				c.Abort()
				return
			}
			// panic 的具体内容绝不会返回给客户端：
			// 因为其中通常可能包含内部路径、SQL 片段或敏感信息。
			httpx.Error(c, errorx.ErrInternal)
		}()

		c.Next()
	}
}

// isBrokenPipe 用于判断当前 panic 是否由客户端关闭连接引起。
func isBrokenPipe(r any) bool {
	err, ok := r.(error)
	if !ok {
		return false
	}

	var ne *net.OpError
	if !errors.As(err, &ne) {
		return false
	}
	var se *os.SyscallError
	if !errors.As(ne, &se) {
		return false
	}

	msg := strings.ToLower(se.Err.Error())
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection reset by peer")
}

// NoRoute 使用统一的响应结构处理未知路由，
// 使客户端收到的 404 响应与其他错误响应保持一致。
func NoRoute() gin.HandlerFunc {
	return func(c *gin.Context) {
		httpx.Error(c, errorx.ErrNotFound.WithMessagef(
			"no route for %s %s", c.Request.Method, c.Request.URL.Path))
	}
}

// NoMethod 用于处理请求路径存在但 HTTP 方法不匹配的情况。
//
// 需要设置 router.HandleMethodNotAllowed = true，
// 否则 Gin 会将这类请求直接处理为 404。
func NoMethod() gin.HandlerFunc {
	return func(c *gin.Context) {
		httpx.Error(c, errorx.New(
			errorx.ErrBadRequest.Code,
			http.StatusMethodNotAllowed,
			"method "+c.Request.Method+" is not allowed on this path",
		))
	}
}
