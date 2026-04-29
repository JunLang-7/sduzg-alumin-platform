package handler

import (
	"errors"
	"net/http"

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

	// 调用认证服务进行登录
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
