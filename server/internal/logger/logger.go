package logger

import (
	"fmt"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	global = zap.NewNop()
	atom   = zap.NewAtomicLevelAt(zap.InfoLevel)
)

// New 根据环境创建一个新的 zap.Logger 实例
func New(env string) (*zap.Logger, error) {
	// 生产环境使用生产配置，其他环境使用开发配置
	if env == config.EnvProduction {
		cfg := zap.NewProductionConfig()
		cfg.Level = atom
		log, err := cfg.Build(zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
		if err != nil {
			return nil, err
		}
		SetLogger(log)
		return log, nil
	}

	cfg := zap.NewDevelopmentConfig()
	cfg.Level = atom
	// 开发环境启用彩色日志输出
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	log, err := cfg.Build(zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	if err != nil {
		return nil, err
	}
	SetLogger(log)
	return log, nil
}

// SetLogger 设置全局 logger 实例
func SetLogger(log *zap.Logger) {
	if log == nil {
		log = zap.NewNop()
	}
	global = log
	zap.ReplaceGlobals(log)
}

func L() *zap.Logger {
	return global
}

// SetLevel 动态设置日志级别
func SetLevel(level string) {
	parsedLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		fmt.Printf("invalid log level: %s\n", level)
		return
	}
	atom.SetLevel(parsedLevel)
}

func Debug(msg string, fields ...zap.Field) {
	global.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	global.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	global.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	global.Error(msg, fields...)
}
