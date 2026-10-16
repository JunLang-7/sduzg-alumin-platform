package handler

import (
	"errors"
	"net/http"
	"unicode"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Login 处理登录请求，支持密码与验证码两种方式。
//
//	@Summary		登录
//	@Description	密码或验证码登录。验证码登录必须能匹配已有校友档案的手机号或邮箱；尚无账号时返回非空 registration_token，此时 access_token 为空，不能据此访问受保护接口。无需登录。
//	@Tags			认证与账号
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.LoginRequest	true	"登录参数；grant_type 省略或 password 为密码登录，sms_code/email_code 为验证码登录"
//	@Success		200		{object}	response.Body{data=dto.LoginResult}
//	@Failure		400		{object}	response.Body	"参数不正确，或验证码错误/已过期/已使用"
//	@Failure		401		{object}	response.Body	"用户名或密码错误，或未找到匹配的校友信息"
//	@Failure		403		{object}	response.Body	"账号已被停用"
//	@Failure		429		{object}	response.Body	"账号临时锁定或操作过于频繁"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.auth.Login(c.Request.Context(), req)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrInvalidCredentials):
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "用户名或密码错误")
	case errors.Is(err, common.ErrAccountDisabled):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "account is disabled")
	case errors.Is(err, common.ErrAccountLocked):
		response.Fail(c, http.StatusTooManyRequests, response.CodeTooManyRequests, "账号已临时锁定，请稍后再试")
	case errors.Is(err, common.ErrCodeInvalid):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "验证码错误")
	case errors.Is(err, common.ErrCodeExpired):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "验证码已过期")
	case errors.Is(err, common.ErrCodeConsumed):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "验证码已被使用")
	case errors.Is(err, common.ErrRateLimited):
		response.Fail(c, http.StatusTooManyRequests, response.CodeTooManyRequests, "操作过于频繁，请稍后再试")
	case errors.Is(err, common.ErrAlumniNotMatch):
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "未找到匹配的校友信息")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// Logout 退出登录，使当前令牌失效。
//
//	@Summary	退出登录
//	@Tags		认证与账号
//	@Produce	json
//	@Success	200	{object}	response.Body{data=dto.LogoutResult}
//	@Failure	500	{object}	response.Body
//	@Security	BearerAuth
//	@Router		/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	result, err := h.auth.Logout(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
		return
	}

	response.Success(c, result)
}

// Me 返回当前用户及本请求实时加载的授权能力。
//
//	@Summary		获取当前登录用户
//	@Description	返回当前用户及本请求实时加载的有效数据域与功能权限。登录响应不加载管理员授权（domains 与 permissions 为空数组），客户端应再请求本接口获取当前能力。
//	@Tags			认证与账号
//	@Produce		json
//	@Success		200	{object}	response.Body{data=dto.UserDTO}
//	@Failure		401	{object}	response.Body	"未登录、令牌失效或用户不存在"
//	@Failure		403	{object}	response.Body	"账号已被停用"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	result, err := h.auth.CurrentUser(c.Request.Context(), *access)
	if err == nil {
		response.Success(c, result)
		return
	}
	switch {
	case errors.Is(err, common.ErrAccountDisabled):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "account is disabled")
	case errors.Is(err, common.ErrUserNotFound):
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// ChangePassword 修改当前账号密码，成功后无 data。
//
//	@Summary		修改密码
//	@Description	新密码必须同时包含字母和数字，且长度至少 8 位。
//	@Tags			认证与账号
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.ChangePasswordRequest	true	"修改密码参数"
//	@Success		200		{object}	response.Body				"成功，无 data"
//	@Failure		400		{object}	response.Body				"参数不正确、两次密码不一致或新密码强度不足"
//	@Failure		401		{object}	response.Body				"未登录，或原密码错误"
//	@Failure		503		{object}	response.Body				"数据库不可用"
//	@Security		BearerAuth
//	@Router			/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "new passwords do not match")
		return
	}
	if !isValidPassword(req.NewPassword) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "密码必须包含字母和数字，且长度至少8位")
		return
	}

	uid, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	if err := h.auth.ChangePassword(c.Request.Context(), uid, req.OldPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidCredentials):
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "用户名或密码错误")
		case errors.Is(err, common.ErrDatabaseUnavailable):
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
		default:
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
		}
		return
	}

	response.Success(c, nil)
}

