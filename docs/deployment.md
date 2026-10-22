# 部署说明

[文档目录](README.md) · [项目首页](../README.md)

## 启动完整环境

在已安装 Docker 与 Docker Compose 的机器上，从仓库根目录执行：

```bash
cp .env.example .env
docker compose up --build
```

已有 `.env` 时直接使用原配置。Compose 启动 MySQL、Redis、Silo 对象存储、数据库迁移任务、API 和 Web；Silo 兼容 MinIO 接口，服务名及配置项保留 `minio`／`MINIO_*` 命名。

| 服务           | 默认地址                                   | 根 `.env` 中的端口配置         |
| -------------- | ------------------------------------------ | ------------------------------ |
| Web            | `http://127.0.0.1`                         | `WEB_HOST_PORT=80`             |
| API            | `http://127.0.0.1:8080`                    | `API_HOST_PORT=8080`           |
| MySQL          | `127.0.0.1:3307`                           | `MYSQL_HOST_PORT=3307`         |
| Redis          | `127.0.0.1:6379`                           | `REDIS_HOST_PORT=6379`         |
| 对象存储 API   | `http://127.0.0.1:9000`                    | `MINIO_API_HOST_PORT=9000`     |
| 对象存储控制台 | `http://127.0.0.1:9001`                    | `MINIO_CONSOLE_HOST_PORT=9001` |
| 接口文档       | `http://127.0.0.1:8080/swagger/index.html` | `SWAGGER_ENABLED`              |

初始化迁移提供开发账号：`admin` / `Admin@123456`，角色为 `super_admin`。公开部署前必须更换默认管理员密码、数据库与对象存储密码，并向 API 容器传入独立的 `AUTH_JWT_SECRET`；同时确认接口文档已关闭（见下文）。

## 配置放在哪里

| 配置文件                                      | 作用                                                                             |
| --------------------------------------------- | -------------------------------------------------------------------------------- |
| [根 `.env.example`](../.env.example)          | 复制为根 `.env`，供 Compose 替换端口、数据库、对象存储、构建源、短信、邮件等变量 |
| [后端 `.env.example`](../server/.env.example) | 复制为 `server/.env`，供本地 `go run` 读取 API 配置                              |
| [前端 `.env.example`](../web/.env.example)    | 前端构建与开发配置，默认接口基础路径为 `/api/v1`                                 |

Compose 的 API 通过 [docker-compose.yml](../docker-compose.yml) 中的 `environment` 接收配置，镜像不会复制或挂载 `server/.env`。仅在根 `.env` 添加变量也不会自动把它传入容器；例如生产 JWT 密钥需在部署编排中显式传递 `AUTH_JWT_SECRET`，可使用下面的本地覆盖文件：

```yaml
# compose.production.yaml（本地维护，不提交真实密钥）
services:
  api:
    environment:
      AUTH_JWT_SECRET: ${AUTH_JWT_SECRET:?请设置生产 JWT 密钥}
```

在部署环境设置 `AUTH_JWT_SECRET` 后，用 `docker compose -f docker-compose.yml -f compose.production.yaml up --build -d` 启动。其他 API 配置参照 [后端配置示例](../server/.env.example) 显式传递；配置加载规则见[后端开发说明](../server/README.md)。

常用配置按用途分为：

- 数据库与缓存：Compose 使用 `MYSQL_*`、`REDIS_*`；本地 API 使用 `DB_*`、`REDIS_*`。
- 文件存储：Compose 使用 `MINIO_*`，并将连接参数传为 API 的 `STORAGE_*`。
- 认证：`AUTH_JWT_SECRET`、`AUTH_ACCESS_TOKEN_TTL`。
- 通知：腾讯云短信 `SMS_TENCENT_*`、SMTP 邮件 `EMAIL_*`；开发环境默认关闭。
- 访问控制：`RATE_LIMIT_*`、`CORS_*`；API 位于反向代理后时配置 `SERVER_TRUSTED_PROXIES`。
- 接口文档：`SWAGGER_ENABLED`；未设置时非生产环境开启、`APP_ENV=production` 时关闭。

