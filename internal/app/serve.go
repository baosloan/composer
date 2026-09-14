package app

import (
	"composer/internal/config"

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

	engine.GET("/livez", func(c *gin.Context) { c.JSON(200, gin.H{"status": "alive"}) })
	return engine, nil
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
