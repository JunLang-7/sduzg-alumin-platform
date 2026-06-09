# 山大政管学院校友平台三期需求文档（MPA 试点）

版本：V0.3
日期：2026-06-09
阶段范围：第三阶段，手机号/邮箱验证码登录体系
前置依赖：二期已完成账号体系、角色权限、校友 CRUD、数据大屏、管理员管理、数据导入导出、文件管理、敏感字段保护

## 1. 文档目的

本文档覆盖一期、二期发布后的新增需求。三期不改变一期、二期已有的角色模型和权限边界，而是在此基础上重构登录认证体系，使校友支持通过手机号和邮箱 + 验证码登录，管理员保持原有账号密码登录方式。校友首次验证码登录时需先设置登录密码，密码设置后系统才创建账号并完成登录。

三期核心目标：

1. 校友支持通过手机号 + 短信验证码登录，首次登录需先设置密码，密码设置后系统才创建账号。
2. 校友支持通过邮箱 + 邮件验证码登录，首次登录需先设置密码，密码设置后系统才创建账号。
3. 校友支持通过手机号或邮箱 + 密码登录。
4. 校友登录后支持在"我的资料"页面绑定/修改手机号和邮箱。
5. 管理员和超级管理员保持原有账号 + 密码登录方式不变。
6. 数据库表结构调整，支持手机号和邮箱的唯一性约束。
7. 支持短信/邮件验证码发送（需短信服务通道 + SMTP 服务，未接入时功能降级跳过发送但接口可用）。
8. 密码复杂度要求：必须同时包含字母和数字，长度至少 8 位，前后端均需校验。

## 2. 三期新增/变更范围

### 2.1 纳入范围

| 模块 | 说明 | 前置状态 |
| --- | --- | --- |
| 校友登录（短信验证码） | 手机号 + 短信验证码快捷登录，首次登录需设置密码后创建账号 | 一期"暂不纳入" |
| 校友登录（邮件验证码） | 邮箱 + 邮件验证码快捷登录，首次登录需设置密码后创建账号 | 一期"暂不纳入" |
| 校友登录（密码） | 手机号/邮箱 + 密码登录 | 一期"暂不纳入" |
| 验证码发送 | 发送短信/邮件验证码 | 一期"暂不纳入" |
| 管理员登录 | 账号 + 密码登录（保持现有逻辑） | 一期已实现 |
| 统一认证接口 | POST /auth/login 支持密码/验证码两种模式 | 一期已实现（仅账号密码） |
| 数据库变更 | users 表增加 email 列；alumni_profiles 表增加 email 列 | 一、二期未实现 |
| 登录尝试限制 | 手机号/邮箱登录失败锁定机制 | 一期已实现（基于 account） |
| 校友绑定手机号/邮箱 | 校友登录后在"我的资料"页面绑定/修改手机号和邮箱 | 一期已实现（仅校友端 mobile 修改） |

### 2.2 未纳入范围

以下能力不作为第三阶段验收目标：

2. 图形验证码（滑块、点选等）防刷。
3. 统一身份认证（CAS、OAuth2、微信登录等）。
4. 校友注册审批流（如需要学院审核后才开通账号）。
5. 密码找回（通过邮箱/手机重置密码）。

## 3. 用户角色与账号模型

### 3.1 角色不变

三期不新增角色，不修改现有角色权限定义。角色定义与一期一致：

| 角色 | 定义 | 核心权限 |
| --- | --- | --- |
| 校友 | 已登录的 MPA 校友 | 查看所有校友信息，维护自己的个人信息 |
| 管理员 | 平台运营管理人员 | 查看数据大屏，管理校友信息，查看所有校友资料 |
| 超级管理员 | 最高权限管理员 | 具备管理员全部权限，额外可创建和删除管理员 |

### 3.2 账号模型

| 角色 | 登录标识 | 创建方式 | 说明 |
| --- | --- | --- | --- |
| 校友 | 手机号 或 邮箱 | 验证码登录时先设置密码，密码设置后创建账号 | 手机号和/或邮箱作为登录标识，二选一即可；首次登录必须设置密码 |
| 管理员 | 账号（account） | 超级管理员后台创建 | 账号+密码登录，与校友体系隔离 |
| 超级管理员 | 账号（account） | 同上 | 同上 |

**校友账号创建规则：**

1. 管理员在后台录入校友档案（alumni_profiles），包含手机号和/或邮箱。
2. 校友通过手机号或邮箱 + 验证码登录。
3. 系统根据手机号/邮箱在 alumni_profiles 中查找匹配记录。
4. 找到校友档案后：
   - 若该校友已有 users 账号，直接登录并返回 JWT 令牌。
   - 若该校友没有 users 账号，系统不立即创建账号。而是签发一个短期有效的 registration_token（JWT，5 分钟有效，包含手机号/邮箱和 alumni_id），返回给前端。
