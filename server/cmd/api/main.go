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