// SendVerifyCode 发送登录验证码。
//
//	@Summary		发送登录验证码
//	@Description	target 为中国大陆手机号（^1[3-9]\d{9}$）或邮箱，purpose 当前只接受 login。同一目标至少间隔 60 秒，每日最多 10 次。需按部署说明配置 Redis 与短信/邮件通道。无需登录。
//	@Tags			认证与账号
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.VerifyCodeRequest	true	"发送目标与用途"
//	@Success		200		{object}	response.Body{data=dto.VerifyCodeResult}
//	@Failure		400		{object}	response.Body	"目标格式不正确，或超出业务频次限制"
//	@Failure		429		{object}	response.Body	"请求过于频繁"
//	@Failure		503		{object}	response.Body	"验证码缓存不可用"
//	@Router			/auth/verify-code/send [post]
func (h *AuthHandler) SendVerifyCode(c *gin.Context) {
	var req dto.VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.auth.SendVerifyCode(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidRequest):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "目标格式不正确")
		case errors.Is(err, common.ErrCacheUnavailable):
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "服务不可用")
		default:
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		}
		return
	}

	response.Success(c, result)
}

// SetupPassword 使用注册令牌首次设置密码并创建校友账号。
//
//	@Summary		首次设置密码
//	@Description	使用验证码登录返回的 registration_token 创建校友账号并设置密码，成功后直接返回登录结果。无需登录。
//	@Tags			认证与账号
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.SetupPasswordRequest	true	"注册令牌与新密码"
//	@Success		200		{object}	response.Body{data=dto.SetupPasswordResult}
//	@Failure		400		{object}	response.Body	"注册令牌无效或已过期、两次密码不一致、新密码强度不足"
//	@Router			/auth/setup-password [post]
func (h *AuthHandler) SetupPassword(c *gin.Context) {
	var req dto.SetupPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "new passwords do not match")
		return
	}
	if !isValidPassword(req.NewPassword) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "密码必须包含字母和数字，且长度至少8位")
		return
	}

	result, err := h.auth.SetupPassword(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}

// UpdateContact 修改本人联系方式，成功后无 data。
//
//	@Summary		修改本人联系方式
//	@Description	仅校友本人可调用。传入的字段才会被修改，变更需通过验证码校验。
//	@Tags			校友本人资料
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.UpdateContactRequest	true	"联系方式与验证码"
//	@Success		200		{object}	response.Body				"成功，无 data"
//	@Failure		400		{object}	response.Body				"参数错误、验证码错误，或手机号/邮箱已被占用"
//	@Failure		403		{object}	response.Body				"权限不足"
//	@Failure		404		{object}	response.Body				"用户不存在"
//	@Failure		503		{object}	response.Body				"数据库不可用"
//	@Security		BearerAuth
//	@Router			/alumni/me/contact [put]
func (h *AuthHandler) UpdateContact(c *gin.Context) {
	var req dto.UpdateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	uid, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	if err := h.auth.UpdateContact(c.Request.Context(), uid, req); err != nil {
		switch {
		case errors.Is(err, common.ErrPermissionDenied):
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
		case errors.Is(err, common.ErrUserNotFound):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "用户不存在")
		case errors.Is(err, common.ErrAccountAlreadyExists):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "手机号或邮箱已被占用")
		case errors.Is(err, common.ErrInvalidRequest):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数错误")
		case errors.Is(err, common.ErrCodeInvalid):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "验证码错误")
		case errors.Is(err, common.ErrDatabaseUnavailable):
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
		default:
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
		}
		return
	}

	response.Success(c, nil)
}

func isValidPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	var hasLetter, hasDigit bool
	for _, ch := range password {
		if unicode.IsLetter(ch) {
			hasLetter = true
		}
		if unicode.IsDigit(ch) {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}
