package router

import (
	"net/http"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/cache"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"

	// 引入生成的接口规范，使 Swagger UI 能读取到已注册的文档。
	_ "github.com/JunLang-7/sduzg-alumin-platform/server/internal/docs"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/handler"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/limiter"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

// redirectToSwaggerUI 把文档前缀跳转到入口页面。
func redirectToSwaggerUI(c *gin.Context) {
	c.Redirect(http.StatusFound, "/swagger/index.html")
}

func New(deps Dependencies) *gin.Engine {
	engine := gin.New()
	if err := engine.SetTrustedProxies(deps.Config.Server.TrustedProxies); err != nil && deps.Logger != nil {
		deps.Logger.Warn("failed to configure trusted proxies", zap.Error(err))
	}

	// 全局中间件
	engine.Use(
		middleware.RequestID(),
		middleware.CORS(deps.Config.CORS),
		middleware.Recovery(deps.Logger),
		middleware.RequestLogger(deps.Logger),
		middleware.RateLimit(deps.Config.RateLimit, limiter.New(deps.RedisClient), deps.Logger),
	)

	// 404 处理
	engine.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "route not found")
	})

	// 在线接口文档。挂在认证组之外，接口调用本身仍受各路由的鉴权与数据域校验约束。
	// gin-swagger 只用正则匹配具体文件名，裸前缀会被它当作未知文件返回 404，
	// 因此在通配处理器里先把 /swagger 与 /swagger/ 跳到入口页，便于手工输入地址访问。
	if deps.Config.Swagger.Enabled {
		engine.GET("/swagger", redirectToSwaggerUI)
		swaggerUI := ginSwagger.WrapHandler(swaggerFiles.Handler)
		engine.GET("/swagger/*any", func(c *gin.Context) {
			if c.Param("any") == "/" {
				redirectToSwaggerUI(c)
				return
			}

			swaggerUI(c)
		})
	}
	// 健康检查路由
	healthHandler := handler.NewHealthHandler(deps.Config, deps.DB, deps.RedisClient)
	// 用户仓库
	userRepository := repository.NewUserRepository(deps.DB)
	// 管理员数据域和功能权限仓库
	accessControlRepository := repository.NewAccessControlRepository(deps.DB)
	// 请求级授权上下文加载器
	accessContextLoader := middleware.NewAccessContextLoader(userRepository, accessControlRepository)
	// 校友仓库
	alumniRepository := repository.NewAlumniRepository(deps.DB)
	// 操作日志写入器
	opLogger := service.NewOperationLogger(deps.DB)
	// 登录尝试仓库
	loginAttemptRepository := repository.NewLoginAttemptRepository(deps.RedisClient)
	// 验证码仓库
	verifyCodeRepository := repository.NewVerifyCodeStore(deps.RedisClient)
	// 认证服务和处理器
	authService := service.NewAuthService(userRepository, alumniRepository, loginAttemptRepository, verifyCodeRepository, deps.Config, accessControlRepository).
		WithOperationLogger(opLogger)
	authHandler := handler.NewAuthHandler(authService)
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
	alumniService := service.NewAlumniService(alumniRepository, alumniFileCleaner, accessControlRepository).
		WithCountCache(cache.NewCountCache(deps.RedisClient)).
		WithExportCache(cache.NewExportCache(deps.RedisClient)).
		WithAuditUserStore(userRepository).
		WithOperationLogger(opLogger)
	alumniHandler := handler.NewAlumniHandler(alumniService)
	// 操作历史查询服务
	auditRepository := repository.NewAuditRepository(deps.DB)
	auditService := service.NewAuditService(auditRepository, alumniRepository)
	auditHandler := handler.NewAuditHandler(auditService)
	// 超级管理员服务和处理器
	adminService := service.NewAdminService(userRepository, accessControlRepository)
	adminHandler := handler.NewAdminHandler(adminService)
	// 院史共编：复用现有认证与数据域授权。
	historyRepository := repository.NewHistoryRepository(deps.DB)
	historyService := service.NewHistoryService(historyRepository, deps.StorageClient)
	historyHandler := handler.NewHistoryHandler(historyService)
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
	api.Use(
		middleware.Auth(authService, whiteList...),
		middleware.LoadAccessContext(accessContextLoader, whiteList...),
	)
	{
		// 健康检查
		api.GET("/health/live", healthHandler.Live)
		api.GET("/health/ready", healthHandler.Ready)

		// 用户认证
		api.POST("/auth/login", authHandler.Login)
		api.GET("/auth/me", authHandler.Me)
		api.POST("/auth/logout", authHandler.Logout)
		api.POST("/auth/change-password", authHandler.ChangePassword)
		api.POST("/auth/setup-password", authHandler.SetupPassword)
		api.POST("/auth/verify-code/send", authHandler.SendVerifyCode)

		// 通用档案查询仅供管理员使用；校友通过 /alumni/me 维护本人资料。
		api.GET("/alumni", middleware.RequireRoles(common.RoleAdmin, common.RoleSuperAdmin), alumniHandler.List)
		api.GET("/alumni/me", alumniHandler.Me)
		api.PUT("/alumni/me", alumniHandler.UpdateMe)
		api.PUT("/alumni/me/contact", authHandler.UpdateContact)
		api.GET("/alumni/:id", middleware.RequireRoles(common.RoleAdmin, common.RoleSuperAdmin), alumniHandler.Detail)

		// 院史共编（所有接口均需已登录；投稿与审核均由服务层校验角色和数据域）
		history := api.Group("/history")
		{
			history.GET("/entries", historyHandler.ListEntries)
			history.GET("/entries/:id", historyHandler.GetEntry)
			history.PUT("/entries/:id", historyHandler.UpdateEntry)
			history.DELETE("/entries/:id", historyHandler.DeleteEntry)
			history.POST("/entries/:id/deletion-requests", historyHandler.RequestEntryDeletion)
			history.GET("/deletion-requests/me", historyHandler.ListMyDeletionRequests)
			history.GET("/deletion-requests", historyHandler.ListPendingDeletionRequests)
			history.POST("/deletion-requests/:id/review", historyHandler.ReviewEntryDeletion)
			history.GET("/contributions/me", historyHandler.ListMine)
			history.GET("/contributions/:id", historyHandler.GetContribution)
			history.POST("/contributions", historyHandler.CreateDraft)
			history.PUT("/contributions/:id", historyHandler.UpdateContribution)
			history.DELETE("/contributions/:id", historyHandler.DeleteDraft)
			history.POST("/contributions/:id/submit", historyHandler.Submit)
			history.POST("/contributions/:id/attachments/upload-url", historyHandler.RequestAttachmentUpload)
			history.POST("/contributions/:id/attachments/:attachmentId/confirm", historyHandler.ConfirmAttachmentUpload)
			history.DELETE("/contributions/:id/attachments/:attachmentId", historyHandler.DeleteAttachment)
			history.PUT("/contributions/:id/attachments/:attachmentId", historyHandler.UpdateAttachment)
			history.GET("/contributions/:id/attachments", historyHandler.ListAttachments)
			history.GET("/contributions/:id/attachments/:attachmentId/download", historyHandler.AttachmentDownloadURL)
			history.GET("/reviews", historyHandler.ListPending)
			history.GET("/reviews/:id/attachments", historyHandler.ListAttachments)
			history.POST("/reviews/:id", historyHandler.Review)
		}

		// 管理员专用接口
		admin := api.Group("/admin")
		admin.Use(middleware.RequireRoles(common.RoleAdmin, common.RoleSuperAdmin))
		{
			// 管理校友信息
			admin.POST("/alumni", alumniHandler.Create)
			admin.PUT("/alumni/:id", alumniHandler.Update)
			admin.DELETE("/alumni/:id", alumniHandler.Delete)
			admin.POST("/alumni/import", alumniHandler.Import)
			admin.GET("/alumni/export", alumniHandler.Export)
			admin.GET("/alumni/template", alumniHandler.ExportTemplate)

			// 操作历史（只读，管理员及以上可访问）
			admin.GET("/audit/operations", auditHandler.List)
			admin.GET("/audit/operations/:id", auditHandler.Detail)

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
		superAdmin.Use(middleware.RequireRoles(common.RoleSuperAdmin))
		{
			// 管理员账号管理
			superAdmin.GET("/admins", adminHandler.List)
			superAdmin.POST("/admins", adminHandler.Create)
			superAdmin.GET("/admins/:id", adminHandler.Detail)
			superAdmin.PUT("/admins/:id/access", adminHandler.ReplaceAccess)
			superAdmin.DELETE("/admins/:id", adminHandler.Delete)
			superAdmin.GET("/data-domains", adminHandler.ListDataDomains)
		}
	}

	return engine
}