5. 前端检测到 registration_token 后，弹出强制密码设置弹窗（不可跳过、不可关闭）。
6. 校友设置密码（须包含字母和数字，长度至少 8 位）后，前端调用 `/auth/setup-password` 接口提交 registration_token 和新密码。
7. 后端验证 registration_token 有效后，创建 users 记录（role=alumni，关联 alumni_id，密码为用户设置的密码），返回 JWT 令牌完成登录。

**校友绑定/修改手机号/邮箱规则：**

1. 校友登录后进入"我的资料"页面，可修改手机号和邮箱。
2. 修改手机号或邮箱需通过验证码验证（向当前绑定的旧号码/邮箱发送验证码）。
3. 验证通过后更新 users 表和 alumni_profiles 表的手机号/邮箱。
4. 新手机号/邮箱必须在系统中唯一（不被其他用户占用）。
5. 修改成功后同步更新两个表的对应字段。

**管理员注册规则：**

1. 由超级管理员在后台创建。
2. 指定账号（account）、密码、姓名。
3. 手机号和邮箱为可选字段，不用于登录。

## 4. 功能需求

### 4.1 校友登录

#### 4.1.1 短信验证码登录

| 编号 | 需求 |
| --- | --- |
| FR-110 | 校友可通过手机号 + 短信验证码登录（快捷登录）。 |
| FR-111 | 验证码登录无需密码，适合移动端快速登录场景。 |
| FR-112 | 验证码为 6 位数字，有效期 5 分钟。 |
| FR-113 | 验证码仅使用一次，使用后立即失效（一次性验证码）。 |
| FR-114 | 验证码登录成功后返回 JWT 访问令牌，与普通登录一致。 |
| FR-115 | 验证码登录成功后更新用户 last_login_at 时间。 |
| FR-116 | 验证码登录尝试失败计数：同一手机号 5 分钟失败 5 次锁定 5 分钟。 |
| FR-117 | 未注册用户通过短信验证码登录后，返回 registration_token，须先设置密码（须包含字母和数字，长度至少 8 位），密码设置后方创建账号并登录。 |
| FR-118 | 首次密码设置弹窗为强制步骤，不可跳过、不可关闭。 |
| FR-119 | 密码须同时包含字母和数字，长度至少 8 位，前端和后端均需校验。 |

**验证码登录前置：发送验证码**

| 接口 | 方法 | 权限 | 说明 |
| --- | --- | --- | --- |
| `/api/v1/auth/verify-code/send` | POST | 游客 | 发送短信/邮件验证码 |

**发送验证码请求体：**

```json
{
  "target": "13800138000",
  "purpose": "login"
}
```

- `target`：目标手机号或邮箱，必填。
- `purpose`：用途，必填，当前支持 `login`。

**发送验证码响应体：**

```json
{
  "code": 0,
  "data": {
    "expire_at": "2026-06-09T12:05:00Z",
    "resend_after": 60
  }
}
```

- `expire_at`：验证码过期时间。
- `resend_after`：距离下一次可重新发送的秒数。

**发送验证码限制：**

| 限制项 | 规则 | 说明 |
| --- | --- | --- |
| 发送频率 | 同一目标每 60 秒只能发送一次 | 防刷 |
| 每日上限 | 同一目标每天最多发送 10 次 | 控制短信/邮件费用 |
| 格式校验 | 目标格式不正确时拒绝发送 | 手机号/邮箱格式校验 |

**验证码存储设计：**

| 项目 | 说明 |
| --- | --- |
| 存储介质 | Redis |
| Key 格式 | `alumni:verify_code:{target}` |
| Value | 6 位随机数字验证码 |
| TTL | 5 分钟 |
| 未接入短信/邮件服务时 | 验证码设为 `888888`，接口正常返回成功 |

**短信验证码登录请求体：**

```json
{
  "phone": "13800138000",
  "code": "123456",
  "grant_type": "sms_code"
}
```

- `phone`：手机号，必填。
- `code`：验证码，6 位数字，必填。
- `grant_type`：固定值 `sms_code`，标识短信验证码登录模式。

**未注册用户短信验证码登录流程：**

1. 用户输入手机号 + 验证码。
2. 系统校验验证码正确。
3. 系统根据手机号在 `alumni_profiles` 表中查找匹配记录（`mobile = ? AND status = 'active'`）。
4. 找到校友档案后：
   - 检查该校友是否已有关联的 `users` 账号。
   - 若有，直接登录并返回 JWT 令牌。
   - 若无，签发 registration_token（JWT，5 分钟有效，包含 mobile 和 alumni_id），返回给前端。
