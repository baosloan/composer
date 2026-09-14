package middleware

import (
	"composer/internal/platform/logx"
	"uuid"

	"github.com/gin-gonic/gin"
)

// HeaderRequestID 是携带关联 ID（correlation id）的请求头。
const HeaderRequestID = "X-Request-ID"

// GinKeyRequestID 是 *gin.Context 的键（key），
// 保留给那些手头只有 *gin.Context 的 handler 使用。
// 业务代码应当通过 logx.RequestID 从标准 context 中读取该 id，
// 而不是使用这里的键。
const GinKeyRequestID = "request_id"

// RequestID 分配一个关联 ID，并使其在每个层级都可达。
//
// 关键的一行是替换 c.Request 的那一行：只用 c.Set 存储 id
// 只会把它放进 Gin 自己的键值映射中，而那是与
// c.Request.Context() 不同的容器。由于 handler 会把标准 context
// 向下传递给 service 和 repository，只存在 Gin context 中的 id
// 在 handler 以下就是不可见的——每个 service 和 repository 的日志行
// 都会丢失其关联 ID，链路追踪实际上就止步于传输层。
//
// 改为把它写入请求的 context，则意味着 logx.From(ctx) 能在任意深度
// 返回一个已经打上该 id 标签的 logger，且不依赖 Gin。
func RequestID(reuseInbound bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := ""
		if reuseInbound {
			// 复用上游的 id，使网关处发起的链路追踪得以延续。
			id = c.GetHeader(HeaderRequestID)
		}
		if id == "" {
			id = uuid.New().String()
		}

		c.Set(GinKeyRequestID, id)
		c.Writer.Header().Set(HeaderRequestID, id)

		// 将 id（以及一个打上该 id 标签的 logger）安装到请求的 context 中。
		c.Request = c.Request.WithContext(logx.WithRequestID(c.Request.Context(), id))

		c.Next()
	}
}
