package router

import (
	"net/http"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/cache"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/handler"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Dependencies struct {
	Config        config.Config
	Logger        *zap.Logger
	DB            *gorm.DB
	RedisClient   *redis.Client
	StorageClient *storage.Client
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
	// 验证码仓库
	verifyCodeRepository := repository.NewVerifyCodeStore(deps.RedisClient)
	// 认证服务和处理器
	authService := service.NewAuthService(userRepository, alumniRepository, loginAttemptRepository, verifyCodeRepository, deps.Config)
	authHandler := handler.NewAuthHandler(authService)
	// 操作日志写入器
	opLogger := service.NewOperationLogger(deps.DB)
	// 校友文件仓库、服务和处理器（仅存储启用时注册）
	var alumniFileHandler *handler.AlumniFileHandler
	var alumniFileCleaner service.AlumniFileCleaner
	if deps.StorageClient != nil {
		alumniFileRepository := repository.NewAlumniFileRepository(deps.DB)
		alumniFileService := service.NewAlumniFileService(alumniFileRepository, alumniRepository, deps.StorageClient, opLogger)
		alumniFileHandler = handler.NewAlumniFileHandler(alumniFileService)
		alumniFileCleaner = alumniFileService
	}
	// 校友服务和处理器（注入文件服务以支持级联删除）
	alumniService := service.NewAlumniService(alumniRepository, userRepository, alumniFileCleaner).
		WithCountCache(cache.NewCountCache(deps.RedisClient)).
		WithExportCache(cache.NewExportCache(deps.RedisClient))
	alumniHandler := handler.NewAlumniHandler(alumniService)
	// 超级管理员服务和处理器
	adminService := service.NewAdminService(userRepository)
	adminHandler := handler.NewAdminHandler(adminService)
	// 数据大屏服务和处理器
	dashboardRepository := repository.NewDashboardRepository(deps.DB)
	dashboardService := service.NewDashboardService(dashboardRepository)
	dashboardHandler := handler.NewDashboardHandler(dashboardService)
	// 白名单路径
	whiteList := []string{
		"/api/v1/health/live",
		"/api/v1/health/ready",
		"/api/v1/auth/login",
		"/api/v1/auth/setup-password",
		"/api/v1/auth/verify-code/send",
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
		api.POST("/auth/setup-password", authHandler.SetupPassword)
		api.POST("/auth/verify-code/send", authHandler.SendVerifyCode)

		// 校友查询与更新
		api.GET("/alumni", alumniHandler.List)
		api.GET("/alumni/me", alumniHandler.Me)
		api.PUT("/alumni/me", alumniHandler.UpdateMe)
		api.PUT("/alumni/me/contact", authHandler.UpdateContact)
		api.GET("/alumni/:id", alumniHandler.Detail)

		// 管理员专用接口
		admin := api.Group("/admin")
		admin.Use(middleware.RequireRoles(userRepository, common.RoleAdmin, common.RoleSuperAdmin))
		{
			// 管理校友信息
			admin.POST("/alumni", alumniHandler.Create)
			admin.PUT("/alumni/:id", alumniHandler.Update)
			admin.DELETE("/alumni/:id", alumniHandler.Delete)
			admin.POST("/alumni/import", alumniHandler.Import)
			admin.GET("/alumni/export", alumniHandler.Export)
			admin.GET("/alumni/template", alumniHandler.ExportTemplate)

			// 管理校友文件（仅存储启用时注册）
			if alumniFileHandler != nil {
				admin.GET("/alumni/:id/files", alumniFileHandler.ListFiles)
				admin.POST("/alumni/:id/files/upload-url", alumniFileHandler.RequestUpload)
				admin.POST("/alumni/:id/files/:fileId/confirm", alumniFileHandler.ConfirmUpload)
				admin.GET("/alumni/:id/files/:fileId/download", alumniFileHandler.DownloadURL)
				admin.DELETE("/alumni/:id/files/:fileId", alumniFileHandler.Delete)
			}

			// 数据大屏
			admin.GET("/dashboard/overview", dashboardHandler.Overview)
			admin.GET("/dashboard/distribution", dashboardHandler.Distribution)
		}

		// 超级管理员专用接口
		superAdmin := api.Group("/super-admin")
		superAdmin.Use(middleware.RequireRoles(userRepository, common.RoleSuperAdmin))
		{
			// 管理员账号管理
			superAdmin.GET("/admins", adminHandler.List)
			superAdmin.POST("/admins", adminHandler.Create)
			superAdmin.DELETE("/admins/:id", adminHandler.Delete)
		}
	}

	return engine
}
