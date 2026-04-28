package router

import (
	"net/http"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/handler"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Dependencies struct {
	Config      config.Config
	Logger      *zap.Logger
	DB          *gorm.DB
	RedisClient *redis.Client
}

func New(deps Dependencies) *gin.Engine {
	engine := gin.New()

	// 全局中间件
	engine.Use(
		middleware.RequestID(),
		middleware.Recovery(deps.Logger),
		middleware.RequestLogger(deps.Logger),
	)

	// 404 处理
	engine.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "route not found")
	})

	// 健康检查路由
	healthHandler := handler.NewHealthHandler(deps.Config, deps.DB, deps.RedisClient)

	api := engine.Group("/api/v1")
	{
		// 健康检查
		api.GET("/health/live", healthHandler.Live)
		api.GET("/health/ready", healthHandler.Ready)
	}

	return engine
}
