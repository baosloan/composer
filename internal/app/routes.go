package app

import (
	"composer/internal/config"
	"composer/internal/modules/health"
	"composer/internal/platform/httpx"
	"composer/internal/platform/middleware"

	"github.com/gin-gonic/gin"
)

// NewEngine 构建 Gin 引擎：先注册全局中间件，再挂载各模块的路由。
//
// 此函数负责决定路由挂载的路径，而不负责构造任何依赖对象，
// 那是 container.go 的职责。
// 将两者分离意味着添加一个新模块只需在此处修改一行、在 container.go 中修改一行，
// 而不是让一个函数既负责构造对象又负责挂载路由，变得越来越臃肿。
func NewEngine(cfg *config.Config) (*gin.Engine, error) {
	gin.SetMode(ginMode(cfg.Server.Mode))

	engine := gin.New()

	// 仅信任已配置的代理。列表为空时，ClientIP() 返回直连对端地址，
	// 因此客户端无法通过 X-Forwarded-For 伪造自己的地址来绕过限流器。
	if err := engine.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
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

	if err := registerGlobalMiddleware(engine); err != nil {
		return nil, err
	}
	engine.NoRoute(middleware.NoRoute())
	engine.NoMethod(middleware.NoMethod())

	healthHandler := health.NewHandler("")
	health.RegisterRoutes(engine, healthHandler)

	return engine, nil
}

// registerGlobalMiddleware 注册应用于所有请求的全局中间件链。
//
// 中间件的顺序非常重要，执行关系可以理解为由外到内：
//
//		RequestID  最先注册，确保后续所有日志和响应都可以关联到同一个请求
//	 Logger     尽量靠前，以便记录真实的响应状态码和请求耗时
//		Recovery   放在所有可能触发 panic 的逻辑之前
func registerGlobalMiddleware(engine *gin.Engine) error {
	engine.Use(
		middleware.RequestID(true),
		//middleware.Logger(),
		middleware.Recovery(),
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
