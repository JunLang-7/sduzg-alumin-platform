package service

import (
	"context"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// OperationLogger 操作日志写入器，封装日志序列化和持久化。
type OperationLogger struct {
	db *gorm.DB
}

// NewOperationLogger 创建操作日志写入器。
func NewOperationLogger(db *gorm.DB) *OperationLogger {
	return &OperationLogger{db: db}
}

// Write 写入操作日志。如果 DB 不可用则静默跳过。
func (l *OperationLogger) Write(ctx context.Context, log *model.OperationLog) error {
	if l == nil || l.db == nil {
		return nil
	}

	if err := l.db.WithContext(ctx).Create(log).Error; err != nil {
		logger.Warn("failed to persist operation log",
			zap.String("action", log.Action),
			zap.Error(err),
		)
		return err
	}
	return nil
}