5. 若未找到校友档案，返回错误"未找到匹配的校友信息"。
6. 前端弹出强制密码设置弹窗，用户设置密码后调用 `/auth/setup-password` 提交 registration_token 和新密码。
7. 后端验证 registration_token 后创建 users 记录并返回 JWT 令牌。

#### 4.1.2 邮件验证码登录

| 编号 | 需求 |
| --- | --- |
| FR-120 | 校友可通过邮箱 + 邮件验证码登录（快捷登录）。 |
| FR-121 | 验证码登录无需密码，适合 PC 端快速登录场景。 |
| FR-122 | 验证码为 6 位数字，有效期 5 分钟。 |
| FR-123 | 验证码仅使用一次，使用后立即失效（一次性验证码）。 |
| FR-124 | 验证码登录成功后返回 JWT 访问令牌，与普通登录一致。 |
| FR-125 | 验证码登录成功后更新用户 last_login_at 时间。 |
| FR-126 | 验证码登录尝试失败计数：同一邮箱 5 分钟失败 5 次锁定 5 分钟。 |
| FR-127 | 未注册用户通过邮件验证码登录后，返回 registration_token，须先设置密码（须包含字母和数字，长度至少 8 位），密码设置后方创建账号并登录。 |
| FR-128 | 首次密码设置弹窗为强制步骤，不可跳过、不可关闭。 |
| FR-129 | 密码须同时包含字母和数字，长度至少 8 位，前端和后端均需校验。 |

**邮件验证码登录请求体：**

```json
{
  "email": "user@example.com",
  "code": "123456",
  "grant_type": "email_code"
}
```

- `email`：邮箱，必填，大小写不敏感。
- `code`：验证码，6 位数字，必填。
- `grant_type`：固定值 `email_code`，标识邮件验证码登录模式。

**未注册用户邮件验证码登录流程：**

1. 用户输入邮箱 + 验证码。
2. 系统校验验证码正确（Redis 读取，忽略大小写）。
3. 系统根据邮箱在 `alumni_profiles` 表中查找匹配记录（`LOWER(email) = LOWER(?) AND status = 'active'`）。
4. 找到校友档案后：
   - 检查该校友是否已有关联的 `users` 账号。
   - 若有，直接登录并返回 JWT 令牌。
   - 若无，签发 registration_token（JWT，5 分钟有效，包含 email 和 alumni_id），返回给前端。
5. 若未找到校友档案，返回错误"未找到匹配的校友信息"。
6. 前端弹出强制密码设置弹窗，用户设置密码后调用 `/auth/setup-password` 提交 registration_token 和新密码。
7. 后端验证 registration_token 后创建 users 记录并返回 JWT 令牌。

#### 4.1.3 密码登录

| 编号 | 需求 |
| --- | --- |
| FR-101 | 校友可通过手机号 + 密码登录。 |
| FR-102 | 校友可通过邮箱 + 密码登录。 |
| FR-103 | 登录接口自动识别登录标识类型（手机号 / 邮箱 / 账号），无需前端指定登录方式。 |
| FR-104 | 登录成功后返回 JWT 访问令牌，包含用户 ID、角色、登录标识。 |
| FR-105 | 登录失败返回统一错误提示"账号或密码错误"，不暴露具体账户状态（如"该手机号未注册"）。 |
| FR-106 | 登录成功后更新用户 last_login_at 时间。 |
| FR-107 | 登录尝试失败计数：同一标识 5 分钟失败 5 次锁定 5 分钟。 |
| FR-108 | 登录成功后清除失败计数。 |
| FR-109 | 锁定期间拒绝该标识的所有登录尝试（无论密码是否正确）。 |
| FR-110 | 修改密码时新密码须同时包含字母和数字，长度至少 8 位，前后端均需校验。 |

**登录接口设计（变更）：**

| 接口 | 方法 | 权限 | 说明 |
| --- | --- | --- | --- |
| `/api/v1/auth/login` | POST | 游客 | 登录（支持手机号/邮箱/账号三种标识，密码和验证码两种模式） |

**密码登录请求体：**

```json
// 方式一：手机号 + 密码
{
  "mobile": "13800138000",
  "password": "Password123"
}

// 方式二：邮箱 + 密码
{
  "email": "user@example.com",
  "password": "Password123"
}

// 方式三：账号 + 密码（管理员）
{
  "account": "admin",
  "password": "Password123"
}
```

