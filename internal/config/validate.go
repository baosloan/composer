package config

import (
	"errors"
	"fmt"
	"net"
	"slices"
)

func (c *Config) Validate() error {
	var errs []error
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	c.validateServer(add)
	c.validateLog(add)


	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration:\n%w", errors.Join(errs...))
}

type addFunc func(format string, args ...any)

// validateServer 验证服务配置
func (c *Config) validateServer(add addFunc) {
	if !slices.Contains([]string{ModeDebug, ModeTest, ModeRelease}, c.Server.Mode) {
		add("server.mode: %q is not one of debug|test|release", c.Server.Mode)
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		add("server.port: %d is out of range 1-65535", c.Server.Port)
	}
	// The handler timeout must be shorter than the write timeout, or the
	// connection dies before the 504 can be written.
	if c.Server.RequestTimeout > 0 && c.Server.WriteTimeout > 0 &&
		c.Server.RequestTimeout >= c.Server.WriteTimeout {
		add("server.request_timeout (%s) must be less than server.write_timeout (%s), "+
			"otherwise the client sees a dropped connection instead of a timeout response",
			c.Server.RequestTimeout, c.Server.WriteTimeout)
	}
	for _, p := range c.Server.TrustedProxies {
		if _, _, err := net.ParseCIDR(p); err != nil && net.ParseIP(p) == nil {
			add("server.trusted_proxies: %q is neither an IP nor a CIDR block", p)
		}
	}
}

// validateLog 验证日志配置
func (c *Config) validateLog(add addFunc) {
	if !slices.Contains([]string{"debug", "info", "warn", "error"}, c.Log.Level) {
		add("log.level: %q is not one of debug|info|warn|error", c.Log.Level)
	}
	if !slices.Contains([]string{"json", "console"}, c.Log.Format) {
		add("log.format: %q is not one of json|console", c.Log.Format)
	}
	if !slices.Contains([]string{"stdout", "stderr", "file"}, c.Log.Output) {
		add("log.output: %q is not one of stdout|stderr|file", c.Log.Output)
	}
	if c.Log.Output == "file" && c.Log.Filename == "" {
		add("log.filename: is required when log.output=file")
	}
}