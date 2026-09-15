package middleware

import (
	"composer/internal/config"
	"composer/internal/platform/errorx"
	"composer/internal/platform/httpx"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimit 为每个客户端应用令牌桶限流。
//
// 如果没有限流，登录接口可能会以网络所允许的最大速率遭受暴力破解。
// 当前限流器运行在单个进程内，因此当有 N 个副本时，实际的全局限流速率
// 大约会变成配置速率的 N 倍——这足以在一定程度上缓解攻击，
// 但当流量分散到多个实例后，更合适的方案是使用基于 Redis 的共享限流器。
func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	store := newLimiterStore(rate.Limit(cfg.RPS), cfg.Burst, cfg.TTL)

	return func(c *gin.Context) {
		// 以客户端 IP 作为限流键。
		// ClientIP 只会信任 server.trusted_proxies 中配置的代理所提供的
		// X-Forwarded-For，因此客户端无法通过伪造请求头绕过限流。
		limiter := store.get(c.ClientIP())

		if !limiter.Allow() {
			// Retry-After 可以让遵循规范的客户端主动退避，而不是持续发送请求，
			// 同时这也是大多数 HTTP 客户端库会识别的标准响应头。
			c.Header("Retry-After", strconv.Itoa(int(cfg.TTL.Seconds())))
			httpx.Error(c, errorx.ErrTooManyReqs)
			return
		}
		c.Next()
	}
}

// limiterStore 为每个客户端维护一个独立的令牌桶，
// 并负责清理长时间未使用的限流记录。
type limiterStore struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   rate.Limit
	burst   int
	ttl     time.Duration
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newLimiterStore 创建一个限流存储，并启动过期记录清理循环。
func newLimiterStore(limit rate.Limit, burst int, ttl time.Duration) *limiterStore {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	s := &limiterStore{
		buckets: make(map[string]*bucket),
		limit:   limit,
		burst:   burst,
		ttl:     ttl,
	}
	// 如果不进行清理，每出现一个新的客户端地址，map 中就会永久增加一条记录，
	// 对于公开接口来说，这最终会造成无上限的内存增长。
	go s.evictLoop()
	return s
}

// get 返回指定 key 对应的令牌桶；
// 如果该客户端第一次出现，则创建一个新的令牌桶。
func (s *limiterStore) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(s.limit, s.burst)}
		s.buckets[key] = b
	}
	b.lastSeen = time.Now()
	return b.limiter
}

// evictLoop 定期删除超过 TTL 时间未被访问的令牌桶。
func (s *limiterStore) evictLoop() {
	t := time.NewTicker(s.ttl)
	defer t.Stop()
	for now := range t.C {
		s.mu.Lock()
		for k, b := range s.buckets {
			if now.Sub(b.lastSeen) > s.ttl {
				delete(s.buckets, k)
			}
		}
		s.mu.Unlock()
	}
}