- `mobile`、`email`、`account` 三个字段互斥，最多一个非空。
- `password` 必填。
- `grant_type` 不传或传 `password`，默认密码登录。

**短信验证码登录请求体：**

```json
{
  "phone": "13800138000",
  "code": "123456",
  "grant_type": "sms_code"
}
```

**邮件验证码登录请求体：**

```json
{
  "email": "user@example.com",
  "code": "123456",
  "grant_type": "email_code"
}
```

**登录自动识别规则（密码模式）：**

| 输入标识 | 识别规则 | 查询条件 |
| --- | --- | --- |
| 手机号 | 匹配 `^1[3-9]\d{9}$` | `SELECT * FROM users WHERE mobile = ? AND status = 'active'` |
| 邮箱 | 匹配 `^[^\s@]+@[^\s@]+\.[^\s@]+$` | `SELECT * FROM users WHERE LOWER(email) = LOWER(?) AND status = 'active'` |
| 账号 | 不匹配手机号/邮箱规则 | `SELECT * FROM users WHERE account = ? AND role IN ('admin', 'super_admin') AND status = 'active'` |

**注意：** 账号登录时额外限制 `role IN ('admin', 'super_admin')`，防止校友通过账号字段绕过限制。

### 4.2 管理员登录（保持不变）

管理员和超级管理员保持原有登录方式不变，账号 + 密码登录。

| 编号 | 需求 |
| --- | --- |
| FR-201 | 管理员通过账号 + 密码登录。 |
| FR-202 | 登录逻辑与一期一致，不变。 |
| FR-203 | 登录失败提示"账号或密码错误"。 |
| FR-204 | 登录尝试限制机制不变（5 次失败锁定 5 分钟）。 |

管理员登录时前端只传 `account` 和 `password` 字段，后端自动识别为账号登录模式。

### 4.3 校友绑定/修改手机号和邮箱

| 编号 | 需求 |
| --- | --- |
| FR-301 | 校友登录后在"我的资料"页面可修改手机号（mobile）和邮箱（email）。 |
| FR-302 | 修改手机号时需向当前绑定的旧手机号发送验证码，验证通过后才能更新。 |
| FR-303 | 修改邮箱时需向当前绑定的旧邮箱发送验证码，验证通过后才能更新。 |
| FR-304 | 若当前未绑定手机号/邮箱，直接修改时向新手机号/邮箱发送验证码验证。 |
| FR-305 | 新手机号/邮箱必须在系统中全局唯一，已被其他用户占用的拒绝修改。 |
| FR-306 | 修改成功后同时更新 users 表和 alumni_profiles 表的对应字段。 |
| FR-307 | 修改成功后清除该用户在旧手机号/邮箱上的登录锁定计数。 |

**修改接口设计：**

| 接口 | 方法 | 权限 | 说明 |
| --- | --- | --- | --- |
| `/api/v1/alumni/me/contact` | PUT | 校友 | 修改校友手机号和/或邮箱 |

**修改请求体：**

```json
{
  "mobile": "13800138001",
  "email": "new@example.com"
}
```

- `mobile` 和 `email` 最多一个非空（可只改一个）。
- 至少一个非空。
- 格式校验：手机号 `^1[3-9]\d{9}$`，邮箱标准格式。
- 通过 `/auth/verify-code/send` 发送的验证码验证身份（验证码 target 为旧手机号/旧邮箱或新手机号/新邮箱，依上述 FR-302/FR-304 规则）。

## 5. 页面需求

### 5.1 校友登录页变更

| 页面 | 说明 | 变更 |
| --- | --- | --- |
| 校友端登录页 | 校友登录入口 | 输入框改为自适应，新增验证码登录选项卡 |

**密码登录交互：**

1. 用户输入框支持手机号、邮箱、账号三种格式输入。
2. 用户输入手机号（如 `13800138000`），前端自动识别为手机号格式。
3. 用户输入邮箱（如 `user@example.com`），前端自动识别为邮箱格式。
4. 用户输入其他字符串（如 `admin`），前端自动识别为账号格式。
5. 用户只需输入用户名和密码，无需切换登录方式。

### 5.2 验证码登录页

| 页面 | 说明 | 变更 |
| --- | --- | --- |
| 校友端登录页 | 校友登录入口 | 新增"短信验证码登录"和"邮件验证码登录"切换选项卡 |

**验证码登录页交互：**

1. 用户输入手机号或邮箱。
2. 点击"获取验证码"按钮，系统发送短信/邮件验证码。
3. 用户输入验证码。
4. 提交后系统校验验证码并登录。
5. 未注册用户在找到匹配的校友档案后，弹出强制密码设置弹窗（不可跳过、不可关闭），用户须设置密码（包含字母和数字，≥ 8 位），密码设置成功后创建账号并登录。
6. 已注册用户验证通过后直接登录，自动跳转到校友首页或原请求页面。

