# 部署说明

[文档目录](README.md) · [项目首页](../README.md)

## 启动完整环境

在已安装 Docker 与 Docker Compose 的机器上，从仓库根目录执行：

```bash
cp .env.example .env
docker compose up --build
```

已有 `.env` 时直接使用原配置。Compose 启动 MySQL、Redis、Silo 对象存储、数据库迁移任务、API 和 Web；Silo 兼容 MinIO 接口，服务名及配置项保留 `minio`／`MINIO_*` 命名。

| 服务           | 默认地址                | 根 `.env` 中的端口配置         |
| -------------- | ----------------------- | ------------------------------ |
| Web            | `http://127.0.0.1`      | `WEB_HOST_PORT=80`             |
| API            | `http://127.0.0.1:8080` | `API_HOST_PORT=8080`           |
| MySQL          | `127.0.0.1:3307`        | `MYSQL_HOST_PORT=3307`         |
| Redis          | `127.0.0.1:6379`        | `REDIS_HOST_PORT=6379`         |
| 对象存储 API   | `http://127.0.0.1:9000` | `MINIO_API_HOST_PORT=9000`     |
| 对象存储控制台 | `http://127.0.0.1:9001` | `MINIO_CONSOLE_HOST_PORT=9001` |

初始化迁移提供开发账号：`admin` / `Admin@123456`，角色为 `super_admin`。公开部署前必须更换默认管理员密码、数据库与对象存储密码，并向 API 容器传入独立的 `AUTH_JWT_SECRET`。

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

不提交真实 `.env`、生产密钥、日志或构建产物。示例文件与编排中的默认值用于启动和开发，部署时按实际环境设置。

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
