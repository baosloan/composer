package middleware

import (
	"composer/internal/config"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS 根据配置应用跨域响应头。
//
// 允许的来源列表通过配置提供，而不是直接硬编码为 "*"：
// 因为通配符不能与凭证请求同时使用，而且如果在生产环境中使用 "*"，
// 会导致 API 暴露给已登录用户访问的任意网站。
//
// 配置校验会在 release 模式下拒绝 "*"，
// 这样就不需要依赖某次容易被遗忘的代码修改来收紧跨域策略。
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	allowAll := slices.Contains(cfg.AllowOrigins, "*")
	methods := strings.Join(cfg.AllowMethods, ", ")
	headers := strings.Join(cfg.AllowHeaders, ", ")
	expose := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := strconv.Itoa(int(cfg.MaxAge.Seconds()))

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			// 不是跨域请求，无需进行 CORS 协商。
			c.Next()
			return
		}

		allowed := allowAll || slices.Contains(cfg.AllowOrigins, origin)
		if !allowed {
			// 不返回 CORS 响应头就是正确的拒绝方式：浏览器会自行拦截该响应。
			// 但预检请求仍然不能继续进入后续处理器，因此需要在这里直接返回。
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		// 当请求涉及凭证时，应回显调用方的 Origin，而不是使用 "*"，
		// 因为浏览器在这种情况下会拒绝通配符。
		//
		// Vary 响应头用于告知缓存：响应内容会因不同的 Origin 而有所不同。
		if allowAll && !cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}

		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if expose != "" {
			c.Header("Access-Control-Expose-Headers", expose)
		}

		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", methods)
			c.Header("Access-Control-Allow-Headers", headers)
			c.Header("Access-Control-Max-Age", maxAge)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
