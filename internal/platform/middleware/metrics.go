package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 用于记录 HTTP 请求数量、请求耗时以及当前正在处理的请求数。
//
// 标签使用 c.FullPath() 获取路由模板，例如 /api/v1/users/:id，
// 而不会直接使用原始 URL。
//
// 如果直接使用原始请求路径，会因为不同的 id 产生大量不同的时间序列，
// 从而导致 Prometheus 指标基数急剧膨胀。
// 这也是因为一次指标改动而拖垮整个监控系统的典型问题之一。
func Metrics(namespace string) gin.HandlerFunc {
	reg := newHTTPMetrics(namespace)

	return func(c *gin.Context) {
		route := c.FullPath()
		if route == "" {
			// 对于未匹配到路由的请求，统一归到一个固定标签下，
			// 避免扫描器随机探测大量 URL 时创建无限增长的时间序列。
			route = "<unmatched>"
		}

		reg.inFlight.Inc()
		start := time.Now()

		c.Next()

		reg.inFlight.Dec()
		status := strconv.Itoa(c.Writer.Status())
		reg.requests.WithLabelValues(c.Request.Method, route, status).Inc()
		reg.duration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
		reg.respSize.WithLabelValues(c.Request.Method, route).Observe(float64(c.Writer.Size()))
	}
}

// httpMetrics 将所有 HTTP 指标收集器集中管理， 确保这些指标只被注册一次。
type httpMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	respSize *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

// newHTTPMetrics 将 HTTP 指标收集器注册到 Prometheus 默认注册表中。
func newHTTPMetrics(namespace string) *httpMetrics {
	return &httpMetrics{
		requests: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests.",
		}, []string{"method", "route", "status"}),

		duration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency.",
			// 桶范围从 5ms 到 10s，
			// 基本覆盖从缓存命中到请求超时的常见场景。
			// Prometheus 默认桶对于 API 请求来说起始粒度通常过粗。
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"method", "route"}),

		respSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size.",
			Buckets:   prometheus.ExponentialBuckets(64, 4, 8),
		}, []string{"method", "route"}),

		inFlight: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being served.",
		}),
	}
}