不提交真实 `.env`、生产密钥、日志或构建产物。示例文件与编排中的默认值用于启动和开发，部署时按实际环境设置。

## 时间与时区契约

- 数据库业务时间列（包括 `operation_logs.created_at`）使用 `DATETIME` 保存 UTC 墙上时间；`DATETIME` 本身不携带时区。
- 默认、示例与 Compose 的 API 数据库连接使用 `DB_PARAMS=charset=utf8mb4&parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27`。`loc=UTC` 控制驱动按 UTC 读写时间，`time_zone` 解码为 SQL 字符串 `'+00:00'`，固定每条连接的会话时区，保证 `CURRENT_TIMESTAMP` 也按 UTC 写入；只配置其中一项不够。Compose 同时显式设置 MySQL 的 `--default-time-zone=+00:00`，避免默认会话依赖宿主机时区。
- API 将这些时间以 RFC3339 UTC 瞬时（`Z` 结尾）返回；Web 统一按**浏览器本地时区**展示，包括操作历史列表、详情抽屉、校友历史卡片、院史审核/词条日期和大屏时钟。院史审核不再固定使用北京时间，海外管理员看到的是当地日期。
- Web 操作历史筛选把本地开始日 00:00、结束日次日 00:00 转换为 `start_at` / `end_at`，区间左闭右开；夏令时切换日可能是 23 或 25 小时，不能简单加 24 小时。旧 `start_date` / `end_date` 仍按 UTC 日边界解析。

升级时检查并更新已有 `server/.env` 或部署覆盖文件中的 `DB_PARAMS`；旧文件不会随示例自动更新，不能保留 `loc=Local`，也不要通过给 API 设置本地 `TZ` 来修正业务时间。新默认连接参数不依赖 API 进程的 `TZ`。MySQL 启动参数需重建容器才能生效，保留原数据卷。

以上约定基于现有数据已经存储为 UTC，不会改写历史记录。如果自定义部署曾以本地时间写入 `DATETIME`，应先备份并核实数据的真实时区，再制定单独的数据校正方案，不能直接批量加减 8 小时。

## 静态资源与缓存策略

Web 容器（`web/nginx.conf`）的缓存策略决定发版后用户能否立刻拿到新前端：

- `/index.html`、根路径与 SPA 深层路由回退：`Cache-Control: no-cache`。允许缓存，但每次使用前向服务器校验，配合 ETag 命中时返回 304。
- `/assets/`：`Cache-Control: public, max-age=31536000, immutable`。Vite 产物文件名带内容 hash，内容变化会生成新文件名，因此可以长期缓存。
- `/api/`、`/sdu-alumni-files/` 是代理路径，不要给它们加缓存头。

注意 nginx 的 `add_header` **不继承**：某个 location 内出现 `add_header` 后，上层/server 级同类指令会被整体覆盖。不要在 server 级统一加缓存头，否则代理路径也会被误加；新增 location 时应单独声明所需头部。

CI 的 `containers` 任务会启动 Web 镜像，断言上述响应头以及 `If-None-Match` 返回 304（`.github/workflows/ci.yml` 的 `Verify web container serves the built application`）。修改缓存策略时要同步更新断言，不要只删除头部。`server_tokens off;` 用于隐藏 nginx 版本号。

## 在线接口文档

Swagger UI 挂在 API 服务的 `/swagger/index.html`，接口规范端点为 `/swagger/doc.json`。它可以浏览全部接口、查看参数与响应结构，并在 Authorize 中填入 `Bearer <access_token>` 后直接发起真实请求；这些请求与手工调用一样受登录、角色、数据域和功能权限校验，不会因为从文档发起而放宽。[接口文档](api.md)是接口约定的文字说明，两者内容一致。文档只由 API 服务提供，Web 容器（默认 80 端口）不代理该路径：从 Web 入口访问 `/swagger/index.html` 只会返回前端页面，请直接访问上表中的 API 地址。

是否挂载由 `SWAGGER_ENABLED` 决定，未设置时的默认值取决于 `APP_ENV`：

