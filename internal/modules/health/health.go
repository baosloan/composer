package health

import (
	"composer/internal/platform/httpx"
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Checker 用于报告某个依赖是否可用。
type Checker struct {
	// Name 会显示在探针的输出结果中。
	Name string

	// Check 用于检查依赖是否健康。
	// 当依赖健康时返回 nil。
	Check func(ctx context.Context) error

	// Critical 表示该依赖是否是应用正常提供服务所必需的。
	//
	// 如果非关键依赖（例如缓存、可选的消息代理）检查失败，
	// 系统会报告该异常，但仍允许当前副本以降级模式继续接收流量。
	Critical bool
}

// Handler 用于提供健康探针服务。
type Handler struct {
	version  string
	checkers []Checker
	// timeout 用于限制整个就绪探针检查过程的最长执行时间，
	// 防止某个依赖发生阻塞时导致探针本身一直阻塞，
	// 从而使 Pod 因错误的原因被终止。
	timeout time.Duration
}

// NewHandler 创建一个 Handler。
func NewHandler(version string, checkers ...Checker) *Handler {
	return &Handler{version: version, checkers: checkers, timeout: 3 * time.Second}
}

// status 表示单个依赖的检查结果。
type status struct {
	Name     string `json:"name"`
	Healthy  bool   `json:"healthy"`
	Critical bool   `json:"critical"`
	Error    string `json:"error,omitempty"`
	TookMS   int64  `json:"took_ms"`
}

// Live 处理 GET /livez 请求。
//
// 该接口有意不执行任何依赖检查：
// 只要能够访问到这个处理器，就已经证明当前进程能够正常调度 goroutine 并提供 HTTP 服务。
func (h *Handler) Live(c *gin.Context) {
	httpx.Success(c, gin.H{"status": "alive", "version": h.version})
}

// Ready 处理 GET /readyz 请求。
//
// 当任意关键依赖不可用时返回 503，
// 从而将当前副本从负载均衡器的流量分发列表中移除。
func (h *Handler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	results := h.runChecks(ctx)

	ready := true
	for _, r := range results {
		if !r.Healthy && r.Critical {
			ready = false
			break
		}
	}

	body := gin.H{
		"status":  map[bool]string{true: "ready", false: "not_ready"}[ready],
		"version": h.version,
		"checks":  results,
	}

	if !ready {
		// 这里直接写入响应，而不是通过 httpx.Error：
		// 因为检查详情才是响应中真正有用的信息，
		// 而错误响应包装中没有用于承载这些数据的 data 字段。
		c.JSON(http.StatusServiceUnavailable, httpx.Response{
			Code:    0,
			Message: "not ready",
			Data:    body,
		})
		return
	}
	httpx.Success(c, body)
}

// runChecks 会并发检查所有依赖。
//
// 如果按顺序逐个执行检查，就绪探针的总耗时会变成所有超时时间之和，
// 这样很容易导致探针本身超过自己的截止时间。
func (h *Handler) runChecks(ctx context.Context) []status {
	results := make([]status, len(h.checkers))

	var wg sync.WaitGroup
	for i, checker := range h.checkers {
		wg.Add(1)
		go func(i int, ch Checker) {
			defer wg.Done()

			start := time.Now()
			err := ch.Check(ctx)

			results[i] = status{
				Name:     ch.Name,
				Healthy:  err == nil,
				Critical: ch.Critical,
				TookMS:   time.Since(start).Milliseconds(),
			}
			if err != nil {
				results[i].Error = err.Error()
			}
		}(i, checker)
	}
	wg.Wait()

	return results
}

// RegisterRoutes 注册健康探针路由。
//
// 这些探针路由直接注册在引擎根路径下，不使用 API 基础路径，
// 也不经过身份认证：因为编排系统需要能够访问探针，而它通常没有认证凭据。
func RegisterRoutes(r gin.IRouter, h *Handler) {
	r.GET("/livez", h.Live)
	r.GET("/readyz", h.Ready)

	// /health 作为别名保留，用于兼容那些默认使用这一常规名称的工具；
	// 它映射到就绪探针，因为在这两种探针中，就绪探针通常更有实际意义。
	r.GET("/health", h.Ready)
}
