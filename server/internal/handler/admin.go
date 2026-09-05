package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	admin *service.AdminService
}

func NewAdminHandler(admin *service.AdminService) *AdminHandler {
	return &AdminHandler{admin: admin}
}

func (h *AdminHandler) List(c *gin.Context) {
	var req dto.AdminListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.admin.List(c.Request.Context(), req)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

func (h *AdminHandler) Create(c *gin.Context) {
	operator, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	var req dto.AdminCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.admin.Create(c.Request.Context(), *operator, req)
	if err == nil {
		response.JSON(c, http.StatusCreated, response.CodeSuccess, "success", result)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
	case errors.Is(err, common.ErrInvalidDataDomain):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "数据域无效")
	case errors.Is(err, common.ErrAccountAlreadyExists):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "账号已存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

func (h *AdminHandler) Detail(c *gin.Context) {
	operator, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid admin id")
		return
	}

	result, err := h.admin.Get(c.Request.Context(), *operator, id)
	if err == nil {
		response.Success(c, result)
		return
	}
	h.writeAccessError(c, err)
}

func (h *AdminHandler) ReplaceAccess(c *gin.Context) {
	operator, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid admin id")
		return
	}
	var req dto.AdminAccessUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.admin.ReplaceAccess(c.Request.Context(), *operator, id, req)
	if err == nil {
		response.Success(c, result)
		return
	}
	h.writeAccessError(c, err)
}

func (h *AdminHandler) ListDataDomains(c *gin.Context) {
	result, err := h.admin.ListDataDomains(c.Request.Context())
	if err == nil {
		response.Success(c, result)
		return
	}
	h.writeAccessError(c, err)
}

func (h *AdminHandler) Delete(c *gin.Context) {
	operator, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid admin id")
		return
	}

	err = h.admin.Delete(c.Request.Context(), *operator, id)
	if err == nil {
		response.Success(c, gin.H{"deleted": true})
		return
	}

	h.writeAccessError(c, err)
}

func (h *AdminHandler) writeAccessError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrInvalidDataDomain):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "数据域无效")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
	case errors.Is(err, common.ErrCannotModifySuper):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "不能修改超级管理员授权")
	case errors.Is(err, common.ErrCannotDeleteSelf):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "不能删除自己")
	case errors.Is(err, common.ErrCannotDeleteSuper):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "不能删除超级管理员账号")
	case errors.Is(err, common.ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应管理员不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
