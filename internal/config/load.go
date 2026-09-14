package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// EnvPrefix 为环境变量覆盖设置命名空间，例如 APP_DATABASE_MODE=cluster 会覆盖 database.mode。
const EnvPrefix = "APP"

func Load(path string) (*Config, error) {
	v := viper.New()

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/composer")
	}

	setDefaults(v)

	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !isConfigNotFound(err, &notFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	// AutomaticEnv 只能解析 Viper 已知的键。显式绑定每个键可以使环境变量覆盖
	// 仅出现在环境变量中的键，这在容器化部署中很常见。
	bindEnvs(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	cfg.normalize()

	// 在校验之前，将环境友好的地址列表展开为结构化节点，
	// 以便格式错误的地址能与其他配置问题一起被报告，
	// 而非单独报出。

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil

}

// isConfigNotFound 判断 err 是否为 Viper 的 "未找到配置文件" 错误。
func isConfigNotFound(err error, target *viper.ConfigFileNotFoundError) bool {
	if e, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok {
		*target = e
		return true
	}
	return false
}

// bindEnvs 绑定所有已知的配置键，使 AutomaticEnv 即使在配置文件中不存在该键时也能正确解析环境变量。
func bindEnvs(v *viper.Viper) {
	for _, key := range v.AllKeys() {
		_ = v.BindEnv(key)
	}
}

// normalize 将枚举类型的字段统一转为小写，并填充依赖其他值的字段。
// 在 Validate 之前执行，意味着校验时比较的是规范化后的标准形式，
// 用户不会因为写了 "Cluster" 而受到惩罚。
func (c *Config) normalize() {
	lower := func(s *string) { *s = strings.ToLower(strings.TrimSpace(*s)) }

	lower(&c.Server.Mode)
	lower(&c.Log.Level)
	lower(&c.Log.Format)
	lower(&c.Log.Output)



	// release 模式默认使用 JSON 日志格式：
	// 生产环境中使用人类可读的控制台编码器会破坏日志聚合。
	if c.Server.IsRelease() && c.Log.Format == "" {
		c.Log.Format = "json"
	}

	if c.Server.BasePath != "" {
		c.Server.BasePath = "/" + strings.Trim(c.Server.BasePath, "/")
	}
}

// setDefaults 为每个配置键声明一个合理的默认值。
//
// 这也作为可配置键的权威清单：由于 bindEnvs 会遍历 AllKeys，
// 在此注册的键会自动支持通过环境变量覆盖。
func setDefaults(v *viper.Viper) {

	// App.
	v.SetDefault("app.name", "composer")
	v.SetDefault("app.env", "local")

	// Server.
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", ModeDebug)
	v.SetDefault("server.base_path", "/api")
	v.SetDefault("server.read_timeout", 15*time.Second)
	v.SetDefault("server.write_timeout", 15*time.Second)
	v.SetDefault("server.idle_timeout", 60*time.Second)
	v.SetDefault("server.request_timeout", 10*time.Second)
	v.SetDefault("server.shutdown_timeout", 15*time.Second)
	v.SetDefault("server.max_body_bytes", 4<<20) // 4 MiB
	v.SetDefault("server.trusted_proxies", []string{})

	// Log.
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("log.output", "stdout")
	v.SetDefault("log.filename", "logs/app.log")
	v.SetDefault("log.rotate.max_size_mb", 100)
	v.SetDefault("log.rotate.max_backups", 7)
	v.SetDefault("log.rotate.max_age_days", 30)
	v.SetDefault("log.rotate.compress", true)
	v.SetDefault("log.caller", true)
	v.SetDefault("log.stacktrace", true)
}
