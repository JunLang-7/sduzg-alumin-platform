package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
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
//
//	@Summary		存活探针
//	@Description	仅表示进程存活，不检查任何下游依赖。无需登录。
//	@Tags			健康检查
//	@Produce		json
//	@Success		200	{object}	response.Body{data=dto.HealthLiveResult}
//	@Router			/health/live [get]
func (h *HealthHandler) Live(c *gin.Context) {
	response.Success(c, dto.HealthLiveResult{
		Status:    "ok",
		App:       h.appName,
		Env:       h.env,
		StartedAt: h.startedAt.Format(time.RFC3339),
	})
}

// Ready 处理 readiness 探针请求，检查数据库和 Redis 连接
//
//	@Summary		就绪探针
//	@Description	检查已启用的数据库与 Redis 连接；未启用的依赖为 disabled。不检查对象存储、短信或邮件。无需登录。
//	@Tags			健康检查
//	@Produce		json
//	@Success		200	{object}	response.Body{data=dto.HealthReadyResult}
//	@Failure		503	{object}	response.Body	"数据库或 Redis 不可用"
//	@Router			/health/ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	result := dto.HealthReadyResult{
		Status:   "ok",
		Database: "disabled",
		Redis:    "disabled",
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

		result.Database = "connected"
	}

	if h.redis != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := h.redis.Ping(ctx).Err(); err != nil {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "redis is unavailable")
			return
		}

		result.Redis = "connected"
	}

	response.Success(c, result)
}
