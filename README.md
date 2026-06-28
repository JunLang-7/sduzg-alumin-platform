# 山大政管学院校友平台

“山大政管学院校友平台”是面向山东大学政治学与公共管理学院 MPA 校友的一体化信息管理平台。项目覆盖校友数据管理、校友查询、个人资料维护、文件档案管理、数据大屏、管理员账号管理，以及短信/邮件验证码登录等能力。

## 功能概览

- **校友档案管理**：管理员可维护校友基础信息、教育信息、联系方式、工作信息等资料。
- **校友查询与个人资料**：校友可登录查看校友目录，并维护自己的个人资料。
- **批量导入导出**：支持 Excel 批量导入、导出校友数据和导出模板，包含去重和错误反馈。
- **档案文件管理**：基于 MinIO 管理学位档案、学籍档案等文件，支持上传、下载和删除。
- **数据大屏**：提供校友总览、行业分布、地域分布、毕业率等可视化分析，并支持交互地图。
- **认证与安全**：支持账号密码、短信验证码、邮箱验证码登录，提供 JWT 鉴权、接口限流、CORS 配置和敏感字段保护。
- **权限体系**：角色包含 `alumni`、`admin`、`super_admin`，区分校友端、管理员端和超级管理员能力边界。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 后端 | Go 1.26、Gin、GORM、GORM Gen、Viper、zap |
| 数据库/缓存 | MySQL 8.0、Redis |
| 文件存储 | MinIO |
| 前端 | React 18、Vite、TypeScript、Ant Design、axios、Zustand、ECharts |
| 本地编排 | Docker Compose |

## 目录结构

```text
.
├── docs/                  需求文档和技术方案
├── server/                Go 后端 API 服务
│   ├── cmd/api/           服务入口
│   ├── internal/          handler/service/repository/model 等内部模块
│   ├── migrations/        MySQL 初始化和增量迁移 SQL
│   └── scripts/           辅助测试脚本
├── web/                   React 前端应用
│   ├── src/api/           API 请求封装
│   ├── src/components/    通用组件
│   ├── src/pages/         页面
│   ├── src/router/        路由与权限守卫
│   ├── src/store/         Zustand 状态
│   └── src/types/         业务类型
├── docker-compose.yml     本地 MySQL、Redis、MinIO、API、Web 编排
└── Makefile               数据库模型生成等辅助命令
```

## 快速启动

### Docker Compose

推荐使用 Docker Compose 一次性启动完整本地环境：

```bash
cp .env.example .env
docker compose up --build
```

默认服务地址：

| 服务 | 地址 |
| --- | --- |
| 前端 | `http://127.0.0.1` |
| API | `http://127.0.0.1:8080` |
| MySQL | `127.0.0.1:3307` |
| Redis | `127.0.0.1:6379` |
| MinIO API | `http://127.0.0.1:9000` |
| MinIO Console | `http://127.0.0.1:9001` |

首次启动时，MySQL 会执行 `server/migrations/` 下的初始化脚本。开发环境默认超级管理员账号：

```text
account: admin
password: Admin@123456
role: super_admin
```

生产环境必须修改默认密码、数据库密码、MinIO 密码和 JWT Secret。

### 后端本地启动

```bash
cd server
cp .env.example .env
go mod tidy
go run ./cmd/api
```

默认 `DB_ENABLED=false`、`REDIS_ENABLED=false`，可以先启动无数据库依赖的服务骨架。需要连接 Docker Compose 中的 MySQL 和 Redis 时，在 `server/.env` 中启用：

```env
DB_ENABLED=true
DB_HOST=127.0.0.1
DB_PORT=3307
DB_USER=sdu_alumni
DB_PASSWORD=sdu_alumni_password
DB_NAME=sdu_alumni_db

REDIS_ENABLED=true
REDIS_ADDR=127.0.0.1:6379
```

健康检查：

```bash
curl http://127.0.0.1:8080/api/v1/health/live
curl http://127.0.0.1:8080/api/v1/health/ready
```

### 前端本地启动

```bash
cd web
npm install
npm run dev
```

前端开发服务默认运行在 `http://127.0.0.1:5173`，开发环境下 `/api` 会代理到 `http://127.0.0.1:8080`，接口基础路径为 `/api/v1`。

## 常用命令

```bash
# 后端测试
cd server
go test ./...

# 前端构建
cd web
npm run build

# 生成 GORM 模型
go install gorm.io/gen/tools/gentool@latest
make gendb

# 限流边界压测
./server/scripts/rate_limit_boundary_test.sh
```

## 环境配置

根目录 `.env.example` 用于 Docker Compose，`server/.env.example` 用于后端本地运行，`web/.env.example` 用于前端构建配置。

关键配置包括：

- MySQL：`MYSQL_*`、`DB_*`
- Redis：`REDIS_*`
- MinIO：`MINIO_*`、`STORAGE_*`
- JWT：`AUTH_JWT_SECRET`、`AUTH_ACCESS_TOKEN_TTL`
- 腾讯云短信：`SMS_TENCENT_*`
- SMTP 邮件：`EMAIL_*`
- 限流：`RATE_LIMIT_*`
- CORS：`CORS_*`

不要提交真实 `.env`、生产密钥或本地构建产物。

## 文档

- [一期需求文档](docs/山大政管学院校友平台一期需求文档-MPA试点.md)
- [一期技术方案文档](docs/山大政管学院校友平台一期技术方案文档-MPA试点.md)
- [二期需求文档](docs/山大政管学院校友平台二期需求文档-MPA试点.md)
- [三期需求文档](docs/山大政管学院校友平台三期需求文档-MPA试点.md)
- [后端说明](server/README.md)
- [前端说明](web/README.md)

## 开发约定

- 后端保持 `handler -> service -> repository -> model/database` 分层。
- API 响应统一使用 `{ code, message, data }`，成功时 `code=0`。
- 前端 API 请求统一走 `src/api/http.ts` 的 `request<T>`。
- 路由权限通过 `RequireAuth` 和角色层级控制。
- 涉及个人信息展示、导出、搜索和权限时，优先保证游客不可见、校友只读他人非敏感资料、管理员可维护、超级管理员管理账号。
- 修改后端逻辑时运行 `go test ./...`；修改前端逻辑时运行 `npm run build`。
