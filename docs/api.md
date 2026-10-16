# 接口文档

[返回文档目录](README.md)

本文档描述平台当前已实现的 HTTP API，覆盖健康检查、认证、本人资料、校友档案、导入导出、档案文件、数据大屏、管理员授权、操作历史和院史共编。接口以[路由注册](../server/internal/router/router.go)、[请求与响应类型](../server/internal/dto/)及服务层校验为准；新增或变更接口随对应 PR 一并更新本文档。

## 目录

- [通用约定](#通用约定)
- [健康检查](#健康检查)
- [认证与账号](#认证与账号)
- [校友本人资料](#校友本人资料)
- [校友档案查询与管理](#校友档案查询与管理)
- [校友导入导出](#校友导入导出)
- [校友档案文件](#校友档案文件)
- [数据大屏](#数据大屏)
- [管理员账号与授权](#管理员账号与授权)
- [操作历史](#操作历史)
- [院史共编](#院史共编)

## 通用约定

- 基础路径为 `/api/v1`，本地开发默认 `http://127.0.0.1:8080/api/v1`。
- 下文路径均相对于基础路径，`:id`、`:fileId`、`:attachmentId` 替换为实际的正整数 ID。
- 无需登录的接口仅有 `GET /health/live`、`GET /health/ready`、`POST /auth/login`、`POST /auth/setup-password` 和 `POST /auth/verify-code/send`。其余接口携带 `Authorization: Bearer <access_token>`。
- 除 Excel 导入使用 `multipart/form-data`、对象存储直传使用文件二进制外，请求体使用 `Content-Type: application/json`；GET 筛选条件使用查询参数。
- JSON 响应使用 `{ code, message, data }`，成功 `code=0`、通常 HTTP 200；新增校友、管理员和院史草稿返回 HTTP 201。`data` 无内容时省略，错误响应通常只有 `code` 和 `message`。导出和模板下载成功返回文件流，不使用 JSON envelope；失败仍返回 JSON。
- 时间字段使用 RFC 3339 格式；可空字段可能返回 `null`，带 `omitempty` 的字段可能省略。文件大小单位为字节，预签名地址的 `expires_in` 单位为秒。

分页接口（校友列表、管理员列表、操作历史）使用 `page`、`page_size`，默认 `1`、`20`；非正数回退默认值，`page_size` 最大 `100`。`data` 结构如下；院史列表、附件列表和分布统计直接返回数组，不套分页结构。

```json
{
  "code": 0,
  "message": "success",
  "data": { "items": [], "page": 1, "page_size": 20, "total": 0 }
}
```

常见错误码：

| HTTP | code  | 含义                                |
| ---- | ----- | ----------------------------------- |
| 400  | 40000 | 请求参数不正确                      |
| 401  | 40100 | 未登录或令牌失效                    |
| 403  | 40300 | 权限不足（含跨数据域）              |
| 404  | 40400 | 资源不存在或不在可访问范围内        |
| 404  | 40401 | 档案文件不存在或尚未完成上传        |
| 409  | 40000 | 当前状态不允许此操作                |
| 413  | 41300 | 档案文件或院史附件超过大小/数量限制 |
| 415  | 41500 | 附件类型不支持                      |
| 429  | 42900 | 请求过于频繁                        |
| 500  | 50000 | 服务器内部错误                      |
| 503  | 50300 | 数据库或对象存储不可用              |

角色与数据域：

- 游客（未登录）只能调用上列公开接口，不可读取校友个人资料或院史内容。
- 校友（`alumni`）的产品入口为本人资料维护与院史共编，不提供校友通讯录或档案文件管理入口。当前后端通用查询接口的实际访问范围另见[查询说明](#查询参数与返回字段)，不能把前端入口限制视为 API 授权。
- 管理员（`admin`）按已分配的有效数据域维护校友档案、查询统计及审计记录；超级管理员（`super_admin`）拥有全部数据域与功能权限，并独占管理员账号管理接口。
- 数据域编码：`undergraduate`（本科生）、`academic_graduate`（学术学位研究生）、`mpa`（MPA 专业学位研究生）。不要硬编码数据库 ID；当前授权域取自 `GET /auth/me` 的 `domains`，超级管理员可通过 `GET /super-admin/data-domains` 获取有效域。
- 功能权限 `alumni.sensitive.read` 控制手机号、邮箱、工作单位、职务、通讯地址的读取及管理员写入；无权限的列表/详情对应字段为 `null`，导出列为空，也不能用敏感字段筛选。本人资料读取不屏蔽本人字段。
- 功能权限 `alumni.files.manage` 独立控制档案文件的列表、上传、下载和删除；敏感字段权限不自动包含文件管理权限，反之亦然。
- JWT 仅用于标识用户；每次受保护请求从数据库加载最新角色、状态、数据域与权限，授权变更无需重新登录。

限流由部署配置控制，触发后返回 HTTP 429 / `42900`，中间件可能携带 `Retry-After`（秒）。发送验证码还存在业务频次限制，其失败当前返回 HTTP 400 / `40000`；Excel 导入超限同样返回 400，不是 413。

## 健康检查

| 方法 | 路径            | 权限 | `data` / 说明                                                                                                       |
| ---- | --------------- | ---- | ------------------------------------------------------------------------------------------------------------------- |
| GET  | `/health/live`  | 公开 | `{ status: "ok", app, env, started_at }`，仅表示进程存活                                                            |
| GET  | `/health/ready` | 公开 | `{ status: "ok", database, redis }`，已启用依赖为 `connected`，未启用为 `disabled`；连接失败返回 HTTP 503 / `50300` |

就绪检查不检查对象存储、短信或邮件服务。数据库和 Redis 均关闭时，骨架服务仍可返回就绪成功，不代表业务接口可用。

## 认证与账号

| 方法 | 路径                     | 权限             | 请求 / `data`                                                     |
| ---- | ------------------------ | ---------------- | ----------------------------------------------------------------- |
| POST | `/auth/verify-code/send` | 公开             | 发送验证码，见下文                                                |
| POST | `/auth/login`            | 公开             | 密码或验证码登录，见下文                                          |
| POST | `/auth/setup-password`   | 公开，需注册令牌 | 首次设置密码并创建校友账号，返回完整登录结果                      |
| GET  | `/auth/me`               | 已登录           | 当前用户及实时有效的 `domains`、`permissions`                     |
| POST | `/auth/change-password`  | 已登录           | `{ old_password, new_password, confirm_password }`，成功无 `data` |
| POST | `/auth/logout`           | 已登录           | 无请求体，返回 `{ logged_out: true }`                             |

### 验证码与登录方式

发送验证码：

```json
{ "target": "alumni@example.com", "purpose": "login" }
```

- `target` 必填，为中国大陆手机号（`^1[3-9]\d{9}$`）或邮箱；`purpose` 当前只接受 `login`。
- 返回 `{ expire_at, resend_after: 60 }`；验证码有效期 5 分钟，同一目标至少间隔 60 秒，每日最多发送 10 次。
- 必须按部署说明配置 Redis 和短信/邮件通道。通道未配置时存在开发用固定验证码回退，不能将该模式作为生产发送能力；见[部署说明](deployment.md)。

`POST /auth/login` 请求字段：

| 字段         | 类型   | 说明                                                                          |
| ------------ | ------ | ----------------------------------------------------------------------------- |
| `grant_type` | string | 省略或 `password`：密码登录；`sms_code`：短信验证码；`email_code`：邮箱验证码 |
| `account`    | string | 管理员账号；密码登录也会按格式识别手机号/邮箱                                 |
| `mobile`     | string | 手机号；短信登录必填；传入时优先于 `email`、`account`                         |
| `email`      | string | 邮箱；邮箱验证码登录必填；密码登录时优先于 `account`                          |
| `password`   | string | 密码登录必填                                                                  |
| `code`       | string | 验证码登录必填                                                                |

密码登录示例（不要在文档或代码中写入真实密码）：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"account":"<管理员账号>","password":"<密码>","grant_type":"password"}'
```

邮箱验证码登录示例：

```json
{ "email": "alumni@example.com", "code": "<收到的验证码>", "grant_type": "email_code" }
```

正常登录的 `data` 包含 `access_token`、`token_type`（`Bearer`）、`expires_at`、`user`。`user` 字段为 `id`、`account`、`role`、可选的 `real_name` / `alumni_id` / `mobile`，以及 `domains`、`permissions`。**登录响应不加载管理员授权，客户端应再请求 `GET /auth/me` 获取当前能力**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "account": "example_admin",
    "role": "admin",
    "domains": [{ "id": 1, "code": "undergraduate", "name": "本科生" }],
    "permissions": ["alumni.sensitive.read"]
  }
}
```

验证码登录必须能匹配已有校友档案的手机号/邮箱。若尚无账号，返回非空 `registration_token`，此时 `access_token` 为空，不能据此访问受保护接口。应在 5 分钟内向 `POST /auth/setup-password` 提交：

```json
{
  "registration_token": "<首次验证码登录返回的令牌>",
  "new_password": "<新密码>",
  "confirm_password": "<相同的新密码>"
}
```

首次设密与修改密码均要求新密码至少 8 位、同时含字母和数字，两次输入一致。创建管理员的密码校验单独见[管理员账号与授权](#管理员账号与授权)。

常见失败：登录凭据错误返回 401；账号禁用返回 403；短时累计 5 次登录失败会临时锁定（5 分钟窗口），返回 429；验证码错误/过期返回 400。退出当前仅返回成功标记，**不撤销已签发 JWT**，客户端必须清除本地令牌；当前无刷新令牌接口。

## 校友本人资料

以下接口仅限 `alumni`；`GET/PUT /alumni/me` 还要求账号绑定有效校友档案。管理员调用不是“查看自己管理的校友”，会被拒绝。

| 方法 | 路径                 | 请求                                                                   | `data`                          |
| ---- | -------------------- | ---------------------------------------------------------------------- | ------------------------------- |
| GET  | `/alumni/me`         | 无                                                                     | 本人 `AlumniDetail`，字段见下节 |
| PUT  | `/alumni/me`         | 可选 string 字段：`work_unit`、`position`、`mailing_address`、`mobile` | 更新后的本人详情                |
| PUT  | `/alumni/me/contact` | `mobile`、`email` 至少传一个，另传 `code`                              | 成功无 `data`                   |

- 本人资料更新只处理上表允许的字段；不支持修改姓名、年级、数据域等学籍/归属信息。未提交或传 `null` 的字段不修改，传空字符串可清空。
- 联系方式接口会校验手机号/邮箱格式和账号占用。验证码发送目标为账号**原有**对应联系方式；原值未设置时验证新值。仍通过 `/auth/verify-code/send`、`purpose=login` 获取验证码。建议手机号和邮箱分次提交，避免同一个 `code` 在两项校验中被重复消费。
- `/alumni/me/contact` 同时更新账号联系方式并尝试同步绑定档案；`PUT /alumni/me` 中的 `mobile` 只更新档案，不等于变更登录手机号，也不执行验证码校验。需要调整登录联系方式时使用专用接口。
- 未登录返回 401；非校友或未绑定资料返回 403；档案不存在返回 404；联系方式被占用、验证码错误或字段不合法返回 400。

## 校友档案查询与管理

| 方法   | 路径                | 权限                      | `data` / 说明                                                |
| ------ | ------------------- | ------------------------- | ------------------------------------------------------------ |
| GET    | `/alumni`           | 已登录，实际范围见下文    | 校友分页列表                                                 |
| GET    | `/alumni/:id`       | 已登录，实际范围见下文    | `AlumniDetail`                                               |
| POST   | `/admin/alumni`     | 管理员/超级管理员         | 新增档案，HTTP 201，返回详情                                 |
| PUT    | `/admin/alumni/:id` | 数据域内管理员/超级管理员 | 修改档案，返回详情                                           |
| DELETE | `/admin/alumni/:id` | 数据域内管理员/超级管理员 | 软删除档案，返回 `{ deleted: true }`，并尽力级联清理档案文件 |

### 查询参数与返回字段

| 参数                                 | 类型   | 匹配方式 / 约束                                                            |
| ------------------------------------ | ------ | -------------------------------------------------------------------------- |
| `page`、`page_size`                  | int    | 通用分页                                                                   |
| `keyword`                            | string | 姓名、导师、辅导员模糊匹配；有敏感字段权限时额外匹配工作单位、职务、手机号 |
| `grade`、`class_name`、`cohort`      | string | 年级、班级、届数，精确匹配                                                 |
| `counselor`、`mentor`                | string | 辅导员、导师，精确匹配                                                     |
| `major`、`training_mode`、`industry` | string | 专业、培养方式、行业，精确匹配                                             |
| `work_unit`、`position`              | string | 工作单位、职务模糊匹配，需 `alumni.sensitive.read`                         |
| `mobile`                             | string | 手机号精确匹配，需 `alumni.sensitive.read`                                 |
| `data_domain_id`                     | uint64 | 指定数据域；普通管理员只能选择已授权域，越域返回 403                       |

普通管理员不传 `data_domain_id` 时查询其全部授权域，无授权域不能查询；单条详情越域或已删除均返回 404。超级管理员默认全域。列表按 ID 降序排列。

> 实现边界：当前 `GET /alumni` 和 `GET /alumni/:id` 未设置管理员角色门禁，校友令牌也可调用并读取非敏感字段（本人详情还可读取本人敏感字段）。这与“校友端仅本人资料与院史共编”的产品边界存在差异；本文如实记录现状，不代表新增通讯录能力，也不宣称后端已限制校友只能查本人。

列表项字段：`id`、`data_domain_id`、`name`、`grade`、`class_name`、`cohort`、`counselor`、`mentor`、`major`、`training_mode`、`industry`、`work_unit`、`position`、`gender`、`mobile`、`email`、`updated_at`。`AlumniDetail` 在此基础上增加 `mailing_address`、`status`、`created_at`。可选字符串可为 `null`；`remark` 虽可写入，但当前详情 DTO 不返回。

### 新增与修改字段

| 字段                                                          | 类型          | 说明                                                                                                              |
| ------------------------------------------------------------- | ------------- | ----------------------------------------------------------------------------------------------------------------- |
| `name`、`grade`                                               | string        | 新增和修改均必填，去除首尾空格后不得为空                                                                          |
| `data_domain_id`                                              | uint64        | 仅新增支持。单域管理员自动使用唯一授权域；多域管理员必须指定授权域；超级管理员省略时兼容默认 MPA 域，建议显式指定 |
| `class_name`、`cohort`、`counselor`、`mentor`                 | string / null | 班级、届数、辅导员、导师                                                                                          |
| `major`、`training_mode`、`industry`、`gender`                | string / null | 专业、培养方式、行业、性别                                                                                        |
| `work_unit`、`position`、`mailing_address`、`mobile`、`email` | string / null | 敏感字段，管理员写入需 `alumni.sensitive.read`；修改时传空字符串清空也需要该权限                                  |
| `remark`                                                      | string / null | 备注                                                                                                              |

修改时 `name`、`grade` 必须每次携带，其余未提交或为 `null` 的字段保持原值，空字符串表示清空；此接口不能迁移数据域。新增数据域无效返回 400，不可用返回 503。

```json
{
  "data_domain_id": 1,
  "name": "示例校友",
  "grade": "2020",
  "class_name": "一班",
  "major": "公共管理"
}
```

## 校友导入导出

| 方法 | 路径                     | 权限                            | 请求 / 响应                                                             |
| ---- | ------------------------ | ------------------------------- | ----------------------------------------------------------------------- |
| GET  | `/admin/alumni/template` | 管理员/超级管理员               | 下载 `alumni_import_template.xlsx`，无请求参数                          |
| POST | `/admin/alumni/import`   | 管理员/超级管理员，按数据域限制 | `multipart/form-data`：`file` 必填、`data_domain_id` 可选，返回导入结果 |
| GET  | `/admin/alumni/export`   | 管理员/超级管理员，按数据域限制 | 支持校友列表的筛选参数（不分页）及 `format`，返回匹配的全部记录文件     |

### Excel 导入

- 仅支持 `.xlsx` 工作簿，读取第一个工作表。上传文件不超过 20 MiB，单次最多 5000 条数据，姓名、年级必填；大小或行数超限返回 HTTP 400 / `40000`。
- 使用接口下载的模板，不要改变列名/顺序。通用 15 列依次为：姓名、年级、班级、届数、辅导员、导师、专业、培养方式、行业、工作单位、职务、通讯地址、性别、手机号、邮箱；下载的 16 列模板在末尾增加“所属领域”。
- “所属领域”填写数据域**编码**（`undergraduate` / `academic_graduate` / `mpa`），不是显示名称或 ID。单域管理员自动使用唯一授权域；其他情况下可通过表单 `data_domain_id` 统一指定目标域（优先于逐行领域），否则必须使用含“所属领域”的模板并逐行填写。普通管理员不能导入未授权域。
- 无敏感字段权限的管理员不能导入包含敏感值的行。以“姓名 + 年级 + 班级 + 届数 + 手机号”去重，库内重复和同文件重复均计为行错误，不执行覆盖更新。
- 逐行校验，允许部分成功；HTTP 200 / `code=0` **不代表全部行成功**，必须检查 `success` 和 `errors`。`row` 为 Excel 实际行号（表头是第 1 行）。

```bash
curl -X POST http://127.0.0.1:8080/api/v1/admin/alumni/import \
  -H 'Authorization: Bearer <access_token>' \
  -F 'file=@alumni_import_template.xlsx' \
  -F 'data_domain_id=1'
```

示例结果：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 2,
    "success": 1,
    "errors": [{ "row": 3, "name": "示例校友", "message": "年级为空" }]
  }
}
```

### 导出文件

`format=csv` 返回 CSV，其余取值或省略返回 XLSX。成功响应携带 `Content-Disposition: attachment; filename=...`，类型为 `text/csv; charset=utf-8` 或 `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`。客户端按文件流/blob 处理，不要读取 JSON 的 `data`。导出沿用查询的数据域与敏感字段限制，无权限的敏感列留空；输出会对可能触发电子表格公式的值作转义。

## 校友档案文件

以下 5 个路由**仅在对象存储启用且客户端初始化成功时注册**，未注册时返回 404。所有操作（含列表和下载）均要求管理员角色、目标校友处于授权域内、且拥有 `alumni.files.manage`；校友本人也不能调用。

| 方法   | 路径                                       | 请求 / `data`                                                                                                                               |
| ------ | ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| GET    | `/admin/alumni/:id/files`                  | `{ alumni_id, degree_archive: [], academic_record: [] }`；每项含 `id`、`file_type`、`original_name`、`file_size`、`mime_type`、`created_at` |
| POST   | `/admin/alumni/:id/files/upload-url`       | JSON 必填 `file_type`、`original_name`、`mime_type`；返回 `{ file_id, upload_url, expires_in: 900 }`                                        |
| POST   | `/admin/alumni/:id/files/:fileId/confirm`  | 无请求体；返回 `{ id, alumni_id, file_type, original_name, file_size, mime_type }`                                                          |
| GET    | `/admin/alumni/:id/files/:fileId/download` | 返回 `{ download_url, expires_in: 900, original_name }`，不是文件流或重定向                                                                 |
| DELETE | `/admin/alumni/:id/files/:fileId`          | 返回 `{ deleted: true }`；软删除记录并尽力清理存储对象                                                                                      |

`file_type` 仅接受 `degree_archive`（学位档案）、`academic_record`（学籍档案）。允许 MIME：

| 格式      | `mime_type`                                                               |
| --------- | ------------------------------------------------------------------------- |
| PDF       | `application/pdf`                                                         |
| Word DOC  | `application/msword`                                                      |
| Word DOCX | `application/vnd.openxmlformats-officedocument.wordprocessingml.document` |
| JPG / PNG | `image/jpeg` / `image/png`                                                |

文件上传流程：

1. 申请上传地址，生成 `pending` 文件记录。
2. 对完整 `upload_url` 直接发送 HTTP PUT，body 为文件二进制（非 multipart），不要拼接 `/api/v1` 或携带平台 Bearer 令牌。
3. PUT 成功后调用 `confirm`。服务端检查对象存在、非空、实际大小不超过 50 MiB，再激活文件；未确认文件不进入正常列表。
4. 同一校友同一档案类型的新文件确认后替换旧文件，不是追加多份；下载前另取临时下载地址。

上传地址过期需重新申请。地址含临时访问凭据，不应写入日志、公开分享或作为永久链接保存。不支持的类型返回 415 / `41500`，确认时超限返回 413 / `41300`，未上传/记录不存在返回 404 / `40401`。

## 数据大屏

仅管理员/超级管理员可访问。普通管理员统计其全部有效授权域，超级管理员统计全域；无授权域返回 403，不支持通过参数扩大范围。

| 方法 | 路径                            | 参数 / `data`                              |
| ---- | ------------------------------- | ------------------------------------------ |
| GET  | `/admin/dashboard/overview`     | 无参数，返回总览对象                       |
| GET  | `/admin/dashboard/distribution` | 必填 `dimension`，返回 `[{ name, value }]` |

总览字段：

| 字段                      | 类型   | 说明                                                                       |
| ------------------------- | ------ | -------------------------------------------------------------------------- |
| `total_alumni`            | int64  | 范围内未删除且有效的校友档案数                                             |
| `total_accounts`          | int64  | 范围内有效校友档案中，已绑定有效校友账号的档案数（按档案去重，不含管理员） |
| `mobile_complete_rate`    | number | 手机号填写率                                                               |
| `work_unit_complete_rate` | number | 工作单位填写率                                                             |
| `mentor_complete_rate`    | number | 导师填写率                                                                 |

填写率为 `0`～`1` 的比例，不是百分数；总人数为零时为 `0`。分布维度仅接受 `grade`、`class_name`、`cohort`、`gender`、`major`、`training_mode`、`industry`，无效值返回 400；空值归为“未填”，按人数降序、名称升序排列。例如：`GET /admin/dashboard/distribution?dimension=grade`。

## 管理员账号与授权

本节接口均**仅限超级管理员**，普通管理员即使拥有全部功能权限也不能调用。

| 方法   | 路径                             | 请求 / `data`                                                 |
| ------ | -------------------------------- | ------------------------------------------------------------- |
| GET    | `/super-admin/admins`            | `page`、`page_size`；返回管理员分页列表                       |
| POST   | `/super-admin/admins`            | 创建普通管理员，HTTP 201，返回管理员详情                      |
| GET    | `/super-admin/admins/:id`        | 返回管理员详情                                                |
| PUT    | `/super-admin/admins/:id/access` | `{ domain_ids, permissions }`，整体替换授权，返回更新后的详情 |
| DELETE | `/super-admin/admins/:id`        | 删除普通管理员，返回 `{ deleted: true }`                      |
| GET    | `/super-admin/data-domains`      | 返回有效数据域数组 `[{ id, code, name }]`                     |

创建请求：

| 字段          | 类型          | 说明                                                                     |
| ------------- | ------------- | ------------------------------------------------------------------------ |
| `account`     | string，必填  | 4～32 位，以英文字母开头，仅允许字母、数字、下划线和连字符               |
| `password`    | string，必填  | 至少 8 位；当前创建接口只校验长度，不执行修改密码接口的字母/数字组合校验 |
| `real_name`   | string / null | 可选姓名                                                                 |
| `mobile`      | string / null | 可选中国大陆手机号                                                       |
| `domain_ids`  | uint64[]      | 至少一个有效数据域 ID，不可重复                                          |
| `permissions` | string[]      | 仅允许 `alumni.sensitive.read`、`alumni.files.manage`，不可重复，可为空  |

```json
{
  "account": "example_admin",
  "password": "<至少8位密码>",
  "real_name": "示例管理员",
  "domain_ids": [1],
  "permissions": ["alumni.sensitive.read", "alumni.files.manage"]
}
```

列表项返回 `id`、`account`、`role`、`real_name`、`mobile`、`status`、`last_login_at`、`created_at`、`domains`、`permissions`；详情不含 `last_login_at`，另含 `updated_at`，不返回密码或哈希。

授权更新不是合并，必须提交完整 `domain_ids` 和 `permissions`；空权限数组会清除功能权限，数据域不可清空，不能修改超级管理员授权。删除不能针对本人或超级管理员。非法账号/手机号、重复账号、无效授权等返回 400；目标不存在返回 404。当前没有管理员基本信息修改、密码重置或启停的独立接口。

## 操作历史

只读接口，限管理员/超级管理员；当前仅查询校友档案（`alumni_profile`）与批量导入（`alumni_batch`）记录，不是全部系统日志或档案文件操作日志。普通管理员仅能查看授权数据域相关记录（批量导入按涉及域是否有交集判定），越域详情返回 404。没有修改、删除历史或恢复旧版本接口。

| 方法 | 路径                          | `data`                                     |
| ---- | ----------------------------- | ------------------------------------------ |
| GET  | `/admin/audit/operations`     | 分页操作记录，不含字段差异明细             |
| GET  | `/admin/audit/operations/:id` | 单条记录及字段差异、批量导入明细（如适用） |

列表查询参数：

| 参数                     | 类型   | 说明                                                              |
| ------------------------ | ------ | ----------------------------------------------------------------- |
| `page`、`page_size`      | int    | 通用分页                                                          |
| `start_date`、`end_date` | string | `YYYY-MM-DD`，包含起止日期（按 UTC 日边界处理），格式错误返回 400 |
| `management_scope`       | string | 管理范围名称精确匹配，例如“本科生”；不会扩大当前用户授权范围      |
| `action`                 | string | 操作类型，例如 `create`、`update`、`delete`、`import`             |
| `target_id`              | uint64 | 筛选指定校友的操作记录                                            |

记录包含 `id`、`created_at`、`operator`、`operator_role`、`operator_role_label`、可选 `target_id`、`target_type`、`target_name`、可选 `target_meta`、`management_scope`、`action`、`source`、可选 `reason`、`status`。

详情中的 `changes` 每项为 `{ field_name, field_label, old_value, new_value, current_value?, sensitive? }`。敏感字段的历史差异只返回“未填写/首次填写/已修改”等安全状态，**即使拥有敏感权限也不返回历史原值**。批量导入详情另有 `batch_created_alumni`、`batch_import_fields`、`batch_hidden_sensitive_count`；有 `alumni.sensitive.read` 时，允许从当前可访问档案补充敏感字段的**当前值**，不是历史快照。

```bash
curl 'http://127.0.0.1:8080/api/v1/admin/audit/operations?page=1&page_size=20&start_date=2026-01-01&end_date=2026-01-31' \
  -H 'Authorization: Bearer <access_token>'
```

## 院史共编

### 词条阅读与维护

| 方法 | 路径                   | 权限                       | 说明                                                                    |
| ---- | ---------------------- | -------------------------- | ----------------------------------------------------------------------- |
| GET  | `/history/entries`     | 登录用户                   | 查询已发布词条，支持 `keyword`（标题/简介/正文模糊匹配）                |
| GET  | `/history/entries/:id` | 登录用户                   | 已发布词条详情                                                          |
| PUT  | `/history/entries/:id` | 数据域内管理员或超级管理员 | 直接编辑已发布词条（`title`、`content`、`source_note`），不进入审核流程 |

### 词条删除

| 方法   | 路径                                     | 权限                                     | 说明                                                                                             |
| ------ | ---------------------------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------ |
| DELETE | `/history/entries/:id`                   | 仅超级管理员                             | 直接删除（软删除），同时将该词条待审删除申请标记为已同意                                         |
| POST   | `/history/entries/:id/deletion-requests` | 数据域内管理员或超级管理员               | 提交删除申请，`reason` 必填、最多 2000 字；同一词条存在待审申请时不可重复提交                    |
| GET    | `/history/deletion-requests/me`          | 管理员或超级管理员                       | 本人全部删除申请及审核结果                                                                       |
| GET    | `/history/deletion-requests`             | 管理员或超级管理员                       | 待审队列：普通管理员仅见同数据域其他管理员的申请；超级管理员可见全部                             |
| POST   | `/history/deletion-requests/:id/review`  | 同数据域其他管理员或超级管理员（可自审） | `action` 为 `approve` 或 `reject`；驳回必须填写 `review_comment`（最多 2000 字）；同意执行软删除 |

删除均为软删除：词条、版本、历史投稿和附件记录保留，原投稿人仍可查看本人投稿正文与附件；已删除词条不会再出现在列表、搜索和详情中，也不能被后续投稿审核复活。

### 投稿

| 方法   | 路径                                | 权限                                      | 说明                                                                  |
| ------ | ----------------------------------- | ----------------------------------------- | --------------------------------------------------------------------- |
| GET    | `/history/contributions/me`         | 校友、管理员或超级管理员                  | 本人全部投稿                                                          |
| GET    | `/history/contributions/:id`        | 投稿作者本人，或数据域内管理员/超级管理员 | 投稿详情                                                              |
| POST   | `/history/contributions`            | 校友、管理员或超级管理员                  | 新建词条或补充修改；管理员投稿必须指定本人数据域内的 `data_domain_id` |
| PUT    | `/history/contributions/:id`        | 投稿作者本人                              | 编辑本人草稿或退回状态的投稿                                          |
| DELETE | `/history/contributions/:id`        | 投稿作者本人                              | 删除本人草稿                                                          |
| POST   | `/history/contributions/:id/submit` | 投稿作者本人                              | 提交审核，审核以最新内容为准                                          |

### 投稿附件

| 方法   | 路径                                                            | 权限                         | 说明                                                               |
| ------ | --------------------------------------------------------------- | ---------------------------- | ------------------------------------------------------------------ |
| POST   | `/history/contributions/:id/attachments/upload-url`             | 投稿作者本人                 | 申请附件直传地址，类型限 JPG/PNG/WebP/PDF，单个 ≤10 MB，每篇 ≤6 个 |
| POST   | `/history/contributions/:id/attachments/:attachmentId/confirm`  | 投稿作者本人                 | 确认上传完成                                                       |
| GET    | `/history/contributions/:id/attachments`                        | 投稿作者本人或数据域内管理员 | 附件列表                                                           |
| PUT    | `/history/contributions/:id/attachments/:attachmentId`          | 投稿作者本人                 | 更新附件说明                                                       |
| DELETE | `/history/contributions/:id/attachments/:attachmentId`          | 投稿作者本人                 | 移除附件                                                           |
| GET    | `/history/contributions/:id/attachments/:attachmentId/download` | 投稿作者本人或数据域内管理员 | 获取附件下载地址（已通过附件对全部登录用户开放）                   |

### 投稿审核

| 方法 | 路径                               | 权限                       | 说明                                                                                          |
| ---- | ---------------------------------- | -------------------------- | --------------------------------------------------------------------------------------------- |
| GET  | `/history/reviews`                 | 管理员或超级管理员         | 待审投稿队列，按管理员数据域过滤                                                              |
| GET  | `/history/reviews/:id/attachments` | 数据域内管理员或超级管理员 | 待审投稿的附件列表（可下载）                                                                  |
| POST | `/history/reviews/:id`             | 数据域内管理员或超级管理员 | 处理方式 `approve` / `return` / `reject`；退回与驳回必须填写 `review_comment`（最多 5000 字） |

删除申请的审核流程与操作入口见[平台使用说明](平台使用说明.md)第 5 章。
