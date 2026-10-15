# 贡献指南

本项目采用“先 Issue，后开发，再由 PR 验收”的工作方式。Issue 说明要达成的业务结果，PR 说明这些结果如何被验证；CI 负责自动检查，评审者负责判断范围、权限和验收证据是否合理。

## 开发流程

1. 使用功能、缺陷或技术任务模板创建 Issue，写清背景、范围和可验证的验收条件。
2. 验收条件清楚后，从最新的 `main` 创建短期分支：`feat/<issue号>-<简短描述>`、`fix/<issue号>-<简短描述>` 或 `chore/<issue号>-<简短描述>`。
3. 每条提交使用 `type(scope): 描述` 格式，例如 `feat(alumni): 新增校友导出` 或 `ci(github): 增加提交消息校验`。`type` 只能是 `feat`、`fix`、`docs`、`style`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`；`scope` 使用小写模块名，如 `auth`、`alumni`、`dashboard`、`web`、`server`、`db`、`github`。
4. 依照验收条件实现并补充测试。修复缺陷必须有能覆盖旧问题的回归测试；权限、个人信息和数据域改动必须覆盖未登录、越权或跨域访问等边界。
5. 提交 PR 到 `main`，用 `Closes #编号` 关联 Issue，并逐项填写验收证据。
6. CI 全部通过、至少一位评审者批准且讨论已解决后，使用 Squash merge 合并到 `main`，完成后删除短期分支。合并不等于发版；需要发布时，在 `main` 上的已验收提交打版本标签。

实现中若目标或范围变化，先更新 Issue 的范围和验收条件，再更新 PR；不要把未讨论的新范围混入已有 PR。

## 验收条件怎么写

验收条件应描述使用者可观察到的结果，而不是实现细节。例如：

- 有导出权限的管理员可以导出当前筛选结果。
- 无导出权限的用户调用接口会得到 403，且不会取得任何数据。
- 无敏感字段权限时，导出内容不含受限字段。

“新增接口”“增加按钮”“测试通过”属于实现或通用检查，不能单独作为验收条件。

## 本地检查

先在 `web/` 执行 `npm ci`，再在仓库根目录使用：

| 命令             | 用途                                   |
| ---------------- | -------------------------------------- |
| `make fmt`       | 格式化 Go 和前端文件（会修改工作区）   |
| `make fmt-check` | 检查格式是否正确                       |
| `make lint`      | 执行 Go vet 与前端 ESLint              |
| `make test`      | 执行后端和前端测试                     |
| `make build`     | 构建 API 和 Web                        |
| `make check`     | 执行格式、静态检查、测试和构建         |
| `make e2e`       | 启动隔离的完整环境并运行浏览器冒烟测试 |

涉及 MySQL、Redis 或 MinIO 的后端集成测试还需要启动隔离的测试服务，并设置相应的 `TEST_*` 环境变量；迁移当前固定初始化 `sdu_alumni_db`，测试 DSN 应指向该隔离数据库。CI 会自动提供这些服务并应用全部迁移。

浏览器端到端测试使用 Playwright 与独立 Compose 项目。首次执行 `make e2e` 会安装 Chromium，启动 MySQL、Redis、MinIO、API 和 Web，等待 API 就绪与 Web 可访问后运行测试，并在结束时删除该项目的容器和卷。默认端口为 13307、16379、18080、18081、19000、19001；可用 `E2E_*_PORT` 环境变量覆盖。失败时可在 `web/test-results/` 查看截图、录像、trace 和 Compose 日志；不要提交这些产物。

## 合并规则

- PR 必须关联一个 Issue；紧急修复可先建最简短 Issue，再补充信息。
- 禁止在 PR、Issue、日志和截图中提交真实个人信息、密码、Token 或生产配置。
- 数据库变更必须新增有序迁移，并验证空库初始化和升级顺序；不得手改 GORM 生成文件。
- CI 必须通过。合并前保持分支可安全 Squash；不要合并红色或被跳过的关键检查。
- `main` 是唯一长期分支，接收功能、修复、文档和依赖升级 PR。不要直接推送到 `main`。
- 指向 `main` 的所有 PR 都会校验提交标题并独立运行 `E2E` 浏览器冒烟检查；失败时下载 CI 的 `e2e-diagnostics-*` 产物排查。
- Dependabot 版本升级 PR 同样提交到 `main`，沿用质量与评审门禁；机器人作者的关联 Issue 豁免见[依赖升级维护说明](docs/依赖升级维护说明.md)。
- 生产环境使用指定版本标签对应的发布产物，不随 `main` 的每次合并自动升级。

