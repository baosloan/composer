package middleware

import (
	"composer/internal/platform/errorx"
	"composer/internal/platform/httpx"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout 用于限制处理器的最大执行时间。
//
// 超时时间会设置到请求的 context 上，而 repository 层传给
// db.WithContext 的也是同一个 context —— 因此一旦发生超时，
// 正在执行的数据库查询会被真正取消，而不是在客户端已经放弃等待后，
// 仍然继续在后台执行直到结束。
func Timeout(d time.Duration) gin.HandlerFunc {
	if d <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		// 处理器直接在当前 goroutine 中执行，而不是放到新的 goroutine 中。
		//
		// 一种常见的写法是启动一个 watchdog goroutine，与处理器竞争，
		// 并在超时时主动写入响应，但这种方式其实是错误的：
		// *gin.Context 和 ResponseWriter 都不支持并发安全访问，
		// 因此 watchdog 写入的超时响应可能会与处理器正在写入的响应交错，
		// 最终导致响应内容损坏。相比响应稍晚返回，损坏响应更加糟糕。
		//
		// 由于超时截止时间被设置到了请求 context 上，
		// 而 repository 层也会将这个 context 传给 db.WithContext，
		// 因此慢查询会在数据库驱动层被取消，
		// 处理器随后能够自行及时返回。
		c.Next()

		// 只有在处理器尚未产生任何响应时才返回超时错误。
		// 如果处理器已经返回了响应，那么即使执行时间较长，
		// 也以处理器已经生成的响应为准。
		if ctx.Err() != nil && !c.Writer.Written() {
			httpx.Error(c, errorx.ErrTimeout.Wrap(ctx.Err()))
		}
	}
}

// BodyLimit 用于拒绝超过 max 字节的请求体。
//
// http.MaxBytesReader 会在读取超过限制后停止继续读取，
// 而不是先把整个请求体缓冲到内存中，
// 因此超大请求无法在被拒绝之前耗尽服务器内存。
// max 为 0 时表示禁用请求体大小限制。
func BodyLimit(max int64) gin.HandlerFunc {
	if max <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		// Content-Length 只能作为参考，它可能不存在，也可能不准确。
		// 因此对于明显超出限制的请求可以提前拒绝，
		// 其余请求仍然需要通过 MaxBytesReader 对实际读取大小进行限制。
		if c.Request.ContentLength > max {
			httpx.Error(c, errorx.ErrPayloadTooLarge.WithMessagef(
				"request body exceeds the %d byte limit", max))
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}
