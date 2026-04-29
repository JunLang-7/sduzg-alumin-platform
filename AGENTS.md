# AGENTS.md

本文件为在本仓库内工作的 AI/自动化编码代理提供项目约定。作用范围为仓库根目录及其所有子目录。

## 项目概览

这是“山大政管学院校友平台”一期 MPA 试点项目，目标是提供校友数据管理、校友查询、个人资料维护、数据大屏和管理员账号管理能力。需求与技术背景见 `docs/山大政管学院校友平台一期需求文档-MPA试点.md` 和 `docs/山大政管学院校友平台一期技术方案文档-MPA试点.md`。

技术栈：

- 后端：Go 1.26、Gin、GORM、MySQL 8.0、Redis、Viper、zap。
- 前端：React 18、Vite、TypeScript、Ant Design、axios、Zustand、ECharts。
- 本地编排：`docker-compose.yml` 启动 MySQL、Redis、API 和 Web。

## 目录结构

- `server/`：后端 API 服务。
  - `cmd/api/main.go`：服务入口、配置加载、日志、数据库、Redis、HTTP Server 生命周期。
  - `internal/router/`：路由注册和依赖装配，统一挂载在 `/api/v1`。
  - `internal/handler/`：HTTP 请求绑定、响应状态和业务错误映射。
  - `internal/service/`：业务逻辑。
  - `internal/repository/`：数据访问接口与实现。
  - `internal/model/`、`internal/query/`：GORM Gen 生成代码。
  - `internal/config/`：环境变量配置和默认值。
  - `internal/response/`：统一响应结构 `{ code, message, data }`。
  - `migrations/`：MySQL 初始化 SQL。
- `web/`：前端应用。
  - `src/api/`：axios 请求封装和业务 API。
  - `src/router/`：路由和权限守卫。
  - `src/store/`：Zustand 状态。
  - `src/pages/`：页面。
  - `src/components/`：通用组件。
  - `src/types/`：业务类型。
  - `src/utils/`：权限、字典等工具。
- `docs/`：需求和技术方案文档。

## 常用命令

后端：

```bash
cd server
go mod tidy
go test ./...
go run ./cmd/api
```

前端：

```bash
cd web
npm install
npm run dev
npm run build
```

整体本地环境：

```bash
docker compose up --build
```

数据库模型生成：

```bash
go install gorm.io/gen/tools/gentool@latest
make gendb
```

健康检查：

```bash
curl http://127.0.0.1:8080/api/v1/health/live
curl http://127.0.0.1:8080/api/v1/health/ready
```

## 后端约定

- 保持现有分层：`handler -> service -> repository -> model/database`。不要把数据库细节写进 handler，也不要把 HTTP 细节写进 service。
- 新接口默认挂在 `router.New` 中的 `/api/v1` 分组下。
- handler 负责 `ShouldBindJSON`/`ShouldBindQuery`、HTTP 状态码、业务错误码和响应消息；service 返回明确的业务错误。
- API 响应必须使用 `internal/response` 的统一结构，成功 `code=0`。
- 配置通过 Viper 从环境变量或 `server/.env` 读取。不要提交真实 `.env` 或生产密钥。
- 数据库和 Redis 默认可关闭，注意保持 `DB_ENABLED=false`、`REDIS_ENABLED=false` 时的骨架服务可运行。
- 密码只允许哈希存储。认证相关改动必须避免向用户暴露账号是否存在等敏感细节。
- 操作用户、校友档案、管理员权限或审计日志时，优先参考 `docs/` 中的角色和权限矩阵。
- `internal/model/*.gen.go` 和 `internal/query/*.gen.go` 是生成代码，表结构变更后通过 `make gendb` 重新生成，不要手工改生成文件。
- Go 代码提交前运行 `gofmt`，涉及后端逻辑时至少运行 `cd server && go test ./...`。

## 前端约定

- 使用 React + TypeScript + Ant Design 的现有风格，优先复用 `src/components`、`src/types`、`src/utils` 中的能力。
- API 请求统一走 `src/api/http.ts` 的 `request<T>`，保持后端 `{ code, message, data }` envelope 兼容。
- 新增接口类型放在 `src/types/`，页面不要内联重复定义复杂后端响应结构。
- 路由权限通过 `RequireAuth` 和角色层级控制，角色包括 `alumni`、`admin`、`super_admin`。
- 登录态由 `src/store/authStore.ts` 管理，避免绕过 store 直接散落读写登录用户信息。
- UI 文案默认使用中文，并保持政管学院/MPA 校友平台语境一致。
- 全局视觉变量在 `src/styles/global.css`，新增样式应尽量复用现有色值和页面结构。
- 前端没有单独 lint/test 脚本；涉及前端改动时至少运行 `cd web && npm run build`。

## 数据库与环境

- MySQL 初始化脚本在 `server/migrations/001_init_schema.sql`，包含 `users`、`alumni_profiles`、`operation_logs`。
- Compose 将 MySQL 映射到本机 `3307`，Redis 映射到 `6379`，API 暴露 `8080`，Web 暴露 `80`。
- 示例配置见 `server/.env.example` 和 `web/.env.example`。
- 前端开发服务器默认 `http://127.0.0.1:5173`，`/api` 代理到 `http://127.0.0.1:8080`。
- 默认超级管理员账号由迁移脚本初始化，仅用于启动和开发场景；生产部署必须更换默认密码和 JWT secret。

## 测试与验证

- 后端业务逻辑优先补充同包 `_test.go`，现有测试使用标准库 `testing` 和轻量 fake store。
- 修改配置加载、路由、中间件、认证或权限时，补充或更新后端测试。
- 修改前端路由、权限、API 类型或构建配置时，运行 `npm run build` 验证 TypeScript 和 Vite 构建。
- 修改数据库 schema 后，同步更新迁移、生成模型、相关 DTO/API 类型和文档。
- 若改动影响联调流程，使用 `docker compose up --build` 验证完整环境。

## 工作注意事项

- 开始改动前检查当前工作区状态，避免覆盖用户已有修改。
- 保持改动聚焦，不做无关格式化、重命名或大范围重构。
- 不提交 `.env`、日志、构建产物、`web/node_modules/`、`web/dist/` 等本地文件。
- 需求或权限不明确时，以 `docs/` 中的一期 MPA 试点范围为准；不要擅自引入活动、内容管理、AI、支付等未纳入一期的模块。
- 任何涉及个人信息展示、导出、搜索和权限的改动，都要优先考虑游客不可见、校友只读他人资料、管理员可维护、超级管理员管理账号的边界。
