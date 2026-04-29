package router

import (
	"net/http"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/handler"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
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
	// 用户仓库
	userRepository := repository.NewUserRepository(deps.DB)
	// 校友仓库
	alumniRepository := repository.NewAlumniRepository(deps.DB)
	// 登录尝试仓库
	loginAttemptRepository := repository.NewLoginAttemptRepository(deps.RedisClient)
	// 认证服务和处理器
	authService := service.NewAuthService(userRepository, loginAttemptRepository, deps.Config)
	authHandler := handler.NewAuthHandler(authService)
	// 校友服务和处理器
	alumniService := service.NewAlumniService(alumniRepository, userRepository)
	alumniHandler := handler.NewAlumniHandler(alumniService)

	// 白名单路径
	whiteList := []string{
		"/api/v1/health/live",
		"/api/v1/health/ready",
		"/api/v1/auth/login",
	}

	api := engine.Group("/api/v1")
	api.Use(middleware.Auth(authService, whiteList...))
	{
		// 健康检查
		api.GET("/health/live", healthHandler.Live)
		api.GET("/health/ready", healthHandler.Ready)

		// 用户认证
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/logout", authHandler.Logout)
		api.POST("/auth/change-password", authHandler.ChangePassword)

		// 校友查询与更新
		api.GET("/alumni", alumniHandler.List)
		api.GET("/alumni/me", alumniHandler.Me)
		api.PUT("/alumni/me", alumniHandler.UpdateMe)
		api.GET("/alumni/:id", alumniHandler.Detail)
	}

	return engine
}