- `APP_ENV=production` 且未设置 `SWAGGER_ENABLED`（如直接运行生产构建的二进制）：默认关闭，`/swagger/index.html` 与 `/swagger/doc.json` 都返回 HTTP 404。
- 其他环境（如本地 `go run ./cmd/api`）：默认开启。

注意 [docker-compose.yml](../docker-compose.yml) 中 API 的 `APP_ENV` 为 `production`，但同时把 `SWAGGER_ENABLED` 默认传为 `true`，所以 `docker compose up --build` 后默认**开启**接口文档，可以直接访问。该编排带有默认管理员账号，用它对外公开部署时，必须在编排中显式设为 `false`：

```yaml
# compose.production.yaml
services:
  api:
    environment:
      SWAGGER_ENABLED: "false"
```

接口文档关闭只影响这两个地址，不影响任何业务接口。规范由 handler 注解生成，重新生成与校验方式见[后端开发说明](../server/README.md)。

## 历史公式前缀数据清理

导出时为防止电子表格公式注入，会对以 `= + - @` 开头的值加 `'` 前缀。旧版本在「导出 → 回灌」时可能把这个前缀当成真实内容写回数据库，例如手机号被存成 `'+8613800000000`。本次修复只保证新导入会还原前缀，**不会自动修正历史记录**。

排查（只读）：

```sql
SELECT COUNT(*) FROM alumni_profiles
WHERE name LIKE '''%''' OR mobile LIKE '''%''' OR work_unit LIKE '''%'''
   OR position LIKE '''%''' OR mailing_address LIKE '''%''';
```

确认命中后逐条人工核对再清理，操作前先备份；不要在未核对时批量 UPDATE。手机号带 `'+86`、工作单位/职务以 `-` / `@` 开头的记录最容易命中。

## 数据迁移与版本更新

首次创建 MySQL 数据卷时，会执行 [server/migrations](../server/migrations) 下的初始化 SQL。之后由 `migrate` 服务在 API 启动前执行 [migrate.sh](../server/scripts/migrate.sh)，补齐尚未记录的增量迁移。

已有数据卷无需删除，更新代码后重新执行 `docker compose up --build` 即可应用新迁移；不要为升级而执行 `docker compose down -v`，它会删除项目数据卷。当前迁移脚本固定初始化 `sdu_alumni_db`，数据库名需与迁移保持一致。

生产更新应选择明确的版本标签，先备份数据库与对象存储数据，再更新对应版本并验证服务；不要直接跟随 `main` 每次合并自动更新。标签、发布产物和升级路径验证见[贡献指南](../CONTRIBUTING.md#构建产物与发布)。

## 构建源与网络排障

构建 API 和 Web 镜像需要访问 Alpine apk、Go module 代理及 npm registry。根 `.env` 支持：

```env
GOPROXY=https://goproxy.cn,direct
NPM_REGISTRY=https://registry.npmmirror.com
APK_MIRROR=mirrors.aliyun.com
```

这些示例与根 `.env.example` 一致。Dockerfile 默认使用官方源，CI 不读取本机 `.env`；`APK_MIRROR` 留空时使用 Alpine 官方源，非空时替换 `dl-cdn.alpinelinux.org` 主机名。

以上构建参数不会代理 Docker 镜像拉取。若 Docker Hub 不可达，需要单独检查 Docker 守护进程的 registry mirror；镜像站返回 502 时，应确认上游和备用镜像可用，而不是调整 npm 或 Go 代理。

## 查看状态与排查启动失败

以下命令在仓库根目录执行，健康检查使用默认 API 端口；如已调整 `API_HOST_PORT`，同步修改地址：

```bash
docker compose ps -a
docker compose logs --tail=100 migrate api web
curl --fail http://127.0.0.1:8080/api/v1/health/live
curl --fail http://127.0.0.1:8080/api/v1/health/ready
```

API 等待迁移成功以及 Redis、对象存储健康检查通过。排障时按依赖状态查看日志，例如 `docker compose logs --tail=100 mysql redis minio`。日志可能含运行配置，分享前先去除敏感信息。
