package app

import (
	"composer/internal/config"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewLogger 根据配置构建应用日志器。
func NewLogger(cfg *config.Config) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Log.Level)
	if err != nil {
		return nil, fmt.Errorf("log.level: %w", err)
	}

	writer, err := logWriter(cfg.Log)
	if err != nil {
		return nil, err
	}

	core := zapcore.NewCore(logEncoder(cfg.Log), writer, level)

	opts := []zap.Option{
		zap.Fields(
			zap.String("app", cfg.App.Name),
			zap.String("env", cfg.App.Env),
		),
	}
	if cfg.Log.Caller {
		opts = append(opts, zap.AddCaller(), zap.AddCallerSkip(0))
	}
	if cfg.Log.Stacktrace {
		// 堆栈跟踪仅从 error 级别及以上开始记录；
		// 附加到 warning 级别会淹没消息本身。
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	return zap.New(core, opts...), nil
}

// logEncoder 构建 JSON 或 console 编码器。
func logEncoder(cfg config.LogConfig) zapcore.Encoder {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.EncodeLevel = zapcore.CapitalLevelEncoder
	encCfg.EncodeDuration = zapcore.MillisDurationEncoder
	encCfg.TimeKey = "ts"
	encCfg.MessageKey = "msg"

	if cfg.Format == "json" {
		return zapcore.NewJSONEncoder(encCfg)
	}

	// Console 格式：为日志级别添加颜色以提高可读性。
	// 仅在本地使用，因此 ANSI 转义码不会污染日志聚合系统。
	encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zapcore.NewConsoleEncoder(encCfg)
}

// logWriter 解析并返回日志输出目标。
func logWriter(cfg config.LogConfig) (zapcore.WriteSyncer, error) {
	switch cfg.Output {
	case "file":
		// 创建目录而非直接报错：新拉取的项目不应在启动前需要手动执行 mkdir。
		if dir := filepath.Dir(cfg.Filename); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return nil, fmt.Errorf("create log directory %q: %w", dir, err)
			}
		}
		// lumberjack 按大小进行日志滚动并清理旧文件，防止无人值守的服务被日志写满磁盘。
		return zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxSize:    cfg.Rotate.MaxSizeMB,
			MaxBackups: cfg.Rotate.MaxBackups,
			MaxAge:     cfg.Rotate.MaxAgeDays,
			Compress:   cfg.Rotate.Compress,
			LocalTime:  true,
		}), nil

	case "stderr":
		return zapcore.Lock(os.Stderr), nil

	default:
		return zapcore.Lock(os.Stdout), nil
	}
}