**登录页 tab 切换：**

```
[密码登录]  [短信验证码登录]  [邮件验证码登录]
```

点击"短信验证码登录"后表单变为：
- 手机号输入框 + 获取验证码按钮
- 6 位验证码输入框
- 登录按钮

点击"邮件验证码登录"后表单变为：
- 邮箱输入框 + 获取验证码按钮
- 6 位验证码输入框
- 登录按钮

### 5.3 校友"我的资料"页面变更

| 页面 | 说明 | 变更 |
| --- | --- | --- |
| 我的资料 | 校友维护个人联系方式 | 增加手机号和邮箱修改，需验证码验证 |

**"我的资料"页面联系人修改交互：**

1. 页面显示当前绑定的手机号和邮箱（脱敏显示，如 `138****8000`、`z***@example.com`）。
2. 点击"修改"按钮后进入验证模式：
   - 若修改已绑定的手机号：发送验证码到旧手机号
   - 若修改已绑定的邮箱：发送验证码到旧邮箱
   - 若首次绑定手机号/邮箱：发送验证码到新手机号/新邮箱
3. 用户输入验证码后提交。
4. 系统校验验证码、检查新手机号/邮箱唯一性。
5. 校验通过后更新 users 表和 alumni_profiles 表。
6. 更新成功后刷新页面显示新联系方式。

## 6. 数据库变更

### 6.1 users 表变更

新增 `email` 列：

| 字段 | 类型 | 说明 | 约束 |
| --- | --- | --- | --- |
| `email` | VARCHAR(255) | 邮箱地址 | NULL, UNIQUE（NULL 除外） |

变更后 `users` 表唯一约束：

| 字段 | 约束 | 说明 |
| --- | --- | --- |
| `account` | UNIQUE | 管理员登录账号唯一 |
| `mobile` | UNIQUE | 手机号全局唯一，NULL 除外 |
| `email` | UNIQUE | 邮箱全局唯一，NULL 除外 |

### 6.2 alumni_profiles 表变更

新增 `email` 列，与 users 表 email 保持一致：

| 字段 | 类型 | 说明 | 约束 |
| --- | --- | --- | --- |
| `email` | VARCHAR(255) | 校友邮箱 | NULL, 无唯一约束 |

### 6.3 迁移脚本

新增文件：`server/migrations/004_add_user_email.sql`

```sql
-- users 表增加 email 列
ALTER TABLE users ADD COLUMN email VARCHAR(255) NULL AFTER mobile;
ALTER TABLE users ADD UNIQUE INDEX uk_users_email (email);

-- alumni_profiles 表增加 email 列
ALTER TABLE alumni_profiles ADD COLUMN email VARCHAR(255) NULL AFTER mobile;
```

### 6.4 模型文件变更

- `server/internal/model/users.gen.go` — 通过 `make gendb` 重新生成，新增 `Email *string` 字段。
- `server/internal/model/alumni_profiles.gen.go` — 通过 `make gendb` 重新生成，新增 `Email *string` 字段。

## 7. 接口需求（汇总）

### 7.1 新增接口

| 接口 | 方法 | 权限 | 说明 |
| --- | --- | --- | --- |
| `/api/v1/auth/verify-code/send` | POST | 游客 | 发送短信/邮件验证码 |
| `/api/v1/auth/setup-password` | POST | 游客 | 校友首次登录设置密码（需 registration_token），密码须包含字母和数字且 ≥ 8 位 |
| `/api/v1/alumni/me/contact` | PUT | 校友 | 修改校友手机号和/或邮箱 |

### 7.2 变更接口

| 接口 | 变更内容 |
| --- | --- |
| `POST /api/v1/auth/login` | 请求体支持 `mobile`、`email`、`account` 三个字段，自动识别登录方式；支持 `grant_type` 字段区分密码/验证码两种登录模式。验证码登录时若用户未注册，返回 `registration_token` 而非 `access_token`，前端须引导用户设置密码后调用 `/auth/setup-password` 完成注册 |

### 7.3 变更接口（复用一期）

| 接口 | 变更内容 |
| --- | --- |
| `PUT /api/v1/alumni/me` | 新增 `email` 字段可修改，原有 `mobile` 字段修改需验证码验证 |

### 7.4 未变更接口

以下接口与一期、二期一致，不做变更：

- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/change-password`
- 校友 CRUD 接口
- 数据大屏接口
- 管理员管理接口

## 8. 非功能需求

| 类别 | 需求 |
| --- | --- |
| 登录标识唯一性 | 手机号、邮箱全局唯一，验证码登录和绑定修改时校验 |
| 密码安全 | 密码 bcrypt 哈希存储；密码须同时包含字母和数字，长度至少 8 位 |
| 错误提示一致性 | 登录失败统一提示"账号或密码错误"，不暴露具体账户状态 |
| 性能 | 登录查询需在 500ms 内完成，通过 mobile/email/account 索引加速 |
| 性能 | 验证码发送接口需在 1 秒内返回（短信/邮件发送为异步） |
| 兼容性 | 登录接口向后兼容，原有账号登录方式不受影响 |
| 安全性 | 登录尝试限制基于 Redis 实现，key 格式按登录标识类型区分 |
| 安全性 | 验证码一次性使用，验证后立即失效 |
| 安全性 | 邮件验证码目标邮箱大小写不敏感 |
| 可用性 | 短信/邮件服务未接入时验证码发送接口降级处理（固定验证码 + 日志记录），不影响功能 |

## 9. 验收标准

三期完成时，至少满足以下标准：

1. 校友可通过手机号 + 密码成功登录。
2. 校友可通过邮箱 + 密码成功登录。
3. 管理员可通过账号 + 密码成功登录（与一期一致）。
4. 登录接口自动识别登录标识类型，无需前端指定。
5. 同一手机号/邮箱不可重复创建账号。
6. 登录失败统一提示"账号或密码错误"，不提示"该手机号未注册"。
7. 同一手机号/邮箱/账号 5 分钟失败 5 次后被锁定 5 分钟，锁定期间拒绝登录。
8. 登录成功后 last_login_at 正确更新。
9. 登录成功后 JWT 令牌中包含正确的用户 ID、角色、登录标识。
10. 数据库 `users` 表 `email` 列存在且唯一约束生效。
11. 原有账号登录方式不受影响，管理员登录流程正常。
12. 可向手机号发送 6 位数字短信验证码。
13. 可向邮箱发送 6 位数字邮件验证码。
14. 验证码有效期 5 分钟，过期不可用。
15. 验证码仅使用一次，使用后立即失效。
16. 同一目标（手机号/邮箱）每 60 秒只能发送一次验证码。
17. 同一目标（手机号/邮箱）每天最多发送 10 次验证码。
18. 未注册校友通过短信验证码登录后，不自动创建账号，返回 registration_token。
19. 未注册校友通过邮件验证码登录后，不自动创建账号，返回 registration_token。
20. 校友拿到 registration_token 后必须设置密码（须包含字母和数字，≥ 8 位），不可跳过。
21. 密码设置成功后系统创建 users 记录并返回 JWT 令牌。
22. 短信验证码登录成功后返回 JWT 令牌，与普通登录一致（已注册用户）。
23. 邮件验证码登录成功后返回 JWT 令牌，与普通登录一致（已注册用户）。
24. 验证码登录时若未匹配到校友档案，返回"未找到匹配的校友信息"。
25. 邮件验证码登录时邮箱大小写不敏感。
26. 修改密码时新密码须同时包含字母和数字，长度至少 8 位，前后端均校验。
27. 校友登录后可在"我的资料"页面修改手机号和邮箱。
28. 修改手机号/邮箱需通过验证码验证。
29. 新手机号/邮箱在系统中唯一，已被占用的拒绝修改。

## 10. 后端实现要点（供开发参考）

### 10.1 Repository 层

`server/internal/repository/user.go` 需新增以下查询方法：

```go
// FindByMobile 通过手机号查找用户（忽略软删除）
FindByMobile(ctx context.Context, mobile string) (*model.Users, error)

// FindByEmail 通过邮箱查找用户（忽略软删除，大小写不敏感）
FindByEmail(ctx context.Context, email string) (*model.Users, error)
```

`server/internal/repository/login_attempt.go` 登录尝试 key 从 `auth:login_failure:{account}` 调整为 `auth:login_failure:{login_identifier}`，其中 `{login_identifier}` 是手机号、邮箱或账号的实际值（统一小写）。

### 10.2 Service 层

`server/internal/service/auth.go` Login 方法需改造为三段式查询：

1. 优先检查 `mobile` 字段是否非空，走 `FindByMobile`。
2. 否则检查 `email` 字段是否非空，走 `FindByEmail`。
3. 否则走 `FindByAccount`，并额外校验 `role IN ('admin', 'super_admin')`。

验证码登录新增方法：

```go
func (s *AuthService) LoginWithSMSCode(ctx context.Context, phone, code string) (*dto.LoginResult, error)

