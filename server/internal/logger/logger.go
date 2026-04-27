package logger

import (
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 根据环境创建一个新的 zap.Logger 实例
func New(env string) (*zap.Logger, error) {
	// 生产环境使用生产配置，其他环境使用开发配置
	if env == config.EnvProduction {
		return zap.NewProduction()
	}

	cfg := zap.NewDevelopmentConfig()
	// 在开发环境中启用颜色输出
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return cfg.Build()
}
