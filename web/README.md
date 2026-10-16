# 前端开发

前端采用 React、Vite、TypeScript、Ant Design 和 ECharts。使用 Node.js 22；完整环境见[部署说明](../docs/deployment.md)，质量检查和 PR 规则见[贡献指南](../CONTRIBUTING.md)。

## 本地启动

从仓库根目录执行，首次安装依赖使用 `npm ci`：

```bash
cd web
npm ci
npm run dev
```

开发服务默认地址为 `http://127.0.0.1:5173`；Vite 将 `/api` 代理到 `http://127.0.0.1:8080`，需先启动 API。接口基础路径默认为 `/api/v1`，可用 [.env.example](.env.example) 中的 `VITE_API_BASE_URL` 配置。

在线接口文档（Swagger UI）不是前端页面，也不经前端 dev server 代理：Vite 只代理 `/api` 与对象存储路径，`/swagger/...` 会落在 `src/router/index.tsx` 的通配路由上渲染[未找到页面](src/pages/common/NotFoundPage.tsx)。请直接访问 API 端口 `http://127.0.0.1:8080/swagger/index.html`（Compose 部署同理，Web 端口的 `/swagger/` 也不会转发）。说明见[部署说明](../docs/deployment.md#在线接口文档)。

## 检查与构建

以下命令在 `web/` 执行：

```bash
npm run format:check
npm run lint
npm run test
npm run build
```

构建产物输出到 `dist/`。Docker 镜像使用 Nginx 托管静态资源，将 `/api/` 反向代理到 Compose 中的 `api:8080`。浏览器 E2E 的隔离环境、运行命令和失败产物见[贡献指南](../CONTRIBUTING.md#本地检查)。

## 目录与约定

| 目录                            | 职责                                             |
| ------------------------------- | ------------------------------------------------ |
| `src/api`                       | 通过 `http.ts` 的 `request<T>` 统一发起 API 请求 |
| `src/components`、`src/layouts` | 通用组件与应用布局                               |
| `src/pages`                     | 校友端、管理端、院史共编、数据大屏等页面         |
| `src/router`、`src/store`       | 路由权限守卫与 Zustand 登录态                    |
| `src/types`、`src/utils`        | 业务类型、权限与字典工具                         |
| `src/styles`                    | 全局视觉变量与样式                               |

权限由 `RequireAuth` 和角色层级控制，登录态通过 `authStore` 管理；修改个人信息展示、导出或搜索时，核对角色、数据域和敏感字段边界。完整开发约定见 [AGENTS.md](../AGENTS.md)。