func (s *AuthService) LoginWithEmailCode(ctx context.Context, email, code string) (*dto.LoginResult, error)
```

验证码登录逻辑（以短信为例）：

1. 校验手机号格式。
2. 校验验证码（从 Redis 读取，一次性）。
3. 根据手机号在 alumni_profiles 中查找匹配记录。
4. 找到校友档案后，检查是否已有 users 账号，有则直接登录并返回 JWT 令牌。
5. 无 users 账号时，签发 registration_token（JWT，5 分钟有效，包含 mobile 和 alumni_id），返回给前端。
6. 前端调用 `/auth/setup-password` 提交 registration_token 和新密码。
7. `SetupPassword` 方法验证 registration_token，创建 users 记录（密码为用户设置的密码），返回 JWT 令牌。

### 10.3 DTO 变更

`server/internal/dto/auth.go` 变更：

```go
// 登录请求（变更）
type LoginRequest struct {
    Account   *string `json:"account" binding:"omitempty"`
    Mobile    *string `json:"mobile" binding:"omitempty"`
    Email     *string `json:"email" binding:"omitempty"`
    Password  string  `json:"password"`
    Code      string  `json:"code" binding:"omitempty"`
    GrantType string  `json:"grant_type"` // "password" | "sms_code" | "email_code"
}

// 验证码发送请求（新增）
type VerifyCodeRequest struct {
    Target  string `json:"target" binding:"required"`
    Purpose string `json:"purpose" binding:"required,oneof=login"`
}

// 验证码发送响应（新增）
type VerifyCodeResult struct {
    ExpireAt    time.Time `json:"expire_at"`
    ResendAfter int       `json:"re_send_after"` // 秒
}
```

### 10.4 Service 层 — 验证码

`server/internal/service/auth.go` 新增验证码相关方法：

```go
// SendVerifyCode 发送短信/邮件验证码
func (s *AuthService) SendVerifyCode(ctx context.Context, req dto.VerifyCodeRequest) (*dto.VerifyCodeResult, error)

// generateRandomCode 生成 6 位随机数字验证码
func (s *AuthService) generateRandomCode() string

// CodeSender 验证码发送接口（可插拔）
type CodeSender interface {
    Send(ctx context.Context, target, code string) error
}

// SMSSender 短信发送（接入短信服务时）
type SMSSender struct {
    APIKey       string
    APISecret    string
    SignName     string
    TemplateCode string
}

// EmailSender 邮件发送（接入 SMTP 服务时）
type EmailSender struct {
    Host     string // SMTP 服务器地址
    Port     int    // SMTP 端口
    Username string // 发件人邮箱
    Password string // 邮箱授权码
    FromName string // 发件人名称
}

// mockSender 测试用 mock（未接入短信/邮件服务时）
type mockSender struct{}
func (m *mockSender) Send(ctx context.Context, target, code string) error {
    log.Info("Code not configured, skipping send", "target", target, "code", code)
    return nil
}
```

**验证码发送流程：**

1. 校验 target 格式（手机号或邮箱）。
2. 根据 target 格式选择发送渠道（手机号→短信，邮箱→邮件）。
3. 检查发送频率限制（60 秒间隔）。
4. 检查每日发送上限（10 次）。
5. 生成 6 位随机数字验证码。
6. 写入 Redis，TTL 5 分钟。
7. 调用 CodeSender 发送。
8. 返回验证码过期时间和重新发送间隔。

**短信/邮件服务配置：**

```
# 短信服务配置
SMS_ENABLED=true/false
SMS_API_KEY=your_api_key
SMS_API_SECRET=your_api_secret
SMS_SIGN_NAME=山东大學政管學院
SMS_TEMPLATE_CODE=XXXXXX

