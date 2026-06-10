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

func (h *AuthHandler) Logout(c *gin.Context) {
	result, err := h.auth.Logout(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
		return
	}

	response.Success(c, result)
}

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
