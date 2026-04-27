package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthHandler struct {
	appName   string
	env       string
	startedAt time.Time
	db        *gorm.DB
	redis     *redis.Client
}

func NewHealthHandler(cfg config.Config, db *gorm.DB, redisClient *redis.Client) *HealthHandler {
	return &HealthHandler{
		appName:   cfg.App.Name,
		env:       cfg.App.Env,
		startedAt: time.Now(),
		db:        db,
		redis:     redisClient,
	}
}

// Live 处理 liveness 探针请求
func (h *HealthHandler) Live(c *gin.Context) {
	response.Success(c, gin.H{
		"status":     "ok",
		"app":        h.appName,
		"env":        h.env,
		"started_at": h.startedAt.Format(time.RFC3339),
	})
}

// Ready 处理 readiness 探针请求，检查数据库和 Redis 连接
func (h *HealthHandler) Ready(c *gin.Context) {
	result := gin.H{
		"status":   "ok",
		"database": "disabled",
		"redis":    "disabled",
	}

	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := sqlDB.PingContext(ctx); err != nil {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
			return
		}

		result["database"] = "connected"
	}

	if h.redis != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := h.redis.Ping(ctx).Err(); err != nil {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "redis is unavailable")
			return
		}

		result["redis"] = "connected"
	}

	response.Success(c, result)
}