## 构建产物与发布

`ci.yml` 是唯一的质量门禁定义，Nightly 和 Release 通过 `workflow_call` 复用它，所以不要在两个发布工作流里重复编写检查步骤。

### Nightly

- 触发：每天 02:00（北京时间）自动执行；也可在 Actions 页面手动 `Run workflow`。
- 内容：在仓库默认分支 `main` 上运行完整质量门禁（含浏览器 E2E），再构建 `sdu-alumni-api-linux-amd64` 与 `sdu-alumni-web-dist.tar.gz`。
- 产物：`nightly-<run_id>` 包含两个构建产物、`BUILD_INFO.txt` 与 `SHA256SUMS`，保留 14 天；API 二进制会先通过 `.github/scripts/check-api-binary.sh` 的存活与就绪检查。
- 升级路径：同时运行 `.github/workflows/migration-upgrade.yml`，验证旧库升级（见下）。
- Nightly 不创建标签，也不创建 Release。

### 升级路径门禁（Nightly 与 Release）

生产环境的数据卷不会重建，只会由 `migrate` 服务执行 `server/scripts/migrate.sh` 把旧库升级到当前迁移集。`.github/scripts/check-migration-upgrade.sh` 在 CI 中复现这条路径：

1. 用**上一个发布标签**的迁移集初始化数据库（默认取 `git describe --tags --abbrev=0 HEAD^`，可用 `BASE_TAG` 覆盖），模拟仍在旧版本的线上库；若基线不包含更旧的迁移集，退化为“除最后一个迁移外”的基线。
2. 在旧 schema 下写入样例校友与账号数据。
3. 执行生产同款 `migrate.sh`（挂载到 `/migrations`）。
4. 断言：迁移记录齐全、每个迁移 `proves` 元数据声明的表存在、既有数据未丢失、数据域回填无空值与孤儿行、中文名称无乱码，并重复执行一次验证幂等。

该门禁目前只在 Nightly 与 Release 运行，不进 PR；需要在 PR 上提前拦住迁移问题时，在 `.github/workflows/ci.yml` 里复用 `.github/scripts/check-migration-upgrade.sh` 即可。

### Release

1. 确认改动已通过 PR 合并到 `main` 并完成验收，然后基于最新远端提交打标签：`git fetch origin main && git tag vX.Y.Z origin/main && git push origin vX.Y.Z`。无需单独的分支合并发布 PR。
2. 标签格式为 `vX.Y.Z`（正式版）或 `vX.Y.Z-<预发布标识>`（预发布）。正式版与预发布标签都必须指向 `main` 上的提交；是否发布为预发布版本由标签格式决定。
3. 工作流先运行与 CI 相同的完整质量门禁（含浏览器 E2E）与升级路径门禁，再构建产物、校验二进制健康检查与前端静态包可服务（含 `SHA256SUMS` 一致性），最后用 `gh release create` 创建 GitHub Release 并附上产物与校验和。
4. `publish` 使用 `release` environment。首次发布前请在仓库 Settings → Environments 为 `release` 配置 required reviewers，这样推送标签不会直接对外发布，需要人工放行；如需限制标签创建权限，再配合 tag protection rules。
5. 发布说明：若存在 `docs/release-notes/<标签>.md`，用它作为 Release 正文，首行 `# 标题` 作为 Release 标题；否则回退到 GitHub 自动生成的 release notes。文件名与写法见 `docs/release-notes/README.md`。

发布失败时可在 Actions 中重跑同一个 run；`gh release create` 不会覆盖已存在的 Release，需要先删除旧 Release 与标签再重新推送。