# 邮件服务配置
EMAIL_ENABLED=true/false
EMAIL_HOST=smtp.example.com
EMAIL_PORT=465
EMAIL_USERNAME=noreply@example.com
EMAIL_PASSWORD=your_smtp_password
EMAIL_FROM_NAME=山东大學政管學院
```

未接入短信/邮件服务时：`SMS_ENABLED=false` 且 `EMAIL_ENABLED=false`，验证码固定为 `888888`，接口正常返回成功。

### 10.5 Repository 层 — 验证码

`server/internal/repository/verify_code.go`（新增文件）：

```go
type VerifyCodeStore interface {
    // Save 存储验证码，TTL 5 分钟
    Save(ctx context.Context, target, code string) error
    // Verify 验证并消费验证码（一次性），返回是否成功
    Verify(ctx context.Context, target, code string) (bool, error)
    // IncrementSendCount 增加发送计数
    IncrementSendCount(ctx context.Context, target string) (int64, error)
}
```

Redis key 设计：

| Key | 说明 | TTL |
| --- | --- | --- |
| `alumni:verify_code:{target}` | 验证码 | 5 分钟 |
| `alumni:verify_code:send_count:{target}` | 每日发送计数 | 当天 23:59:59 |

**注意：** target 在 Redis key 中统一小写处理（手机号不变，邮箱转为小写），确保验证码查找一致性。

### 10.6 Service 层 — 校友修改手机号/邮箱

`server/internal/service/alumni.go` 修改 UpdateMe 方法：

```go
// UpdateContact 修改校友手机号和/或邮箱
func (s *AuthService) UpdateContact(ctx context.Context, currentUserID uint64, req dto.UpdateContactRequest) error
```

**修改流程：**

1. 从 JWT 获取当前用户 ID，查询该用户的当前 mobile/email。
2. 校验新手机号/邮箱格式。
3. 检查新手机号/邮箱唯一性（users 表）。
4. 若修改的是已绑定的号码/邮箱（旧值=新值则跳过）。
5. 若修改已绑定的号码/邮箱：发送验证码到旧值，验证通过后更新。
6. 若首次绑定（旧值为空）：发送验证码到新值，验证通过后更新。
7. 同时更新 users 表和 alumni_profiles 表的 mobile/email。
8. 更新成功后清除该用户在旧手机号/旧邮箱上的登录锁定计数。

### 10.7 DTO 变更 — 修改联系方式

`server/internal/dto/alumni.go` 新增：

```go
type UpdateContactRequest struct {
    Mobile *string `json:"mobile"`
    Email  *string `json:"email"`
}

// 至少一个非空
```

## 11. 前端实现要点（供开发参考）

### 11.1 登录页

`web/src/pages/login/LoginPage.tsx` 输入框改为自适应，提交前自动识别：

```typescript
function normalizeLoginInput(raw: string): Partial<LoginRequest> {
  if (/^1[3-9]\d{9}$/.test(raw)) return { mobile: raw };
  if (/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(raw)) return { email: raw };
  return { account: raw };
}
```

### 11.2 类型定义

`web/src/types/auth.ts` 变更：

```typescript
interface LoginRequest {
    account?: string;
    mobile?: string;
    email?: string;
    password?: string;
    code?: string;
    grant_type?: 'password' | 'sms_code' | 'email_code';
}

interface LoginResponse {
    access_token: string;
    token_type: 'Bearer';
    expires_at: string;
    user: CurrentUser;
    registration_token?: string;  // 新用户首次登录时返回，非空表示需设置密码
}

interface SetupPasswordRequest {
    registration_token: string;
    new_password: string;
    confirm_password: string;
}

interface VerifyCodeRequest {
    target: string;   // 手机号或邮箱
    purpose: 'login';
}
```

### 11.3 API 层

`web/src/api/auth.ts` 新增验证码方法：

```typescript
// 发送验证码（短信或邮件）
sendVerifyCode(payload: VerifyCodeRequest) {
  return request<VerifyCodeResult>('/auth/verify-code/send', {
    method: 'POST',
    data: payload,
  });
}
```

### 11.4 登录页验证码登录 Tab

`web/src/pages/login/LoginPage.tsx` 新增"短信验证码登录"和"邮件验证码登录"选项卡：

- 手机号输入框 + "获取验证码"按钮（带倒计时）
- 邮箱输入框 + "获取验证码"按钮（带倒计时）
- 6 位验证码输入框（可分格输入）
- 登录按钮
- Tab 切换逻辑：密码登录 / 短信验证码登录 / 邮件验证码登录

### 11.5 "我的资料"页面

`web/src/pages/profile/ProfilePage.tsx` 增加手机号和邮箱的修改功能：

- 当前绑定手机号（脱敏显示）+ "修改"按钮
- 当前绑定邮箱（脱敏显示）+ "修改"按钮
- 修改弹窗：输入新手机号/邮箱 → 发送验证码 → 输入验证码 → 提交
- 脱敏显示规则：手机号 `138****8000`，邮箱 `z***@example.com`

## 12. 后续阶段预留

三期实现了手机号/邮箱验证码登录和密码登录，以及校友登录后绑定/修改手机号和邮箱。为后续扩展预留以下空间：

1. **换绑** — 校友已绑定的手机号/邮箱需要更换为另一个号码/邮箱时（与当前修改的不同，换绑需同时验证旧号码和新号码）。
2. **密码找回** — 通过手机号或邮箱验证码重置密码。
3. **图形验证码** — 验证码接口受频率限制保护，后续可叠加图形验证码防止批量验证。
5. **统一身份认证** — CAS、OAuth2、微信登录等。
