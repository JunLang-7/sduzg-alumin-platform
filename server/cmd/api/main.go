package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/cache"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/database"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/router"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/storage"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// @title						政管全球校友数字化平台 API
// @version					1.0
// @description				山东大学政管学院校友数字化平台与院史在线平台的后端接口。
// @description				基础路径为 /api/v1。除健康检查与登录、首次设置密码、发送验证码外，其余接口都需要在 Authorize 中填入 `Bearer <access_token>`。
// @description				响应统一为 `{ code, message, data }`，成功 `code=0`。错误码、角色与数据域规则的完整说明见仓库 docs/api.md。
// @description				本页面的调用会真实访问当前服务，受同一套登录、角色、数据域与功能权限校验约束，未登录或越权调用不会被放过。
// @contact.name				项目仓库
// @contact.url				https://github.com/JunLang-7/sduzg-alumin-platform
// @tag.name					健康检查
// @tag.description			进程存活与依赖就绪状态，无需登录。
// @tag.name					认证与账号
// @tag.description			登录、退出、首次设置密码、验证码发送与当前用户信息。
// @tag.name					校友本人资料
// @tag.description			校友维护自己的资料与联系方式，不能访问他人档案。
// @tag.name					校友档案查询与管理
// @tag.description			管理员按数据域查询、新增、修改与删除校友档案。
// @tag.name					校友导入导出
// @tag.description			按模板批量导入与按筛选条件导出 Excel。
// @tag.name					校友档案文件
// @tag.description			档案文件的预签名上传、下载与删除，需独立的功能权限，且仅在启用对象存储时可用。
// @tag.name					数据大屏
// @tag.description			概览指标与分维度校友分布统计。
// @tag.name					管理员账号与授权
// @tag.description			仅超级管理员可用的账号、数据域与功能权限管理。
// @tag.name					操作历史
// @tag.description			校友档案变更与批量导入的审计记录。
// @tag.name					院史共编
// @tag.description			院史词条维护、投稿与交叉审核。
// @BasePath					/api/v1
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				填入 `Bearer <access_token>`，access_token 来自 POST /auth/login 或 POST /auth/setup-password。
func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// 初始化日志
	log, err := logger.New(cfg.App.Env)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = log.Sync()
	}()

	if cfg.App.Env == config.EnvProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化数据库连接
	db, err := database.Open(cfg.Database, log)
	if err != nil {
		log.Fatal("failed to initialize database", zap.Error(err))
	}
	defer func() {
		if err := database.Close(db); err != nil {
			log.Warn("failed to close database", zap.Error(err))
		}
	}()

	// 初始化 Redis 连接
	redisClient, err := cache.OpenRedis(cfg.Redis, log)
	if err != nil {
		log.Fatal("failed to initialize redis", zap.Error(err))
	}
	defer func() {
		if err := cache.CloseRedis(redisClient); err != nil {
			log.Warn("failed to close redis", zap.Error(err))
		}
	}()

	// 初始化对象存储客户端
	storageClient, err := storage.New(cfg.Storage, log)
	if err != nil {
		log.Fatal("failed to initialize storage client", zap.Error(err))
	}

	// 初始化路由
	engine := router.New(router.Dependencies{
		Config:        cfg,
		Logger:        log,
		DB:            db,
		RedisClient:   redisClient,
		StorageClient: storageClient,
	})

	// 初始化服务器
	server := &http.Server{
		Addr:              cfg.Server.Address(),
		Handler:           engine,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
	}

	// 异步启动服务器
	go func() {
		log.Info("api server started", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("api server stopped unexpectedly", zap.Error(err))
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("api server shutdown failed", zap.Error(err))
	}

	log.Info("api server stopped")
}
