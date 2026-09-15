package config

import (
	"net"
	"strconv"
	"time"
)

// Server 运行模式常量（debug / test / release）。
const (
	ModeDebug   = "debug"
	ModeTest    = "test"
	ModeRelease = "release"
)

// DefaultJWTSecret 是 config/config.yaml 中自带的占位密钥。
// release 模式启动时会校验并拒绝此值，防止新生成的项目带着公开的密钥部署到生产环境。
const DefaultJWTSecret = "change-me-in-production"

// Config 为根配置对象，包含所有子配置项。
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Server    ServerConfig    `mapstructure:"server"`
	Log       LogConfig       `mapstructure:"log"`
	CORS      CORSConfig      `mapstructure:"cors"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
}

// AppConfig 用于保存应用的身份标识信息。
type AppConfig struct {
	Name string `mapstructure:"name"`
	// Env 是一个自由格式的部署标签（例如 local/dev/staging/prod），用于日志和监控指标中。
	// 程序行为由 Server.Mode 控制，而非本字段。
	Env string `mapstructure:"env"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	// Mode 为 debug|test|release，用于驱动 Gin 的运行模式、日志编码格式以及启动校验的严格程度。
	Mode string `mapstructure:"mode"`

	// BasePath 为所有注册路由添加前缀，例如 /api。
	BasePath string `mapstructure:"base_path"`

	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	// RequestTimeout 通过超时中间件限制处理器执行时间。
	// 应将其保持在 WriteTimeout 之下，以便客户端收到 504 状态码而非连接重置。
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	// MaxBodyBytes 限制请求体大小。设置为 0 表示不限制。
	MaxBodyBytes int64 `mapstructure:"max_body_bytes"`

	// TrustedProxies 是 Gin 信任的 CIDR/IP 列表，用于获取 X-Forwarded-For 头部。
	// 若列表为空，则表示不信任任何代理，此时 ClientIP() 返回的是直连对端地址。
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

// IsRelease 判断 Server 是否运行在生产模式（release）。
func (c ServerConfig) IsRelease() bool {
	return c.Mode == ModeRelease
}

// IsTest 判断 Server 是否运行在测试模式（test）。
func (c ServerConfig) IsTest() bool {
	return c.Mode == ModeTest
}

// IsDebug 判断 Server 是否运行在调试模式（debug）。
func (c ServerConfig) IsDebug() bool {
	return c.Mode == ModeDebug
}

// Addr 返回服务器绑定的主机和端口（host:port）。
func (c ServerConfig) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// LogConfig 包含日志记录器的配置。
type LogConfig struct {
	// Level 日志级别，可选值为 debug|info|warn|error。
	Level string `mapstructure:"level"`
	// Format 日志格式，可选值为 json|console。生产环境（release 模式）应使用 json。
	Format string `mapstructure:"format"`
	// Output 日志输出目标，可选值为 stdout|stderr|file。设置为 "file" 时，将应用 Rotate 滚动配置。
	Output string `mapstructure:"output"`
	// Filename 日志文件路径，仅在 Output 为 "file" 时生效。
	Filename string `mapstructure:"filename"`

	Rotate LogRotate `mapstructure:"rotate"`

	// Caller 和 Stacktrace 用于添加源码位置信息。
	// Stacktrace 仅对 error 级别及以上生效。
	Caller     bool `mapstructure:"caller"`
	Stacktrace bool `mapstructure:"stacktrace"`
}

// LogRotate 包含日志文件滚动配置（基于 lumberjack）。
type LogRotate struct {
	MaxSizeMB  int  `mapstructure:"max_size_mb"`  // 单个日志文件最大大小（MB），超过则触发滚动
	MaxBackups int  `mapstructure:"max_backups"`  // 保留的旧日志文件最大数量
	MaxAgeDays int  `mapstructure:"max_age_days"` // 旧日志文件保留的最大天数
	Compress   bool `mapstructure:"compress"`     // 是否压缩旧的日志文件（gzip）
}

// CORSConfig 包含跨域资源共享（CORS）配置。
//
// 与硬编码的通配符不同，本配置支持按环境灵活调整：
// release 模式下，当 AllowCredentials 为 true 时，会拒绝通配符来源（*），
// 因为 CORS 规范明确禁止这种组合，浏览器也会拒绝此类请求。
type CORSConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	AllowOrigins     []string      `mapstructure:"allow_origins"`
	AllowMethods     []string      `mapstructure:"allow_methods"`
	AllowHeaders     []string      `mapstructure:"allow_headers"`
	ExposeHeaders    []string      `mapstructure:"expose_headers"`
	AllowCredentials bool          `mapstructure:"allow_credentials"`
	MaxAge           time.Duration `mapstructure:"max_age"`
}

// RateLimitConfig 包含令牌桶限流器的配置。
type RateLimitConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// RPS 为每个客户端稳定的每秒请求数（持续速率），Burst 为瞬时突发容量。
	// 客户端按 IP 进行区分。
	RPS   float64 `mapstructure:"rps"`
	Burst int     `mapstructure:"burst"`
	// TTL 用于淘汰空闲的客户端令牌桶，防止内存无限增长。
	TTL time.Duration `mapstructure:"ttl"`
}

// MetricsConfig 包含 Prometheus 指标暴露配置。
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}
