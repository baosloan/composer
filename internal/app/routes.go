package app

import (
	"composer/internal/config"
	"composer/internal/modules/health"
	"composer/internal/platform/httpx"
	"composer/internal/platform/middleware"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewEngine 构建 Gin 引擎：先注册全局中间件，再挂载各模块的路由。
//
// 此函数负责决定路由挂载的路径，而不负责构造任何依赖对象，
// 那是 container.go 的职责。
// 将两者分离意味着添加一个新模块只需在此处修改一行、在 container.go 中修改一行，
// 而不是让一个函数既负责构造对象又负责挂载路由，变得越来越臃肿。
func NewEngine(c *Container) (*gin.Engine, error) {
	gin.SetMode(ginMode(c.Cfg.Server.Mode))

	engine := gin.New()

	// 仅信任已配置的代理。列表为空时，ClientIP() 返回直连对端地址，
	// 因此客户端无法通过 X-Forwarded-For 伪造自己的地址来绕过限流器。
	if err := engine.SetTrustedProxies(c.Cfg.Server.TrustedProxies); err != nil {
		return nil, err
	}
	engine.HandleMethodNotAllowed = true
	// 将 /users/ 重定向到 /users 在某些客户端上会把 POST 变成 GET；
	// 直接返回 404 更不容易造成混淆。
	engine.RedirectTrailingSlash = false

	// 校验错误消息中的字段名取自 json 标签，
	// 各模块 binding 标签中使用的自定义校验规则也会在这里统一注册。
	if err := httpx.SetupValidator(); err != nil {
		return nil, err
	}

	if err := registerGlobalMiddleware(engine, c); err != nil {
		return nil, err
	}
	engine.NoRoute(middleware.NoRoute())
	engine.NoMethod(middleware.NoMethod())

	// 健康探针和指标接口直接挂载在根路径下，位于 API 基础路径之外，
	// 同时也不经过身份认证：因为编排系统和指标采集器通常没有认证凭据。
	health.RegisterRoutes(engine, c.Health)
	if c.Cfg.Metrics.Enabled {
		engine.GET(c.Cfg.Metrics.Path, gin.WrapH(promhttp.Handler()))
	}

	return engine, nil
}

// registerGlobalMiddleware 注册应用于所有请求的全局中间件链。
//
// 中间件的顺序非常重要，执行关系可以理解为由外到内：
//
//	RequestID  最先注册，确保后续所有日志和响应都可以关联到同一个请求
//	Logger     尽量靠前，以便记录真实的响应状态码和请求耗时
//	Recovery   放在所有可能触发 panic 的逻辑之前
//	Metrics    放在限流器外层，这样被拒绝的请求也能够被统计
//	CORS       放在限流器之前，避免浏览器的预检请求被限流
//	RateLimit   为每个客户端应用令牌桶限流。
//	BodyLimit  放在任何读取请求体的处理逻辑之前
//	Timeout    放在最内层，使超时时间只覆盖实际的业务处理过程
func registerGlobalMiddleware(engine *gin.Engine, c *Container) error {
	engine.Use(
		middleware.RequestID(true),
		middleware.Logger(c.Logger, "/livez", "/readyz", "/health", c.Cfg.Metrics.Path),
		middleware.Recovery(),
	)

	if c.Cfg.Metrics.Enabled {
		engine.Use(middleware.Metrics(c.Cfg.App.Name))
	}

	engine.Use(
		middleware.CORS(c.Cfg.CORS),
		middleware.RateLimit(c.Cfg.RateLimit),
		middleware.BodyLimit(c.Cfg.Server.MaxBodyBytes),
		middleware.Timeout(c.Cfg.Server.RequestTimeout),
	)

	return nil
}

// ginMode 将应用的 server.mode 映射到 Gin 的运行模式。
func ginMode(mode string) string {
	switch mode {
	case config.ModeRelease:
		return gin.ReleaseMode
	case config.ModeTest:
		return gin.TestMode
	default:
		return gin.DebugMode
	}
}
