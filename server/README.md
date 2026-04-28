# 山大政管学院校友平台后端

一期后端采用 Go + Gin + GORM + MySQL，按 `handler -> service -> repository -> model -> database` 分层组织。

## 本地启动

配置由 Viper 加载，默认读取当前目录或 `./server` 下的 `.env`，环境变量会覆盖 `.env` 文件。

```bash
cd server
cp .env.example .env
go mod tidy
go run ./cmd/api
```

默认不强制连接数据库，便于先验证服务骨架。需要连接 MySQL 时设置：

```bash
DB_ENABLED=true
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=sdu_alumni
DB_PASSWORD=sdu_alumni_password
DB_NAME=sdu_alumni
```

需要启用 Redis 时设置：

```bash
REDIS_ENABLED=true
REDIS_ADDR=127.0.0.1:6379
REDIS_DB=0
```

## Docker Compose

```bash
docker compose up --build
```

启动后可检查：

```bash
curl http://127.0.0.1:8080/api/v1/health/live
curl http://127.0.0.1:8080/api/v1/health/ready
```

## GORM 代码生成

后续数据表变更后，可以使用 GORM gentool 从当前数据库生成模型代码。

```bash
go install gorm.io/gen/tools/gentool@latest
make gendb
```

生成配置在 `server/gen.yml`，默认生成 `users`、`alumni_profiles`、`operation_logs` 的 GORM 模型到 `server/internal/model`。如果新增表或数据库连接变化，直接改 `gen.yml` 后重新执行 `make gendb`。

## 当前初始化范围

- Gin API 服务入口
- 环境变量配置
- zap 结构化日志
- 统一 JSON 响应
- 请求 ID、访问日志、panic recovery 中间件
- MySQL/GORM 连接辅助
- Redis 连接辅助，供登录态、鉴权缓存和限流扩展使用
- 健康检查接口
- MySQL 8.0 初始化脚本
