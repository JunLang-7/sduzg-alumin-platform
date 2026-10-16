# 后端开发

后端采用 Go、Gin、GORM、MySQL 和 Redis。完整环境的端口、Compose 配置和迁移方式见[部署说明](../docs/deployment.md)，提交检查与发布流程见[贡献指南](../CONTRIBUTING.md)。

## 本地启动

使用 [go.mod](go.mod) 指定的 Go 工具链，从仓库根目录执行：

```bash
cd server
cp .env.example .env
go mod download
go run ./cmd/api
```

已有 `server/.env` 时保留原配置。Viper 读取当前目录或 `./server` 下的 `.env`，同名环境变量优先于文件。

默认 `DB_ENABLED=false`、`REDIS_ENABLED=false`，可先运行服务骨架；这不提供依赖数据库的完整业务功能。连接 Compose 中的服务时，在 `server/.env` 启用并配置：

```env
DB_ENABLED=true
DB_HOST=127.0.0.1
DB_PORT=3307
DB_USER=sdu_alumni
DB_NAME=sdu_alumni_db

REDIS_ENABLED=true
REDIS_ADDR=127.0.0.1:6379
```

另外将 `DB_PASSWORD` 设置为根 `.env` 的 `MYSQL_PASSWORD`，`REDIS_PASSWORD` 与 Compose 使用的值一致；已调整宿主机端口时同步修改连接地址。需要文件服务时再启用 `STORAGE_ENABLED` 并配置对象存储连接，`STORAGE_ACCESS_KEY`／`STORAGE_SECRET_KEY` 分别与根 `.env` 的 `MINIO_ROOT_USER`／`MINIO_ROOT_PASSWORD` 保持一致。完整字段见 [.env.example](.env.example)。

需要数据库和 Redis 时，可先在仓库根目录执行 `docker compose up -d mysql redis`，等待服务就绪后执行 `docker compose run --rm migrate`。本地 API 与 Compose API 默认都使用 8080，开发时只运行其中一个。

健康检查：

```bash
curl --fail http://127.0.0.1:8080/api/v1/health/live
curl --fail http://127.0.0.1:8080/api/v1/health/ready
```

开发环境默认开启在线接口文档，启动后可访问 `http://127.0.0.1:8080/swagger/index.html`，在 Authorize 中填入 `Bearer <access_token>` 即可直接调试接口。默认值取决于 `APP_ENV`：非生产环境开启、生产环境关闭，可用 `SWAGGER_ENABLED` 覆盖，详见[部署说明](../docs/deployment.md#在线接口文档)。

## 代码结构与约定

业务调用遵循 `handler → service → repository → model/database`：

| 目录                                      | 职责                                          |
| ----------------------------------------- | --------------------------------------------- |
| `cmd/api`                                 | 入口、依赖初始化与服务生命周期                |
| `internal/router`、`internal/handler`     | `/api/v1` 路由、请求绑定与 HTTP 响应          |
| `internal/service`、`internal/repository` | 业务逻辑与数据访问                            |
| `internal/model`、`internal/query`        | GORM Gen 生成模型与查询代码                   |
| `internal/config`、`internal/response`    | 配置加载、统一 `{ code, message, data }` 响应 |
| `internal/docs`                           | 由 handler 注解生成的接口规范，不要手工修改  |
| `migrations`、`scripts`                   | 有序 SQL 迁移与运维验证脚本                   |

具体权限、错误处理与生成代码要求见 [AGENTS.md](../AGENTS.md)。

## 测试与工具

后端测试在 `server/` 执行：

```bash
go test -race -count=1 ./...
```

MySQL、Redis 和对象存储集成测试需要隔离服务及对应 `TEST_*` 配置，说明见[贡献指南的本地检查](../CONTRIBUTING.md#本地检查)。

模型生成与限流验证在仓库根目录执行：

```bash
# 表结构变化后，先核对 server/gen.yml 的 DSN 和表清单。
go install gorm.io/gen/tools/gentool@latest
make gendb

# 需已启动 MySQL、Redis，并按脚本要求配置后端环境。
./server/scripts/rate_limit_boundary_test.sh
```

[gen.yml](gen.yml) 管理数据库连接和生成范围，生成结果位于 `internal/model` 和 `internal/query`，不要手工修改生成文件；`gen.yml` 中的 DSN 指向本地 Compose MySQL（宿主机 3307），需先应用对应迁移再执行 `make gendb`。工具链升级时同步核对 Dockerfile、CI 与开发环境。

## 接口文档注解

接口规范由 handler 上的 swag 注解生成，结果提交在 `internal/docs`（`docs.go`、`swagger.json`），由 `internal/router` 挂载到 `/swagger/*any`。新增或修改接口时：

1. 在 handler 方法上补 `@Summary`、`@Tags`、`@Param`、`@Success`、`@Failure`、`@Security` 和 `@Router` 注解；响应沿用 `response.Body{data=dto.X}` 组合，不要新增包装类型。
2. 在仓库根目录运行 `make swagger`（首次需先 `make swagger-tool`）重新生成，并提交生成结果。
3. 运行 `go test ./internal/docs/...`，它会按当前注解重新生成规范并与实际注册的路由双向比对，缺少注解时会指出具体的 `METHOD /path`。

分页响应因为 swag 稳定版无法表达 `common.Pager[T]` 泛型，在 `internal/dto/page.go` 中按相同字段单独声明，由 `page_shape_test.go` 断言与 `common.Pager[T]` 一致。接口文档入口开关为 `SWAGGER_ENABLED`，默认非生产环境开启、生产环境关闭。
